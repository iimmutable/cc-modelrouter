package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/usage"
	"github.com/spf13/cobra"
)

// NewUsageCommand creates the usage command.
func NewUsageCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "usage [instance-id] [period]",
		Short: "Show token usage statistics",
		Long:  "Displays token usage statistics per model, per route, and per instance.",
		Args:  cobra.MaximumNArgs(2),
		RunE:  runUsage,
	}

	return cmd
}

func runUsage(cmd *cobra.Command, args []string) error {
	// Parse arguments
	var instanceID, period string
	if len(args) > 0 {
		// Check if first arg is an instance ID or a period
		if isPeriod(args[0]) {
			period = args[0]
		} else {
			instanceID = args[0]
		}
	}
	if len(args) > 1 {
		period = args[1]
	}

	// Default period
	if period == "" {
		period = "all-time"
	}

	// Parse period
	now := time.Now()
	start, end, err := usage.ParsePeriod(period, now)
	if err != nil {
		return fmt.Errorf("invalid period: %w", err)
	}

	// Open database
	dbPath, err := usage.DBPath()
	if err != nil {
		return fmt.Errorf("failed to get db path: %w", err)
	}

	db, err := usage.InitDB(dbPath)
	if err != nil {
		// Database might not exist yet
		fmt.Fprintln(os.Stderr, "No usage data available yet")
		return nil
	}
	defer db.Close()

	// Query records
	records, err := usage.GetRecordsByPeriod(db, instanceID, start, end)
	if err != nil {
		return fmt.Errorf("failed to query usage: %w", err)
	}

	if len(records) == 0 {
		fmt.Println("No usage records found for the specified period")
		return nil
	}

	// Format and display
	usage.FormatUsage(os.Stdout, instanceID, period, records)
	return nil
}

// isPeriod checks if a string looks like a period specification.
func isPeriod(s string) bool {
	periods := []string{"all-time", "today", "this-week", "last-week",
		"this-month", "last-month", "this-quarter", "last-quarter",
		"this-year", "last-year"}
	for _, p := range periods {
		if s == p {
			return true
		}
	}
	// Check for custom date range format (YYYYMMDD-YYYYMMDD is 17 chars)
	if len(s) == 17 && s[8] == '-' {
		return true
	}
	return false
}
