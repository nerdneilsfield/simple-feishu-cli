package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/nerdneilsfield/simple-feishu-cli/internal/cli"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/config"
	"github.com/nerdneilsfield/simple-feishu-cli/internal/feishu"
	"github.com/spf13/cobra"
)

func TestRunPrintsErrorAndReturnsMappedExitCode(t *testing.T) {
	cmd := cli.NewRootCmdWithDeps(cli.Deps{
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			return config.Config{}, errors.New("missing app_id")
		},
		NewMessenger: func(config.Config) (feishu.Messenger, error) {
			t.Fatal("NewMessenger should not be called when config loading fails")
			return nil, nil
		},
	})
	cmd.SetArgs([]string{"send", "text", "--to-type", "open_id", "--to", "ou_xxx", "--text", "hello"})

	var stderr bytes.Buffer
	code := run(&stderr, cmd)

	if code != 3 {
		t.Fatalf("run() code = %d, want %d", code, 3)
	}

	output := stderr.String()
	if !strings.Contains(output, "error: missing app_id") {
		t.Fatalf("stderr = %q, want useful error output", output)
	}
}

func TestRunReturnsZeroWithoutWritingError(t *testing.T) {
	cmd := &cobra.Command{
		Use:           "feishu",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return nil
		},
	}

	var stderr bytes.Buffer
	code := run(&stderr, cmd)

	if code != 0 {
		t.Fatalf("run() code = %d, want %d", code, 0)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty stderr", stderr.String())
	}
}
