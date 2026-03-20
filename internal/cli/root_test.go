package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

	help := stdout.String()
	for _, want := range []string{
		"Usage:\n  feishu [command]",
		"list        List Feishu resources",
		"send        Send messages or files",
		"--app-id string",
		"--app-secret string",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestListChatsCommandIsDiscoverableInHelp(t *testing.T) {
	cmd := NewRootCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"list", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := stdout.String()
	for _, want := range []string{
		"feishu list [command]",
		"chats       List chats joined by the app bot",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestListChatsCommandRejectsInvalidFormat(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			t.Fatal("LoadConfig should not be called for invalid --format")
			return config.Config{}, nil
		},
	})
	cmd.SetArgs([]string{"list", "chats", "--format", "yaml"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "invalid --format") {
		t.Fatalf("error = %q, want invalid --format message", err)
	}
}

func TestListChatsTableOutput(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewChatLister: func(config.Config) (feishu.ChatLister, error) {
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					return []feishu.ChatSummary{
						{ChatID: "oc_xxx", Name: "Ops", Owner: feishu.ChatOwner{OpenID: "ou_xxx", UnionID: "on_xxx"}},
						{ChatID: "oc_yyy", Name: "Infra", Owner: feishu.ChatOwner{OpenID: "ou_yyy", UnionID: "on_yyy"}},
					}, nil
				},
			}, nil
		},
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want %d; output=%q", len(lines), 3, stdout.String())
	}

	re := regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+$`)
	for i, line := range lines {
		if !re.MatchString(line) {
			t.Fatalf("line %d = %q, want 4-column table row", i, line)
		}
	}
	if !strings.Contains(lines[0], "CHAT_ID") || !strings.Contains(lines[0], "OWNER_UNION_ID") {
		t.Fatalf("header = %q, want table headers", lines[0])
	}
}

func TestListChatsTableOutputHandlesChineseNames(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewChatLister: func(config.Config) (feishu.ChatLister, error) {
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					return []feishu.ChatSummary{
						{ChatID: "oc_cn", Name: "研发群", Owner: feishu.ChatOwner{OpenID: "ou_cn", UnionID: "on_cn"}},
						{ChatID: "oc_en", Name: "Ops", Owner: feishu.ChatOwner{OpenID: "ou_en", UnionID: "on_en"}},
					}, nil
				},
			}, nil
		},
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want %d; output=%q", len(lines), 3, stdout.String())
	}
	if !regexp.MustCompile(`^oc_cn\s+研发群\s+ou_cn\s+on_cn$`).MatchString(lines[1]) {
		t.Fatalf("chinese row = %q, want aligned row", lines[1])
	}
	if !regexp.MustCompile(`^oc_en\s+Ops\s+ou_en\s+on_en$`).MatchString(lines[2]) {
		t.Fatalf("english row = %q, want aligned row", lines[2])
	}
}

func TestListChatsJSONOutput(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewChatLister: func(config.Config) (feishu.ChatLister, error) {
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					return []feishu.ChatSummary{{
						ChatID: "oc_xxx",
						Name:   "报警群",
						Owner:  feishu.ChatOwner{OpenID: "ou_xxx", UnionID: "on_xxx"},
					}}, nil
				},
			}, nil
		},
	})

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats", "--format", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var got struct {
		Items []struct {
			ChatID string `json:"chat_id"`
			Name   string `json:"name"`
			Owner  struct {
				OpenID  string `json:"open_id"`
				UnionID string `json:"union_id"`
			} `json:"owner"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, stdout.String())
	}
	if len(got.Items) != 1 {
		t.Fatalf("items len = %d, want %d", len(got.Items), 1)
	}
	if got.Items[0].ChatID != "oc_xxx" || got.Items[0].Name != "报警群" || got.Items[0].Owner.OpenID != "ou_xxx" || got.Items[0].Owner.UnionID != "on_xxx" {
		t.Fatalf("json items = %#v", got.Items)
	}
}

