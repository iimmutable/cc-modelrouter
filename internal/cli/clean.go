package cli

import (
	"fmt"

	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/spf13/cobra"
)

// NewCleanCommand creates the clean command.
func NewCleanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Remove stale instance files",
		Long: `Removes instance metadata files for processes that are no longer running.

Over time, instance metadata files can accumulate if processes terminate abnormally.
This command cleans up these stale files.

By default, only removes instances whose processes are no longer running.

Flags:
  -a, --all    Remove all instance files including running ones.
                Use with caution - this will remove metadata for active instances.

Examples:
  # Remove stale instance files only
  ccrouter clean

  # Remove all instance files (use with caution)
  ccrouter clean --all`,
		RunE: runClean,
	}

	cmd.Flags().BoolP("all", "a", false, "Remove all instance files including running ones")

	return cmd
}

func runClean(cmd *cobra.Command, args []string) error {
	cleanAll, _ := cmd.Flags().GetBool("all")

	// List all instances
	instances, err := daemon.ListInstances()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instances) == 0 {
		fmt.Println("No instances to clean")
		return nil
	}

	removed := 0
	for _, inst := range instances {
		shouldRemove := cleanAll || !daemon.IsRunning(inst)

		if shouldRemove {
			if err := daemon.DeleteInstance(inst.ID); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to remove %s: %v\n", inst.ID, err)
			} else {
				fmt.Printf("Removed stale instance: %s\n", inst.ID)
				removed++
			}
		}
	}

	if removed == 0 {
		fmt.Println("No stale instances to remove")
	} else {
		fmt.Printf("Removed %d stale instance(s)\n", removed)
	}

	return nil
}
