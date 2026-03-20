package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/feishu"
	"github.com/spf13/cobra"
)

type Deps struct {
	LoadConfig   func(config.LoadOptions) (config.Config, error)
	NewMessenger func(config.Config) (feishu.Messenger, error)
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
		LoadConfig:   config.Load,
		NewMessenger: newMessenger,
	})
}

func NewRootCmdWithDeps(deps Deps) *cobra.Command {
	if deps.LoadConfig == nil {
		deps.LoadConfig = config.Load
	}
	if deps.NewMessenger == nil {
		deps.NewMessenger = newMessenger
	}

	f := &flags{}
	cmd := &cobra.Command{
		Use:           "feishu",
		Short:         "A simple Feishu CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	cmd.PersistentFlags().StringVar(&f.appID, "app-id", "", "Feishu app ID")
	cmd.PersistentFlags().StringVar(&f.appSecret, "app-secret", "", "Feishu app secret")
	cmd.PersistentFlags().StringVar(&f.configPath, "config", "", "Path to config file")
	cmd.SetHelpCommand(newHelpCmd(cmd))
	cmd.AddCommand(newSendCmd(deps, f))

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

func newMessenger(cfg config.Config) (feishu.Messenger, error) {
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
