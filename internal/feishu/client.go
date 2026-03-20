package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
)

type MessageResult struct {
	MessageID     string
	MsgType       string
	ReceiveID     string
	ReceiveIDType string
}

type TextMessageInput struct {
	ReceiveIDType string
	ReceiveID     string
	Text          string
}

type FileMessageInput struct {
	ReceiveIDType string
	ReceiveID     string
	FilePath      string
}

type Messenger interface {
	SendText(ctx context.Context, input TextMessageInput) (MessageResult, error)
	SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error)
}

type Client struct {
	sdk        *lark.Client
	fileAPI    fileAPI
	messageAPI messageAPI
}

type APIError struct {
	Op      string
	Code    int
	Message string
	Err     error
}

type LocalFileError struct {
	Op   string
	Path string
	Err  error
}

type messageAPI interface {
	Create(ctx context.Context, req *larkim.CreateMessageReq, options ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
}

type fileAPI interface {
	Create(ctx context.Context, req *larkim.CreateFileReq, options ...larkcore.RequestOptionFunc) (*larkim.CreateFileResp, error)
}

func (e *APIError) Error() string {
	if e == nil {
		return "<nil>"
	}

	if e.Code == 0 {
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	}

	return fmt.Sprintf("%s: code=%d msg=%s", e.Op, e.Code, e.Message)
}

func (e *APIError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *LocalFileError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s %q: %v", e.Op, e.Path, e.Err)
}

func (e *LocalFileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func NewClient(cfg config.Config) (*Client, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, fmt.Errorf("missing app credentials")
	}

	sdk := lark.NewClient(cfg.AppID, cfg.AppSecret)

	return &Client{
		sdk:        sdk,
		fileAPI:    sdk.Im.V1.File,
		messageAPI: sdk.Im.V1.Message,
	}, nil
}

func (c *Client) SendText(ctx context.Context, input TextMessageInput) (MessageResult, error) {
	if c == nil || c.messageAPI == nil {
		return MessageResult{}, errors.New("message api is not configured")
	}

	content, err := json.Marshal(map[string]string{"text": input.Text})
	if err != nil {
		return MessageResult{}, fmt.Errorf("marshal text content: %w", err)
	}

	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(input.ReceiveID).
		MsgType(larkim.MsgTypeText).
		Content(string(content)).
		Build()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(input.ReceiveIDType).
		Body(body).
		Build()
	req.Body = body

	resp, err := c.messageAPI.Create(ctx, req)
	if err != nil {
		return MessageResult{}, wrapError("send_text", err)
	}
	if !resp.Success() {
		return MessageResult{}, wrapError("send_text", resp.CodeError)
	}
	if resp.Data == nil {
		return MessageResult{}, &APIError{Op: "send_text", Message: "missing response data"}
	}

	return MessageResult{
		MessageID:     larkcore.StringValue(resp.Data.MessageId),
		MsgType:       larkcore.StringValue(resp.Data.MsgType),
		ReceiveID:     input.ReceiveID,
		ReceiveIDType: input.ReceiveIDType,
	}, nil
}

func (c *Client) SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error) {
	if c == nil || c.fileAPI == nil {
		return MessageResult{}, errors.New("file api is not configured")
	}

	info, err := os.Stat(input.FilePath)
	if err != nil {
		return MessageResult{}, &LocalFileError{Op: "stat_file", Path: input.FilePath, Err: err}
	}
	if info.IsDir() {
		return MessageResult{}, &LocalFileError{Op: "stat_file", Path: input.FilePath, Err: errors.New("path is a directory")}
	}

	fileName := filepath.Base(input.FilePath)
	fileType := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	if fileType == "" {
		fileType = "stream"
	}

	uploadBody, err := larkim.NewCreateFilePathReqBodyBuilder().
		FileType(fileType).
		FileName(fileName).
		FilePath(input.FilePath).
		Build()
	if err != nil {
		return MessageResult{}, &LocalFileError{Op: "read_file", Path: input.FilePath, Err: err}
	}

	uploadReq := larkim.NewCreateFileReqBuilder().Body(uploadBody).Build()
	uploadReq.Body = uploadBody

	uploadResp, err := c.fileAPI.Create(ctx, uploadReq)
	if err != nil {
		return MessageResult{}, wrapError("upload_file", err)
	}
	if !uploadResp.Success() {
		return MessageResult{}, wrapError("upload_file", uploadResp.CodeError)
	}
	if uploadResp.Data == nil || strings.TrimSpace(larkcore.StringValue(uploadResp.Data.FileKey)) == "" {
		return MessageResult{}, &APIError{Op: "upload_file", Message: "missing file_key in response"}
	}

	content, err := json.Marshal(map[string]string{"file_key": larkcore.StringValue(uploadResp.Data.FileKey)})
	if err != nil {
		return MessageResult{}, fmt.Errorf("marshal file content: %w", err)
	}
	if c.messageAPI == nil {
		return MessageResult{}, errors.New("message api is not configured")
	}

	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(input.ReceiveID).
		MsgType("file").
		Content(string(content)).
		Build()
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(input.ReceiveIDType).
		Body(body).
		Build()
	req.Body = body

	resp, err := c.messageAPI.Create(ctx, req)
	if err != nil {
		return MessageResult{}, wrapError("send_file", err)
	}
	if !resp.Success() {
		return MessageResult{}, wrapError("send_file", resp.CodeError)
	}
	if resp.Data == nil {
		return MessageResult{}, &APIError{Op: "send_file", Message: "missing response data"}
	}

	return MessageResult{
		MessageID:     larkcore.StringValue(resp.Data.MessageId),
		MsgType:       larkcore.StringValue(resp.Data.MsgType),
		ReceiveID:     input.ReceiveID,
		ReceiveIDType: input.ReceiveIDType,
	}, nil
}

func wrapError(op string, err error) error {
	if err == nil {
		return nil
	}

	var codeErr larkcore.CodeError
	if errors.As(err, &codeErr) {
		return &APIError{
			Op:      op,
			Code:    codeErr.Code,
			Message: codeErr.Msg,
			Err:     err,
		}
	}

	return &APIError{
		Op:      op,
		Message: err.Error(),
		Err:     err,
	}
}
