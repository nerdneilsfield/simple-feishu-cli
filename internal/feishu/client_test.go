package feishu

import (
	"context"
	"errors"
	"strings"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
)

func TestNewClientCreatesSDKClient(t *testing.T) {
	client, err := NewClient(config.Config{
		AppID:     "cli_xxx",
		AppSecret: "secret",
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.sdk == nil {
		t.Fatal("NewClient() returned nil sdk client")
	}
}

func TestNewClientRejectsMissingCredentials(t *testing.T) {
	_, err := NewClient(config.Config{})
	if err == nil {
		t.Fatal("NewClient() error = nil, want missing credential error")
	}
}

func TestWrapErrorHandlesCodeError(t *testing.T) {
	err := wrapError("send_text", larkcore.CodeError{
		Code: 99991663,
		Msg:  "insufficient permission",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("wrapError() error = %T, want *APIError", err)
	}

	if apiErr.Code != 99991663 || apiErr.Message != "insufficient permission" {
		t.Fatalf("wrapError() = %#v, want code/message preserved", apiErr)
	}
}

func TestSendTextBuildsRequestAndReturnsMessageResult(t *testing.T) {
	fake := &fakeMessageService{
		resp: &larkim.CreateMessageResp{
			CodeError: larkcore.CodeError{Code: 0},
			Data: &larkim.CreateMessageRespData{
				MessageId: larkcore.StringPtr("om_xxx"),
				MsgType:   larkcore.StringPtr(larkim.MsgTypeText),
			},
		},
	}
	client := &Client{messageAPI: fake}

	result, err := client.SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}

	if result.MessageID != "om_xxx" || result.MsgType != larkim.MsgTypeText {
		t.Fatalf("SendText() result = %#v", result)
	}

	if fake.req == nil {
		t.Fatal("SendText() did not call Create")
	}

	body := fake.req.Body
	if body == nil {
		t.Fatal("SendText() request body = nil")
	}

	if got := larkcore.StringValue(body.ReceiveId); got != "ou_xxx" {
		t.Fatalf("request receive_id = %q, want %q", got, "ou_xxx")
	}
	if got := larkcore.StringValue(body.MsgType); got != larkim.MsgTypeText {
		t.Fatalf("request msg_type = %q, want %q", got, larkim.MsgTypeText)
	}
	if got := larkcore.StringValue(body.Content); !strings.Contains(got, `"text":"hello"`) {
		t.Fatalf("request content = %q, want text payload", got)
	}
}

func TestSendTextWrapsSDKErrors(t *testing.T) {
	client := &Client{
		messageAPI: &fakeMessageService{
			err: larkcore.CodeError{Code: 99991663, Msg: "insufficient permission"},
		},
	}

	_, err := client.SendText(context.Background(), TextMessageInput{
		ReceiveIDType: larkim.ReceiveIdTypeOpenId,
		ReceiveID:     "ou_xxx",
		Text:          "hello",
	})
	if err == nil {
		t.Fatal("SendText() error = nil, want wrapped api error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("SendText() error = %T, want *APIError", err)
	}
	if apiErr.Code != 99991663 {
		t.Fatalf("SendText() error code = %d, want %d", apiErr.Code, 99991663)
	}
}

type fakeMessageService struct {
	req  *larkim.CreateMessageReq
	resp *larkim.CreateMessageResp
	err  error
}

func (f *fakeMessageService) Create(_ context.Context, req *larkim.CreateMessageReq, _ ...larkcore.RequestOptionFunc) (*larkim.CreateMessageResp, error) {
	f.req = req
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}
