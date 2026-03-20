package cli

import "github.com/spf13/cobra"

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "feishu",
		Short:         "A simple Feishu CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	return cmd
}
