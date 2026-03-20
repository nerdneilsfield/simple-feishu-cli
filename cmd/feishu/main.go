package main

import (
	"os"

	"github.com/nerdneilsfield/simple-feishu-cli/internal/cli"
)

func main() {
	if err := cli.NewRootCmd().Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
