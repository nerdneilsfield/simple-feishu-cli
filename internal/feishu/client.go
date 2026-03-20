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

type ChatOwner struct {
	OpenID  string
	UnionID string
}

type ChatSummary struct {
	ChatID string
	Name   string
	Owner  ChatOwner
}

type Messenger interface {
	SendText(ctx context.Context, input TextMessageInput) (MessageResult, error)
	SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error)
}

type ChatLister interface {
	ListChats(ctx context.Context) ([]ChatSummary, error)
}

type Client struct {
	sdk         *lark.Client
	fileAPI     fileAPI
	messageAPI  messageAPI
	chatListAPI chatListAPI
	chatGetAPI  chatGetAPI
}

type APIError struct {
	Op      string
	Code    int
	Message string
	Err     error
}

type ClientError struct {
	Op      string
	Message string
	Err     error
}

type LocalFileError struct {
	Op   string
	Path string
	Err  error
}

type createMessageInput struct {
	ReceiveIDType string
	ReceiveID     string
	MsgType       string
	Content       string
}

type createFileInput struct {
	FileType string
	FileName string
	FilePath string
}

type listChatsPageInput struct {
	UserIDType string
	PageToken  string
	PageSize   int
}

type getChatInput struct {
	ChatID     string
	UserIDType string
}

type messageAPI interface {
	Create(ctx context.Context, input createMessageInput) (*larkim.CreateMessageResp, error)
}

type fileAPI interface {
	Create(ctx context.Context, input createFileInput) (*larkim.CreateFileResp, error)
}

type chatListAPI interface {
	List(ctx context.Context, input listChatsPageInput) (*larkim.ListChatResp, error)
}

type chatGetAPI interface {
	Get(ctx context.Context, input getChatInput) (*larkim.GetChatResp, error)
}

type sdkMessageService interface {
	Create(ctx context.Context, req *larkim.CreateMessageReq, options ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error)
}

type sdkFileService interface {
	Create(ctx context.Context, req *larkim.CreateFileReq, options ...larkcore.RequestOptionFunc) (*larkim.CreateFileResp, error)
}

type sdkChatService interface {
	List(ctx context.Context, req *larkim.ListChatReq, options ...larkcore.RequestOptionFunc) (*larkim.ListChatResp, error)
	Get(ctx context.Context, req *larkim.GetChatReq, options ...larkcore.RequestOptionFunc) (*larkim.GetChatResp, error)
}

type sdkMessageAPI struct {
	service sdkMessageService
}

type sdkFileAPI struct {
	service sdkFileService
}

type sdkChatAPI struct {
	service sdkChatService
}

func (a sdkMessageAPI) Create(ctx context.Context, input createMessageInput) (*larkim.CreateMessageResp, error) {
	body := larkim.NewCreateMessageReqBodyBuilder().
		ReceiveId(input.ReceiveID).
		MsgType(input.MsgType).
		Content(input.Content).
		Build()

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(input.ReceiveIDType).
		Body(body).
		Build()
	req.Body = body

	return a.service.Create(ctx, req)
}

func (a sdkFileAPI) Create(ctx context.Context, input createFileInput) (*larkim.CreateFileResp, error) {
	uploadBody, err := larkim.NewCreateFilePathReqBodyBuilder().
		FileType(input.FileType).
		FileName(input.FileName).
		FilePath(input.FilePath).
		Build()
	if err != nil {
		return nil, &LocalFileError{Op: "read_file", Path: input.FilePath, Err: err}
	}

	uploadReq := larkim.NewCreateFileReqBuilder().Body(uploadBody).Build()
	uploadReq.Body = uploadBody

	return a.service.Create(ctx, uploadReq)
}

func (a sdkChatAPI) List(ctx context.Context, input listChatsPageInput) (*larkim.ListChatResp, error) {
	builder := larkim.NewListChatReqBuilder().
		UserIdType(input.UserIDType).
		PageSize(input.PageSize)
	if input.PageToken != "" {
		builder.PageToken(input.PageToken)
	}

	return a.service.List(ctx, builder.Build())
}

