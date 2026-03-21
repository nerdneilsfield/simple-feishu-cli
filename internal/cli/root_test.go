package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mattn/go-runewidth"

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
		"-c, --config string",
		"-i, --app-id string",
		"-s, --app-secret string",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestShortGlobalFlagsReachConfigLoader(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" || opts.ConfigPath != "./feishu.yaml" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
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

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"-i", "flag-id", "-s", "flag-secret", "-c", "./feishu.yaml", "list", "chats"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
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

	header, chineseRow, englishRow := lines[0], lines[1], lines[2]
	for _, tc := range []struct {
		name string
		line string
		want []string
	}{
		{name: "chinese", line: chineseRow, want: []string{"oc_cn", "研发群", "ou_cn", "on_cn"}},
		{name: "english", line: englishRow, want: []string{"oc_en", "Ops", "ou_en", "on_en"}},
	} {
		columns := regexp.MustCompile(`\s{2,}`).Split(tc.line, -1)
		if len(columns) != 4 {
			t.Fatalf("%s columns = %#v, want 4 table columns", tc.name, columns)
		}
		for i, want := range tc.want {
			if columns[i] != want {
				t.Fatalf("%s column %d = %q, want %q; line=%q", tc.name, i, columns[i], want, tc.line)
			}
		}
	}

	headerOpenCol := displayColumn(header, "OWNER_OPEN_ID")
	headerUnionCol := displayColumn(header, "OWNER_UNION_ID")
	cnOpenCol := displayColumn(chineseRow, "ou_cn")
	enOpenCol := displayColumn(englishRow, "ou_en")
	cnUnionCol := displayColumn(chineseRow, "on_cn")
	enUnionCol := displayColumn(englishRow, "on_en")

	if cnOpenCol != enOpenCol || cnOpenCol != headerOpenCol {
		t.Fatalf("OWNER_OPEN_ID column starts differ: header=%d chinese=%d english=%d\n%s", headerOpenCol, cnOpenCol, enOpenCol, stdout.String())
	}
	if cnUnionCol != enUnionCol || cnUnionCol != headerUnionCol {
		t.Fatalf("OWNER_UNION_ID column starts differ: header=%d chinese=%d english=%d\n%s", headerUnionCol, cnUnionCol, enUnionCol, stdout.String())
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

	got := strings.TrimSpace(stdout.String())
	want := `{"items":[{"chat_id":"oc_xxx","name":"报警群","owner":{"open_id":"ou_xxx","union_id":"on_xxx"}}]}`
	if got != want {
		t.Fatalf("json output = %q, want %q", got, want)
	}

	var decoded map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v; output=%q", err, stdout.String())
	}
}

func TestListChatsTableOutputWriteFailureReturnsExitCodeThree(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewChatLister: func(config.Config) (feishu.ChatLister, error) {
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					return []feishu.ChatSummary{{ChatID: "oc_xxx", Name: "Ops", Owner: feishu.ChatOwner{OpenID: "ou_xxx", UnionID: "on_xxx"}}}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want write error")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
	if !strings.Contains(err.Error(), "write list chats") {
		t.Fatalf("error = %q, want write-list-chats error", err)
	}
}

func TestListChatsJSONOutputWriteFailureReturnsExitCodeThree(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewChatLister: func(config.Config) (feishu.ChatLister, error) {
			return fakeChatLister{
				listChats: func(context.Context) ([]feishu.ChatSummary, error) {
					return []feishu.ChatSummary{{ChatID: "oc_xxx", Name: "Ops", Owner: feishu.ChatOwner{OpenID: "ou_xxx", UnionID: "on_xxx"}}}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "list", "chats", "--format", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want write error")
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 3)
	}
	if !strings.Contains(err.Error(), "write list chats") {
		t.Fatalf("error = %q, want write-list-chats error", err)
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

func TestSendPostCommandIsDiscoverableInHelp(t *testing.T) {
	cmd := NewRootCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"send", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := stdout.String()
	if !strings.Contains(help, "post        Send a post message") {
		t.Fatalf("help output missing %q:\n%s", "post        Send a post message", help)
	}
}

func TestSendPostCommandRejectsMissingFile(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--file is required" {
		t.Fatalf("error = %q, want %q", err, "--file is required")
	}
}

func TestSendPostCommandRejectsInvalidToType(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "email", "--to", "oc_xxx", "--file", "./post.json"})

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

func TestSendPostCommandRejectsBlankTo(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "   ", "--file", "./post.json"})

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

func TestSendPostCommandRejectsBlankFile(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", "   "})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--file must not be blank" {
		t.Fatalf("error = %q, want %q", err, "--file must not be blank")
	}
}

func TestSendPostCommandRejectsMalformedJSON(t *testing.T) {
	path := writeTempCLIFile(t, "post.json", "not-json")
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "parse JSON file") {
		t.Fatalf("error = %q, want parse-json error", err)
	}
}

