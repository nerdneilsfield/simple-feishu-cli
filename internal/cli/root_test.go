package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/feishu"
)

func TestNewRootCmdUsesFeishuNameAndPrintsHelp(t *testing.T) {
	cmd := NewRootCmd()

	if got := cmd.Name(); got != "feishu" {
		t.Fatalf("root command name = %q, want %q", got, "feishu")
	}

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if stdout.Len() == 0 {
		t.Fatal("expected help output, got empty output")
	}
}

func TestSendTextCommandOutputsStableFields(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewMessenger: func(cfg config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendText: func(_ context.Context, input feishu.TextMessageInput) (feishu.MessageResult, error) {
					if input.ReceiveIDType != "open_id" || input.ReceiveID != "ou_xxx" || input.Text != "hello" {
						t.Fatalf("SendText input = %#v", input)
					}
					return feishu.MessageResult{
						MessageID:     "om_xxx",
						MsgType:       "text",
						ReceiveID:     "ou_xxx",
						ReceiveIDType: "open_id",
					}, nil
				},
			}, nil
		},
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{
		"--app-id", "flag-id",
		"--app-secret", "flag-secret",
		"send", "text",
		"--to-type", "open_id",
		"--to", "ou_xxx",
		"--text", "hello",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := "message_id=om_xxx\nmsg_type=text\nreceive_id=ou_xxx\nreceive_id_type=open_id\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestExitCodeMapsConfigErrorsToThree(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{}, errors.New("missing app_id")
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			t.Fatal("NewMessenger should not be called on config error")
			return nil, nil
		},
	})

	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "hello"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want config error")
	}

	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
}

func TestExitCodeMapsLocalFileErrorsToFour(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "id", AppSecret: "secret"}, nil
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendFile: func(context.Context, feishu.FileMessageInput) (feishu.MessageResult, error) {
					return feishu.MessageResult{}, &feishu.LocalFileError{Op: "stat_file", Path: "/tmp/missing", Err: errors.New("no such file")}
				},
			}, nil
		},
	})

	cmd.SetArgs([]string{"send", "file", "--to-type", "open_id", "--to", "ou_xxx", "--path", "/tmp/missing"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want local file error")
	}

	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 4)
	}
}

type fakeMessenger struct {
	sendText func(context.Context, feishu.TextMessageInput) (feishu.MessageResult, error)
	sendFile func(context.Context, feishu.FileMessageInput) (feishu.MessageResult, error)
}

func (f fakeMessenger) SendText(ctx context.Context, input feishu.TextMessageInput) (feishu.MessageResult, error) {
	return f.sendText(ctx, input)
}

func (f fakeMessenger) SendFile(ctx context.Context, input feishu.FileMessageInput) (feishu.MessageResult, error) {
	return f.sendFile(ctx, input)
}
