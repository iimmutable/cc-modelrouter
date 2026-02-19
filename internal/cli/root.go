// Package cli implements the command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the application version.
var Version = "0.1.0"

// NewRootCommand creates the root command.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ccrouter",
		Short:   "Claude Code Model Router",
		Version: Version,
	}

	cmd.AddCommand(NewCodeCommand())
	cmd.AddCommand(NewStartCommand())
	cmd.AddCommand(NewStopCommand())
	cmd.AddCommand(NewRestartCommand())
	cmd.AddCommand(NewStatusCommand())
	cmd.AddCommand(NewCleanCommand())
	cmd.AddCommand(NewConfigCommand())
	cmd.AddCommand(NewLogsCommand())
	cmd.AddCommand(NewUsageCommand())

	return cmd
}

// Execute runs the CLI.
func Execute() {
	cmd := NewRootCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
