package cli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/spf13/cobra"
)

// NewRestartCommand creates the restart command.
func NewRestartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart [instance-id]",
		Short: "Restart a router instance",
		Long:  "Restarts a running router instance. If no instance ID is provided, restarts all running instances.",
		RunE:  runRestart,
	}

	cmd.Flags().StringP("config", "c", "", "Path to config file for restart")

	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	// List all instances
	instances, err := daemon.ListInstances()
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	if len(instances) == 0 {
		fmt.Println("No instances found")
		return nil
	}

	// Filter to only running instances
	var runningInstances []*daemon.InstanceMetadata
	for _, inst := range instances {
		if daemon.IsRunning(inst) {
			runningInstances = append(runningInstances, inst)
		}
	}

	if len(runningInstances) == 0 {
		fmt.Println("No running instances to restart")
		return nil
	}

	// If instance ID provided, restart only that instance
	if len(args) > 0 {
		instanceID := args[0]
		found := false
		for _, inst := range runningInstances {
			if inst.ID == instanceID {
				found = true
				if err := restartInstance(inst); err != nil {
					return err
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("running instance not found: %s", instanceID)
		}
		return nil
	}

	// Restart all running instances
	fmt.Printf("Restarting %d instance(s)...\n", len(runningInstances))

	for _, inst := range runningInstances {
		if err := restartInstance(inst); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to restart %s: %v\n", inst.ID, err)
		}
	}

	return nil
}

func restartInstance(inst *daemon.InstanceMetadata) error {
	fmt.Printf("Restarting instance %s (PID %d)...\n", inst.ID, inst.PID)

	// Find the process
	proc, err := os.FindProcess(inst.PID)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send SIGTERM
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to signal process: %w", err)
	}

	// Wait for process to stop
	for i := 0; i < 30; i++ {
		if !daemon.IsRunning(inst) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Clean up instance file
	daemon.DeleteInstance(inst.ID)

	fmt.Printf("Instance %s stopped. Please start a new instance manually.\n", inst.ID)
	fmt.Printf("  ccrouter start --port %d", inst.Port)
	if inst.ConfigPath != "" {
		fmt.Printf(" --config %s", inst.ConfigPath)
	}
	fmt.Println()

	return nil
}
