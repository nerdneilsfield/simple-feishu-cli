package feishu

import (
	"context"
	"errors"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
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
	sdk *lark.Client
}

type APIError struct {
	Op      string
	Code    int
	Message string
	Err     error
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

func NewClient(cfg config.Config) (*Client, error) {
	if strings.TrimSpace(cfg.AppID) == "" || strings.TrimSpace(cfg.AppSecret) == "" {
		return nil, fmt.Errorf("missing app credentials")
	}

	return &Client{
		sdk: lark.NewClient(cfg.AppID, cfg.AppSecret),
	}, nil
}

func (c *Client) SendText(ctx context.Context, input TextMessageInput) (MessageResult, error) {
	return MessageResult{}, errors.New("send text not implemented")
}

func (c *Client) SendFile(ctx context.Context, input FileMessageInput) (MessageResult, error) {
	return MessageResult{}, errors.New("send file not implemented")
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
