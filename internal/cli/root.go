package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/nerdneilsfield/simple-feishu-cli/config"
	"github.com/nerdneilsfield/simple-feishu-cli/feishu"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/markdown"
	"github.com/spf13/cobra"
)

type Deps struct {
	LoadConfig    func(config.LoadOptions) (config.Config, error)
	NewMessenger  func(config.Config) (feishu.Messenger, error)
	NewPostSender func(config.Config) (feishu.PostSender, error)
	NewChatLister func(config.Config) (feishu.ChatLister, error)
}

type flags struct {
	appID      string
	appSecret  string
	configPath string
}

type cliError struct {
	code int
	err  error
}

func (e *cliError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *cliError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func NewRootCmd() *cobra.Command {
	return NewRootCmdWithDeps(Deps{
		LoadConfig:    config.Load,
		NewMessenger:  newMessenger,
		NewPostSender: newPostSender,
		NewChatLister: newChatLister,
	})
}

func NewRootCmdWithDeps(deps Deps) *cobra.Command {
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.Load
	}
	if deps.NewMessenger == nil {
		deps.NewMessenger = newMessenger
	}
	if deps.NewPostSender == nil {
		deps.NewPostSender = newPostSender
	}
	if deps.NewChatLister == nil {
		deps.NewChatLister = newChatLister
	}

	f := &flags{}
	cmd := &cobra.Command{
		Use:           "feishu",
		Short:         "A simple Feishu CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().StringVarP(&f.appID, "app-id", "i", "", "Feishu app ID")
	cmd.PersistentFlags().StringVarP(&f.appSecret, "app-secret", "s", "", "Feishu app secret")
	cmd.PersistentFlags().StringVarP(&f.configPath, "config", "c", "", "Path to config file")
	cmd.SetHelpCommand(newHelpCmd(cmd))
	cmd.AddCommand(
		newSendCmd(deps, f),
		newListCmd(deps, f),
	)

	return cmd
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}

	var coded *cliError
	if errors.As(err, &coded) {
		return coded.code
	}
	if isCobraInputError(err) {
		return 2
	}

	return 3
}

func isCobraInputError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.TrimSpace(err.Error())
	switch {
	case strings.HasPrefix(msg, "unknown flag:"),
		strings.HasPrefix(msg, "unknown shorthand flag:"),
		strings.HasPrefix(msg, "unknown command "),
		strings.HasPrefix(msg, "unknown help topic "),
		strings.HasPrefix(msg, "accepts "),
		strings.HasPrefix(msg, "requires at least "),
		strings.HasPrefix(msg, "requires at most "),
		strings.HasPrefix(msg, "accepts between "),
		strings.HasPrefix(msg, "required flag(s) "):
		return true
	default:
		return false
	}
}

func newHelpCmd(root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "help [command ...]",
		Short: "Help about any command",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}

			target, _, err := root.Find(args)
			if err != nil {
				return &cliError{code: 2, err: fmt.Errorf("unknown help topic %q", strings.Join(args, " "))}
			}
			return target.Help()
		},
	}
}

func newListCmd(deps Deps, f *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Feishu resources",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newListChatsCmd(deps, f))

	return cmd
}

