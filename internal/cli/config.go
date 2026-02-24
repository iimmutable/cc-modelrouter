package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/spf13/cobra"
)

// NewConfigCommand creates the config command.
func NewConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage configuration",
		Long: `Show or manage configuration files for the router.

Configuration is loaded from the following locations (in order of priority):
  1. Custom file specified via --config flag
  2. .ccrouter.yml in the current project root
  3. ~/.config/ccrouter/config.yml (global configuration)

Available subcommands:
  show    Display the active configuration (API keys are masked)
  path    Show the configuration file search paths
  init    Create a sample configuration file

Use "ccrouter config [subcommand] --help" for more information.`,
	}

	cmd.AddCommand(newConfigShowCommand())
	cmd.AddCommand(newConfigPathCommand())
	cmd.AddCommand(newConfigInitCommand())

	return cmd
}

func newConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show active configuration",
		Long: `Shows the currently active configuration.

Displays the merged configuration from all sources (project, global, custom).
API keys are masked for security.

Flags:
  -c, --config <path>   Path to custom configuration file to display.

Examples:
  # Show active configuration
  ccrouter config show

  # Show specific config file
  ccrouter config show --config /path/to/config.yml`,
		RunE: runConfigShow,
	}

	cmd.Flags().StringP("config", "c", "", "Path to config file")

	return cmd
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	configPath, _ := cmd.Flags().GetString("config")

	// Get working directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Load configuration
	var cfg *config.Config
	var configType string
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
		configType = "custom"
	} else {
		cfg, configType, err = config.LoadWithOverride(projectRoot)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	fmt.Printf("Configuration source: %s\n\n", configType)

	// Mask sensitive data before displaying
	maskedCfg := maskSensitiveConfig(cfg)

	// Pretty print JSON
	data, err := json.MarshalIndent(maskedCfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func newConfigPathCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "Show configuration file paths",
		Long: `Shows the paths where configuration files are searched.

This displays both the project-specific and global configuration file paths
relative to the current working directory.

Examples:
  ccrouter config path`,
		RunE:  runConfigPath,
	}

	return cmd
}

func runConfigPath(cmd *cobra.Command, args []string) error {
	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	fmt.Printf("Project config: %s\n", config.ProjectConfigPath(projectRoot))
	fmt.Printf("Global config:  %s\n", config.GlobalConfigPath())

	return nil
}

func newConfigInitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a sample configuration file",
		Long: `Creates a sample configuration file in the project directory.

The sample configuration includes:
  - Server settings (host, port)
  - Example provider configurations (Anthropic, OpenRouter)
  - Route mappings for different Claude Code modes
  - Retry settings

Flags:
  --global    Create the configuration in the global location
              (~/.config/ccrouter/config.yml) instead of project directory.

Examples:
  # Create project-level config
  ccrouter config init

  # Create global config
  ccrouter config init --global`,
		RunE:  runConfigInit,
	}

	cmd.Flags().Bool("global", false, "Create in global location")

	return cmd
}

func runConfigInit(cmd *cobra.Command, args []string) error {
	useGlobal, _ := cmd.Flags().GetBool("global")

	projectRoot, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create sample config
	sampleCfg := &config.Config{
		Server: config.ServerConfig{
			Port: 8081,
			Host: "localhost",
		},
		Providers: map[string]config.ProviderConfig{
			"anthropic": {
				APIKey:      "${ANTHROPIC_API_KEY}",
				BaseURL:     "https://api.anthropic.com",
				Models:      []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514"},
				Transformer: "anthropic",
			},
			"openrouter": {
				APIKey:      "${OPENROUTER_API_KEY}",
				BaseURL:     "https://openrouter.ai/api",
				Models:      []string{"anthropic/claude-sonnet-4"},
				Transformer: "openrouter",
			},
		},
		Router: config.RouterConfig{
			Routes: map[string]string{
				"default":     "anthropic:claude-sonnet-4-20250514",
				"background":  "anthropic:claude-sonnet-4-20250514",
				"think":       "anthropic:claude-sonnet-4-20250514",
				"thinkMore":   "anthropic:claude-sonnet-4-20250514",
				"ultrathink":  "anthropic:claude-opus-4-20250514",
				"longContext": "anthropic:claude-sonnet-4-20250514",
			},
			MaxRetries: 2,
			RetryDelay: "500ms",
		},
	}

	var path string
	if useGlobal {
		path = config.GlobalConfigPath()
	} else {
		path = config.ProjectConfigPath(projectRoot)
	}

	// Check if file exists
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("config file already exists: %s", path)
	}

	if err := config.Save(sampleCfg, path); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Created sample configuration: %s\n", path)
	fmt.Println("\nEdit the file to add your API keys and customize routes.")

	return nil
}

func maskSensitiveConfig(cfg *config.Config) *config.Config {
	// Create a copy with masked API keys
	masked := &config.Config{
		Server:  cfg.Server,
		Router:  cfg.Router,
		Logging: cfg.Logging,
		Providers: make(map[string]config.ProviderConfig),
	}

	for name, pc := range cfg.Providers {
		maskedPC := pc
		if maskedPC.APIKey != "" {
			maskedPC.APIKey = maskKey(maskedPC.APIKey)
		}
		masked.Providers[name] = maskedPC
	}

	return masked
}

func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}