func TestListChatsEmptyOutputs(t *testing.T) {
	newCmd := func(format string) *cobra.Command {
		cmd := NewRootCmdWithDeps(Deps{
			LoadConfig: func(config.LoadOptions) (config.Config, error) {
				return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
			},
			NewChatLister: func(config.Config) (feishu.ChatLister, error) {
				return fakeChatLister{
					listChats: func(context.Context) ([]feishu.ChatSummary, error) {
						return []feishu.ChatSummary{}, nil
					},
				}, nil
			},
		})
		args := []string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats"}
		if format != "" {
			args = append(args, "--format", format)
		}
		cmd.SetArgs(args)
		return cmd
	}

	var tableOut bytes.Buffer
	tableCmd := newCmd("")
	tableCmd.SetOut(&tableOut)
	tableCmd.SetErr(&tableOut)
	if err := tableCmd.Execute(); err != nil {
		t.Fatalf("table Execute() error = %v", err)
	}
	if !regexp.MustCompile(`^CHAT_ID\s+NAME\s+OWNER_OPEN_ID\s+OWNER_UNION_ID$`).MatchString(strings.TrimSpace(tableOut.String())) {
		t.Fatalf("table output = %q, want header-only table", tableOut.String())
	}

	var jsonOut bytes.Buffer
	jsonCmd := newCmd("json")
	jsonCmd.SetOut(&jsonOut)
	jsonCmd.SetErr(&jsonOut)
	if err := jsonCmd.Execute(); err != nil {
		t.Fatalf("json Execute() error = %v", err)
	}
	if strings.TrimSpace(jsonOut.String()) != `{"items":[]}` {
		t.Fatalf("json output = %q, want %q", strings.TrimSpace(jsonOut.String()), `{"items":[]}`)
	}
}

func TestListChatsCommandLoadsConfigAndCallsListMethod(t *testing.T) {
	wantCfg := config.Config{AppID: "flag-id", AppSecret: "flag-secret"}
	loadCalled := false
	newListerCalled := false
	listCalled := false

	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			loadCalled = true
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
			return wantCfg, nil
		},
		NewChatLister: func(cfg config.Config) (feishu.ChatLister, error) {
			newListerCalled = true
			if cfg != wantCfg {
				t.Fatalf("NewChatLister() cfg = %#v, want %#v", cfg, wantCfg)
			}
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					listCalled = true
					return []feishu.ChatSummary{{ChatID: "oc_xxx", Name: "Example"}}, nil
				},
			}, nil
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			t.Fatal("NewMessenger should not be called by list chats")
			return nil, nil
		},
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if !loadCalled {
		t.Fatal("LoadConfig was not called")
	}
	if !newListerCalled {
		t.Fatal("NewChatLister was not called")
	}
	if !listCalled {
		t.Fatal("ListChats was not called")
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

func TestSendTextCommandRejectsMissingToType(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to", "ou_xxx", "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to-type is required" {
		t.Fatalf("error = %q, want %q", err, "--to-type is required")
	}
}

func TestSendTextCommandRejectsInvalidToType(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to-type", "email", "--to", "ou_xxx", "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "invalid --to-type") {
		t.Fatalf("error = %q, want invalid --to-type message", err)
	}
}

func TestSendTextCommandRejectsMissingTo(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to is required" {
		t.Fatalf("error = %q, want %q", err, "--to is required")
	}
}

func TestSendTextCommandRejectsBlankTo(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "   ", "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to must not be blank" {
		t.Fatalf("error = %q, want %q", err, "--to must not be blank")
	}
}

func TestSendTextCommandRejectsMissingText(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "ou_xxx"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--text is required" {
		t.Fatalf("error = %q, want %q", err, "--text is required")
	}
}

func TestSendTextCommandRejectsBlankText(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "   "})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--text must not be blank" {
		t.Fatalf("error = %q, want %q", err, "--text must not be blank")
	}
}

func TestSendTextCommandRejectsExtraArgs(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			t.Fatal("LoadConfig should not be called for extra args")
			return config.Config{}, nil
		},
	})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "hello", "extra"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
		t.Fatalf("error = %q, want extra-arg input error", err)
	}
}

