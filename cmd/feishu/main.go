package main

import (
	"fmt"
	"io"
	"os"

	"github.com/nerdneilsfield/simple-feishu-cli/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	os.Exit(run(os.Stderr, cli.NewRootCmd()))
}

func run(stderr io.Writer, cmd *cobra.Command) int {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return cli.ExitCode(err)
	}

	return 0
}