func newListChatsCmd(deps Deps, f *flags) *cobra.Command {
	format := "table"

	cmd := &cobra.Command{
		Use:   "chats",
		Short: "List chats joined by the app bot",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !isAllowedListFormat(format) {
				return &cliError{code: 2, err: fmt.Errorf("invalid --format %q; allowed values: table, json", format)}
			}

			cfg, err := deps.LoadConfig(config.LoadOptions{
				AppID:      f.appID,
				AppSecret:  f.appSecret,
				ConfigPath: f.configPath,
			})
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			lister, err := deps.NewChatLister(cfg)
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			chats, err := lister.ListChats(cmd.Context())
			if err != nil {
				return classifyRunError(err)
			}

			var writeErr error
			switch format {
			case "json":
				writeErr = writeChatJSON(cmd.OutOrStdout(), chats)
			default:
				writeErr = writeChatTable(cmd.OutOrStdout(), chats)
			}
			if writeErr != nil {
				return fmt.Errorf("write list chats: %w", writeErr)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", format, "Output format")

	return cmd
}

func newSendCmd(deps Deps, f *flags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send messages or files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSendTextCmd(deps, f),
		newSendFileCmd(deps, f),
		newSendPostCmd(deps, f),
		newSendMDCmd(deps, f),
	)

	return cmd
}

func newSendTextCmd(deps Deps, f *flags) *cobra.Command {
	var toType, toID, text string

	cmd := &cobra.Command{
		Use:   "text",
		Short: "Send a text message",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if toType == "" {
				return &cliError{code: 2, err: errors.New("--to-type is required")}
			}
			if !isAllowedReceiveIDType(toType) {
				return &cliError{code: 2, err: fmt.Errorf("invalid --to-type %q; allowed values: open_id, user_id, union_id, chat_id", toType)}
			}
			if toID == "" {
				return &cliError{code: 2, err: errors.New("--to is required")}
			}
			if strings.TrimSpace(toID) == "" {
				return &cliError{code: 2, err: errors.New("--to must not be blank")}
			}
			if text == "" {
				return &cliError{code: 2, err: errors.New("--text is required")}
			}
			if strings.TrimSpace(text) == "" {
				return &cliError{code: 2, err: errors.New("--text must not be blank")}
			}

			cfg, err := deps.LoadConfig(config.LoadOptions{
				AppID:      f.appID,
				AppSecret:  f.appSecret,
				ConfigPath: f.configPath,
			})
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			messenger, err := deps.NewMessenger(cfg)
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			result, err := messenger.SendText(cmd.Context(), feishu.TextMessageInput{
				ReceiveIDType: toType,
				ReceiveID:     toID,
				Text:          text,
			})
			if err != nil {
				return classifyRunError(err)
			}

			if err := writeResult(cmd.OutOrStdout(), result); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&toType, "to-type", "", "Receive ID type")
	cmd.Flags().StringVar(&toID, "to", "", "Receive ID")
	cmd.Flags().StringVar(&text, "text", "", "Text content")

	return cmd
}

func newSendPostCmd(deps Deps, f *flags) *cobra.Command {
	var toType, toID, path string

	cmd := &cobra.Command{
		Use:   "post",
		Short: "Send a post message",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if toType == "" {
				return &cliError{code: 2, err: errors.New("--to-type is required")}
			}
			if !isAllowedReceiveIDType(toType) {
				return &cliError{code: 2, err: fmt.Errorf("invalid --to-type %q; allowed values: open_id, user_id, union_id, chat_id", toType)}
			}
			if toID == "" {
				return &cliError{code: 2, err: errors.New("--to is required")}
			}
			if strings.TrimSpace(toID) == "" {
				return &cliError{code: 2, err: errors.New("--to must not be blank")}
			}
			if path == "" {
				return &cliError{code: 2, err: errors.New("--file is required")}
			}
			if strings.TrimSpace(path) == "" {
				return &cliError{code: 2, err: errors.New("--file must not be blank")}
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return &cliError{code: 4, err: &feishu.LocalFileError{Op: "read_file", Path: path, Err: err}}
			}

			var payload any
			if err := json.Unmarshal(content, &payload); err != nil {
				return &cliError{code: 2, err: fmt.Errorf("parse JSON file %q: %w", path, err)}
			}
			if _, ok := payload.(map[string]any); !ok {
				return &cliError{code: 2, err: errors.New("post JSON must be an object")}
			}

			cfg, err := deps.LoadConfig(config.LoadOptions{
				AppID:      f.appID,
				AppSecret:  f.appSecret,
				ConfigPath: f.configPath,
			})
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			sender, err := deps.NewPostSender(cfg)
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			result, err := sender.SendPost(cmd.Context(), feishu.PostMessageInput{
				ReceiveIDType: toType,
				ReceiveID:     toID,
				Post:          json.RawMessage(content),
			})
			if err != nil {
				return classifyRunError(err)
			}

			if err := writeResult(cmd.OutOrStdout(), result); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&toType, "to-type", "", "Receive ID type")
	cmd.Flags().StringVar(&toID, "to", "", "Receive ID")
	cmd.Flags().StringVar(&path, "file", "", "Local post JSON file")

	return cmd
}

func newSendMDCmd(deps Deps, f *flags) *cobra.Command {
	var toType, toID, path string

	cmd := &cobra.Command{
		Use:   "md",
		Short: "Convert Markdown to post and send it",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if toType == "" {
				return &cliError{code: 2, err: errors.New("--to-type is required")}
			}
			if !isAllowedReceiveIDType(toType) {
				return &cliError{code: 2, err: fmt.Errorf("invalid --to-type %q; allowed values: open_id, user_id, union_id, chat_id", toType)}
			}
			if toID == "" {
				return &cliError{code: 2, err: errors.New("--to is required")}
			}
			if strings.TrimSpace(toID) == "" {
				return &cliError{code: 2, err: errors.New("--to must not be blank")}
			}
			if path == "" {
				return &cliError{code: 2, err: errors.New("--file is required")}
			}
			if strings.TrimSpace(path) == "" {
				return &cliError{code: 2, err: errors.New("--file must not be blank")}
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return &cliError{code: 4, err: &feishu.LocalFileError{Op: "read_file", Path: path, Err: err}}
			}

			post, err := markdown.ConvertToFeishuPost(content)
			if err != nil {
				return &cliError{code: 2, err: fmt.Errorf("convert Markdown file %q: %w", path, err)}
			}

			cfg, err := deps.LoadConfig(config.LoadOptions{
				AppID:      f.appID,
				AppSecret:  f.appSecret,
				ConfigPath: f.configPath,
			})
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			sender, err := deps.NewPostSender(cfg)
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			result, err := sender.SendPost(cmd.Context(), feishu.PostMessageInput{
				ReceiveIDType: toType,
				ReceiveID:     toID,
				Post:          post,
			})
			if err != nil {
				return classifyRunError(err)
			}

			if err := writeResult(cmd.OutOrStdout(), result); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&toType, "to-type", "", "Receive ID type")
	cmd.Flags().StringVar(&toID, "to", "", "Receive ID")
	cmd.Flags().StringVar(&path, "file", "", "Local Markdown file")

	return cmd
}

func newSendFileCmd(deps Deps, f *flags) *cobra.Command {
	var toType, toID, path string

	cmd := &cobra.Command{
		Use:   "file",
		Short: "Upload and send a file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if toType == "" {
				return &cliError{code: 2, err: errors.New("--to-type is required")}
			}
			if !isAllowedReceiveIDType(toType) {
				return &cliError{code: 2, err: fmt.Errorf("invalid --to-type %q; allowed values: open_id, user_id, union_id, chat_id", toType)}
			}
			if toID == "" {
				return &cliError{code: 2, err: errors.New("--to is required")}
			}
			if strings.TrimSpace(toID) == "" {
				return &cliError{code: 2, err: errors.New("--to must not be blank")}
			}
			if path == "" {
				return &cliError{code: 2, err: errors.New("--path is required")}
			}
			if strings.TrimSpace(path) == "" {
				return &cliError{code: 2, err: errors.New("--path must not be blank")}
			}

			cfg, err := deps.LoadConfig(config.LoadOptions{
				AppID:      f.appID,
				AppSecret:  f.appSecret,
				ConfigPath: f.configPath,
			})
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			messenger, err := deps.NewMessenger(cfg)
			if err != nil {
				return &cliError{code: 3, err: err}
			}

			result, err := messenger.SendFile(cmd.Context(), feishu.FileMessageInput{
				ReceiveIDType: toType,
				ReceiveID:     toID,
				FilePath:      path,
			})
			if err != nil {
				return classifyRunError(err)
			}

			if err := writeResult(cmd.OutOrStdout(), result); err != nil {
				return fmt.Errorf("write result: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&toType, "to-type", "", "Receive ID type")
	cmd.Flags().StringVar(&toID, "to", "", "Receive ID")
	cmd.Flags().StringVar(&path, "path", "", "Local file path")

	return cmd
}

func isAllowedReceiveIDType(value string) bool {
	switch value {
	case "open_id", "user_id", "union_id", "chat_id":
		return true
	default:
		return false
	}
}

func isAllowedListFormat(value string) bool {
	switch value {
	case "table", "json":
		return true
	default:
		return false
	}
}

func newPostSender(cfg config.Config) (feishu.PostSender, error) {
	return feishu.NewClient(cfg)
}

func newMessenger(cfg config.Config) (feishu.Messenger, error) {
	return feishu.NewClient(cfg)
}

func newChatLister(cfg config.Config) (feishu.ChatLister, error) {
	return feishu.NewClient(cfg)
}

func classifyRunError(err error) error {
	var fileErr *feishu.LocalFileError
	if errors.As(err, &fileErr) {
		return &cliError{code: 4, err: err}
	}

	var clientErr *feishu.ClientError
	if errors.As(err, &clientErr) {
		return &cliError{code: 3, err: err}
	}

	var apiErr *feishu.APIError
	if errors.As(err, &apiErr) {
		return &cliError{code: 10, err: err}
	}

	return &cliError{code: 10, err: err}
}

type chatListJSON struct {
	Items []chatSummaryJSON `json:"items"`
}

type chatSummaryJSON struct {
	ChatID string        `json:"chat_id"`
	Name   string        `json:"name"`
	Owner  chatOwnerJSON `json:"owner"`
}

type chatOwnerJSON struct {
	OpenID  string `json:"open_id"`
	UnionID string `json:"union_id"`
}

func writeChatTable(w io.Writer, chats []feishu.ChatSummary) error {
	headers := []string{"CHAT_ID", "NAME", "OWNER_OPEN_ID", "OWNER_UNION_ID"}
	rows := make([][]string, 0, len(chats))
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = runewidth.StringWidth(header)
	}

	for _, chat := range chats {
		row := []string{chat.ChatID, chat.Name, chat.Owner.OpenID, chat.Owner.UnionID}
		rows = append(rows, row)
		for i, cell := range row {
			if width := runewidth.StringWidth(cell); width > widths[i] {
				widths[i] = width
			}
		}
	}

	if _, err := io.WriteString(w, formatChatTableRow(headers, widths)); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := io.WriteString(w, formatChatTableRow(row, widths)); err != nil {
			return err
		}
	}
	return nil
}

func formatChatTableRow(cells []string, widths []int) string {
	var b strings.Builder
	for i, cell := range cells {
		b.WriteString(cell)
		if i == len(cells)-1 {
			b.WriteByte('\n')
			break
		}

		padding := widths[i] - runewidth.StringWidth(cell) + 2
		if padding < 2 {
			padding = 2
		}
		b.WriteString(strings.Repeat(" ", padding))
	}
	return b.String()
}

func writeChatJSON(w io.Writer, chats []feishu.ChatSummary) error {
	items := make([]chatSummaryJSON, 0, len(chats))
	for _, chat := range chats {
		items = append(items, chatSummaryJSON{
			ChatID: chat.ChatID,
			Name:   chat.Name,
			Owner: chatOwnerJSON{
				OpenID:  chat.Owner.OpenID,
				UnionID: chat.Owner.UnionID,
			},
		})
	}
	return json.NewEncoder(w).Encode(chatListJSON{Items: items})
}

func writeResult(w io.Writer, result feishu.MessageResult) error {
	if _, err := fmt.Fprintf(w, "message_id=%s\n", result.MessageID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "msg_type=%s\n", result.MsgType); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "receive_id=%s\n", result.ReceiveID); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "receive_id_type=%s\n", result.ReceiveIDType); err != nil {
		return err
	}
	return nil
}