func TestSendTextCommandReturnsErrorWhenOutputWriteFails(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewMessenger: func(cfg config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendText: func(context.Context, feishu.TextMessageInput) (feishu.MessageResult, error) {
					return feishu.MessageResult{MessageID: "om_xxx", MsgType: "text", ReceiveID: "ou_xxx", ReceiveIDType: "open_id"}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "hello"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want write error")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
	if !strings.Contains(err.Error(), "write result") {
		t.Fatalf("error = %q, want write-result error", err)
	}
}

func TestSendTextCommandUsesCommandContext(t *testing.T) {
	type contextKey string
	const key contextKey = "trace"

	wantCtx := context.WithValue(context.Background(), key, "ctx-value")
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewMessenger: func(cfg config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendText: func(ctx context.Context, input feishu.TextMessageInput) (feishu.MessageResult, error) {
					if got := ctx.Value(key); got != "ctx-value" {
						t.Fatalf("context value = %#v, want %q", got, "ctx-value")
					}
					return feishu.MessageResult{
						MessageID:     "om_xxx",
						MsgType:       "text",
						ReceiveID:     input.ReceiveID,
						ReceiveIDType: input.ReceiveIDType,
					}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "hello"})

	err := cmd.ExecuteContext(wantCtx)
	if err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
}

func TestSendFileCommandOutputsStableFields(t *testing.T) {
	type contextKey string
	const key contextKey = "trace"

	wantCtx := context.WithValue(context.Background(), key, "ctx-value")
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewMessenger: func(cfg config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendFile: func(ctx context.Context, input feishu.FileMessageInput) (feishu.MessageResult, error) {
					if got := ctx.Value(key); got != "ctx-value" {
						t.Fatalf("context value = %#v, want %q", got, "ctx-value")
					}
					if input.ReceiveIDType != "chat_id" || input.ReceiveID != "oc_xxx" || input.FilePath != "./report.pdf" {
						t.Fatalf("SendFile input = %#v", input)
					}
					return feishu.MessageResult{
						MessageID:     "om_file",
						MsgType:       "file",
						ReceiveID:     "oc_xxx",
						ReceiveIDType: "chat_id",
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
		"send", "file",
		"--to-type", "chat_id",
		"--to", "oc_xxx",
		"--path", "./report.pdf",
	})

	if err := cmd.ExecuteContext(wantCtx); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}

	want := "message_id=om_file\nmsg_type=file\nreceive_id=oc_xxx\nreceive_id_type=chat_id\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if strings.Contains(stdout.String(), "file_key") {
		t.Fatalf("stdout = %q, must not include file_key", stdout.String())
	}
}

func TestSendFileCommandRejectsExtraArgs(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			t.Fatal("LoadConfig should not be called for extra args")
			return config.Config{}, nil
		},
	})
	cmd.SetArgs([]string{"send", "file", "--to-type", "chat_id", "--to", "oc_xxx", "--path", "./report.pdf", "extra"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "unknown command") && !strings.Contains(err.Error(), "accepts 0 arg") {
		t.Fatalf("error = %q, want extra-arg input error", err)
	}
}

func TestSendFileCommandReturnsErrorWhenOutputWriteFails(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewMessenger: func(cfg config.Config) (feishu.Messenger, error) {
			return fakeMessenger{
				sendFile: func(context.Context, feishu.FileMessageInput) (feishu.MessageResult, error) {
					return feishu.MessageResult{MessageID: "om_file", MsgType: "file", ReceiveID: "oc_xxx", ReceiveIDType: "chat_id"}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "file", "--to-type", "chat_id", "--to", "oc_xxx", "--path", "./report.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want write error")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
	if !strings.Contains(err.Error(), "write result") {
		t.Fatalf("error = %q, want write-result error", err)
	}
}

func TestSendFileCommandRejectsMissingToType(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to", "oc_xxx", "--path", "./report.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to-type is required" {
		t.Fatalf("error = %q, want %q", err, "--to-type is required")
	}
}

func TestSendFileCommandRejectsMissingTo(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to-type", "chat_id", "--path", "./report.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to is required" {
		t.Fatalf("error = %q, want %q", err, "--to is required")
	}
}

func TestSendFileCommandRejectsBlankTo(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to-type", "chat_id", "--to", "   ", "--path", "./report.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--to must not be blank" {
		t.Fatalf("error = %q, want %q", err, "--to must not be blank")
	}
}

func TestSendFileCommandRejectsInvalidToType(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to-type", "email", "--to", "ou_xxx", "--path", "/tmp/report.pdf"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "invalid --to-type") {
		t.Fatalf("error = %q, want invalid --to-type message", err)
	}
}

func TestSendFileCommandRejectsMissingPath(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to-type", "open_id", "--to", "ou_xxx"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--path is required" {
		t.Fatalf("error = %q, want %q", err, "--path is required")
	}
}

func TestSendFileCommandRejectsBlankPath(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "file", "--to-type", "open_id", "--to", "ou_xxx", "--path", "   "})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--path must not be blank" {
		t.Fatalf("error = %q, want %q", err, "--path must not be blank")
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

func TestExitCodeMapsClientErrorsToThree(t *testing.T) {
	err := classifyRunError(&feishu.ClientError{Op: "send_text", Message: "message api is not configured"})
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
}

func TestExitCodeMapsUncategorizedErrorsToThree(t *testing.T) {
	err := errors.New("write result: write failed")
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

type fakeChatLister struct {
	listChats func(context.Context) ([]feishu.ChatSummary, error)
}

func (f fakeChatLister) ListChats(ctx context.Context) ([]feishu.ChatSummary, error) {
	return f.listChats(ctx)
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}