func (a sdkChatAPI) Get(ctx context.Context, input getChatInput) (*larkim.GetChatResp, error) {
	req := larkim.NewGetChatReqBuilder().
		ChatId(input.ChatID).
		UserIdType(input.UserIDType).
		Build()

	return a.service.Get(ctx, req)
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

func (e *ClientError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *ClientError) Unwrap() error {
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

func normalizeAppCredentials(cfg config.Config) (string, string) {
	return strings.TrimSpace(cfg.AppID), strings.TrimSpace(cfg.AppSecret)
}

func NewClient(cfg config.Config) (*Client, error) {
	appID, appSecret := normalizeAppCredentials(cfg)
	if appID == "" || appSecret == "" {
		return nil, &ClientError{Op: "new_client", Message: "missing app credentials"}
	}

	sdk := lark.NewClient(appID, appSecret)

	return &Client{
		sdk:         sdk,
		fileAPI:     sdkFileAPI{service: sdk.Im.V1.File},
		messageAPI:  sdkMessageAPI{service: sdk.Im.V1.Message},
		chatListAPI: sdkChatAPI{service: sdk.Im.V1.Chat},
		chatGetAPI:  sdkChatAPI{service: sdk.Im.V1.Chat},
	}, nil
}

const chatListPageSize = 100

func (c *Client) ListChats(ctx context.Context) ([]ChatSummary, error) {
	if c == nil || c.chatListAPI == nil {
		return nil, &ClientError{Op: "list_chats", Message: "chat list api is not configured"}
	}
	if c.chatGetAPI == nil {
		return nil, &ClientError{Op: "list_chats", Message: "chat get api is not configured"}
	}

	var (
		pageToken string
		results   []ChatSummary
	)

	for {
		resp, err := c.chatListAPI.List(ctx, listChatsPageInput{
			UserIDType: larkim.UserIdTypeListChatOpenId,
			PageToken:  pageToken,
			PageSize:   chatListPageSize,
		})
		if err != nil {
			return nil, wrapError("list_chats", err)
		}
		if !resp.Success() {
			return nil, wrapError("list_chats", resp.CodeError)
		}
		if resp.Data == nil {
			return nil, &APIError{Op: "list_chats", Message: "missing response data"}
		}

		for _, item := range resp.Data.Items {
			if item == nil {
				continue
			}

			summary := ChatSummary{
				ChatID: larkcore.StringValue(item.ChatId),
				Name:   larkcore.StringValue(item.Name),
			}
			openID, err := c.getChatOwnerID(ctx, summary.ChatID, larkim.UserIdTypeGetChatOpenId)
			if err != nil {
				return nil, err
			}
			unionID, err := c.getChatOwnerID(ctx, summary.ChatID, larkim.UserIdTypeGetChatUnionId)
			if err != nil {
				return nil, err
			}
			summary.Owner.OpenID = openID
			summary.Owner.UnionID = unionID
			results = append(results, summary)
		}

		if !larkcore.BoolValue(resp.Data.HasMore) {
			break
		}
		pageToken = larkcore.StringValue(resp.Data.PageToken)
		if pageToken == "" {
			return nil, &APIError{Op: "list_chats", Message: "missing page_token for paginated response"}
		}
	}

	return results, nil
}

func (c *Client) getChatOwnerID(ctx context.Context, chatID, userIDType string) (string, error) {
	resp, err := c.chatGetAPI.Get(ctx, getChatInput{ChatID: chatID, UserIDType: userIDType})
	if err != nil {
		return "", wrapError("get_chat", err)
	}
	if !resp.Success() {
		return "", wrapError("get_chat", resp.CodeError)
	}
	if resp.Data == nil {
		return "", &APIError{Op: "get_chat", Message: "missing response data"}
	}

	return larkcore.StringValue(resp.Data.OwnerId), nil
}

func (c *Client) SendText(ctx context.Context, input TextMessageInput) (MessageResult, error) {
	if c == nil || c.messageAPI == nil {
		return MessageResult{}, &ClientError{Op: "send_text", Message: "message api is not configured"}
	}

	content, err := json.Marshal(map[string]string{"text": input.Text})
	if err != nil {
		return MessageResult{}, &ClientError{Op: "send_text", Message: "marshal text content", Err: err}
	}

	resp, err := c.messageAPI.Create(ctx, createMessageInput{
		ReceiveIDType: input.ReceiveIDType,
		ReceiveID:     input.ReceiveID,
		MsgType:       larkim.MsgTypeText,
		Content:       string(content),
	})
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
		return MessageResult{}, &ClientError{Op: "send_file", Message: "file api is not configured"}
	}
	if c.messageAPI == nil {
		return MessageResult{}, &ClientError{Op: "send_file", Message: "message api is not configured"}
	}

	info, err := os.Stat(input.FilePath)
	if err != nil {
		return MessageResult{}, &LocalFileError{Op: "stat_file", Path: input.FilePath, Err: err}
	}
	if info.IsDir() {
		return MessageResult{}, &LocalFileError{Op: "stat_file", Path: input.FilePath, Err: errors.New("path is a directory")}
	}

	fileName := filepath.Base(input.FilePath)
	fileType := uploadFileType(fileName)

	uploadResp, err := c.fileAPI.Create(ctx, createFileInput{
		FileType: fileType,
		FileName: fileName,
		FilePath: input.FilePath,
	})
	if err != nil {
		var fileErr *LocalFileError
		if errors.As(err, &fileErr) {
			return MessageResult{}, fileErr
		}
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
		return MessageResult{}, &ClientError{Op: "send_file", Message: "marshal file content", Err: err}
	}

	resp, err := c.messageAPI.Create(ctx, createMessageInput{
		ReceiveIDType: input.ReceiveIDType,
		ReceiveID:     input.ReceiveID,
		MsgType:       "file",
		Content:       string(content),
	})
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

func uploadFileType(fileName string) string {
	extension := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")

	switch extension {
	case larkim.FileTypeOpus,
		larkim.FileTypeMp4,
		larkim.FileTypePdf,
		larkim.FileTypeDoc,
		larkim.FileTypeXls,
		larkim.FileTypePpt:
		return extension
	default:
		return larkim.FileTypeStream
	}
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