func TestSendPostCommandRejectsNonObjectJSON(t *testing.T) {
	path := writeTempCLIFile(t, "post.json", `[]`)
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "post JSON must be an object" {
		t.Fatalf("error = %q, want %q", err, "post JSON must be an object")
	}
}

func TestSendPostCommandReturnsLocalFileErrorForMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want local file error")
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 4)
	}
	if !strings.Contains(err.Error(), "read_file") {
		t.Fatalf("error = %q, want read_file error", err)
	}
}

func TestSendPostCommandReturnsLocalFileErrorForDirectoryPath(t *testing.T) {
	path := t.TempDir()
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want local file error")
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 4)
	}
	if !strings.Contains(err.Error(), "read_file") {
		t.Fatalf("error = %q, want read_file error", err)
	}
}

func TestSendPostCommandReturnsErrorWhenOutputWriteFails(t *testing.T) {
	path := writeTempCLIFile(t, "post.json", `{"zh_cn":{"title":"Alarm","content":[[{"tag":"text","text":"hello"}]]}}`)
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			return config.Config{AppID: "flag-id", AppSecret: "flag-secret"}, nil
		},
		NewPostSender: func(cfg config.Config) (feishu.PostSender, error) {
			return fakePostSender{
				sendPost: func(context.Context, feishu.PostMessageInput) (feishu.MessageResult, error) {
					return feishu.MessageResult{MessageID: "om_post", MsgType: "post", ReceiveID: "oc_xxx", ReceiveIDType: "chat_id"}, nil
				},
			}, nil
		},
	})
	cmd.SetOut(failingWriter{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

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

func TestSendPostCommandLoadsConfigReadsFileAndCallsSendPost(t *testing.T) {
	path := writeTempCLIFile(t, "post.json", `{"zh_cn":{"title":"Alarm","content":[[{"tag":"text","text":"hello"}]]}}`)
	wantCfg := config.Config{AppID: "flag-id", AppSecret: "flag-secret"}
	loadCalled := false
	newSenderCalled := false
	sendCalled := false

	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			loadCalled = true
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
			return wantCfg, nil
		},
		NewPostSender: func(cfg config.Config) (feishu.PostSender, error) {
			newSenderCalled = true
			if cfg != wantCfg {
				t.Fatalf("NewPostSender() cfg = %#v, want %#v", cfg, wantCfg)
			}
			return fakePostSender{
				sendPost: func(_ context.Context, input feishu.PostMessageInput) (feishu.MessageResult, error) {
					sendCalled = true
					if input.ReceiveIDType != "chat_id" || input.ReceiveID != "oc_xxx" {
						t.Fatalf("SendPost input = %#v", input)
					}
					if string(input.Post) != `{"zh_cn":{"title":"Alarm","content":[[{"tag":"text","text":"hello"}]]}}` {
						t.Fatalf("SendPost content = %q", string(input.Post))
					}
					return feishu.MessageResult{MessageID: "om_post", MsgType: "post", ReceiveID: "oc_xxx", ReceiveIDType: "chat_id"}, nil
				},
			}, nil
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			t.Fatal("NewMessenger should not be called by send post")
			return nil, nil
		},
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "post", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !loadCalled {
		t.Fatal("LoadConfig was not called")
	}
	if !newSenderCalled {
		t.Fatal("NewPostSender was not called")
	}
	if !sendCalled {
		t.Fatal("SendPost was not called")
	}
	want := "message_id=om_post\nmsg_type=post\nreceive_id=oc_xxx\nreceive_id_type=chat_id\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestSendMDCommandIsDiscoverableInHelp(t *testing.T) {
	cmd := NewRootCmd()

	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"send", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	help := stdout.String()
	for _, want := range []string{
		"md          Convert Markdown to post and send it",
	} {
		if !strings.Contains(help, want) {
			t.Fatalf("help output missing %q:\n%s", want, help)
		}
	}
}

func TestSendMDCommandRejectsMissingFile(t *testing.T) {
	cmd := NewRootCmdWithDeps(Deps{})
	cmd.SetArgs([]string{"send", "md", "--to-type", "chat_id", "--to", "oc_xxx"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if err.Error() != "--file is required" {
		t.Fatalf("error = %q, want %q", err, "--file is required")
	}
}

func TestSendMDCommandLoadsConfigConvertsMarkdownAndCallsSendPost(t *testing.T) {
	path := writeTempCLIFile(t, "notice.md", "# Notice\n\nHello world.\n")
	wantCfg := config.Config{AppID: "flag-id", AppSecret: "flag-secret"}
	loadCalled := false
	newSenderCalled := false
	sendCalled := false

	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(opts config.LoadOptions) (config.Config, error) {
			loadCalled = true
			if opts.AppID != "flag-id" || opts.AppSecret != "flag-secret" {
				t.Fatalf("LoadConfig() got %#v", opts)
			}
			return wantCfg, nil
		},
		NewPostSender: func(cfg config.Config) (feishu.PostSender, error) {
			newSenderCalled = true
			if cfg != wantCfg {
				t.Fatalf("NewPostSender() cfg = %#v, want %#v", cfg, wantCfg)
			}
			return fakePostSender{
				sendPost: func(_ context.Context, input feishu.PostMessageInput) (feishu.MessageResult, error) {
					sendCalled = true
					if input.ReceiveIDType != "chat_id" || input.ReceiveID != "oc_xxx" {
						t.Fatalf("SendPost input = %#v", input)
					}
					if string(input.Post) != `{"zh_cn":{"title":"Notice","content":[[{"tag":"text","text":"Hello world."}]]}}` {
						t.Fatalf("SendPost content = %q", string(input.Post))
					}
					return feishu.MessageResult{MessageID: "om_md", MsgType: "post", ReceiveID: "oc_xxx", ReceiveIDType: "chat_id"}, nil
				},
			}, nil
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			t.Fatal("NewMessenger should not be called by send md")
			return nil, nil
		},
	})
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--app-id", "flag-id", "--app-secret", "flag-secret", "send", "md", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !loadCalled {
		t.Fatal("LoadConfig was not called")
	}
	if !newSenderCalled {
		t.Fatal("NewPostSender was not called")
	}
	if !sendCalled {
		t.Fatal("SendPost was not called")
	}
	want := "message_id=om_md\nmsg_type=post\nreceive_id=oc_xxx\nreceive_id_type=chat_id\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestSendMDCommandRejectsUnsupportedMarkdown(t *testing.T) {
	path := writeTempCLIFile(t, "unsupported.md", "![image](https://example.com/x.png)\n")
	cmd := NewRootCmdWithDeps(Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			t.Fatal("LoadConfig should not be called for unsupported markdown")
			return config.Config{}, nil
		},
		NewPostSender: func(config.Config) (feishu.PostSender, error) {
			t.Fatal("NewPostSender should not be called for unsupported markdown")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"send", "md", "--to-type", "chat_id", "--to", "oc_xxx", "--file", path})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want parameter error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode(err) = %d, want %d", got, 2)
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %q, want unsupported markdown error", err)
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

func displayColumn(line, token string) int {
	idx := strings.Index(line, token)
	if idx < 0 {
		return -1
	}
	return runewidth.StringWidth(line[:idx])
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

type fakePostSender struct {
	sendPost func(context.Context, feishu.PostMessageInput) (feishu.MessageResult, error)
}

func (f fakePostSender) SendPost(ctx context.Context, input feishu.PostMessageInput) (feishu.MessageResult, error) {
	return f.sendPost(ctx, input)
}

func writeTempCLIFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}
