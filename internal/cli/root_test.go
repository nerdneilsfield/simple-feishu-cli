package cli

import (
	"bytes"
	"testing"
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
