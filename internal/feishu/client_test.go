package feishu

import (
	"errors"
	"testing"

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
