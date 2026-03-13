package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/iimmutable/cc-modelrouter/internal/logging"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	transformers "github.com/iimmutable/cc-modelrouter/internal/transformer/transformers"
	"github.com/iimmutable/cc-modelrouter/internal/usage"
	"github.com/spf13/cobra"
)

// NewStartCommand creates the start command.
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the router server",
		Long: `Starts the router server in standalone mode.

The router acts as a proxy between Claude Code and LLM providers. It routes requests
based on configured rules and transforms requests/responses for provider compatibility.

Flags:
  -c, --config <path>   Path to custom configuration file (YAML or JSON).
                        If not specified, searches for config in:
                        - .ccrouter.yml in project root
                        - ~/.config/ccrouter/config.yml (global)

  -p, --port <number>   Port number for the router to listen on.
                        Overrides the port specified in config file.
                        Default: 8081 (or value from config)

  -H, --host <address>  Host address to bind to.
                        Overrides the host specified in config file.
                        Default: localhost (or value from config)

Examples:
  # Start with default configuration
  ccrouter start

  # Start with custom config file
  ccrouter start --config /path/to/config.yml

  # Start on specific port
  ccrouter start --port 9090

  # Start on specific host and port
  ccrouter start --host 0.0.0.0 --port 8081

After starting, set ANTHROPIC_BASE_URL to point to the router:
  export ANTHROPIC_BASE_URL=http://localhost:8081

Flags:
  --log-destination <dest>  Log destination: "file", "stdout", "stderr", or a file path.
                           Overrides config file setting.`,
		RunE: runStart,
	}

	cmd.Flags().StringP("config", "c", "", "Path to config file")
	cmd.Flags().IntP("port", "p", 0, "Port to listen on (overrides config)")
	cmd.Flags().StringP("host", "H", "", "Host to bind to (overrides config)")
	cmd.Flags().String("log-destination", "", "Log destination (file|stdout|stderr|path)")
	cmd.Flags().String("log-level", "", "Log level: debug, info, warn, error (default: from config)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	// Get flags
	configPath, _ := cmd.Flags().GetString("config")
	port, _ := cmd.Flags().GetInt("port")
	host, _ := cmd.Flags().GetString("host")
	logDestination, _ := cmd.Flags().GetString("log-destination")

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

	// Apply flag overrides
	if port > 0 {
		cfg.Server.Port = port
	}
	if host != "" {
		cfg.Server.Host = host
	}
	if logDestination != "" {
		cfg.Logging.Destination = logDestination
		cfg.Logging.Enabled = true // CLI flag implicitly enables logging
	}

	// Apply log level override
	logLevel, _ := cmd.Flags().GetString("log-level")
	if logLevel != "" {
		cfg.Logging.Level = logLevel
		cfg.Logging.Enabled = true // CLI flag implicitly enables logging
	}

	// Generate instance ID and address early (needed for logging)
	instanceID := daemon.GenerateInstanceID()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Set per-instance log file path if logging to file
	if cfg.Logging.ShouldLogToFile() && cfg.Logging.FilePath == "" {
		logPath, err := cfg.Logging.GetLogPathWithInstance(instanceID)
		if err == nil {
			cfg.Logging.FilePath = logPath
		}
	}

	// Initialize logging based on configuration
	logCleanup, err := logging.Init(&cfg.Logging)
	if err != nil {
		return fmt.Errorf("failed to initialize logging: %w", err)
	}
	defer logCleanup()

	// Verify logging is working by writing a test message
	logging.Infof("Logging initialized - router starting on %s", addr)

	// Log startup
	fmt.Printf("Starting router on %s\n", addr)
	if cfg.Logging.IsEnabled() {
		if cfg.Logging.Destination == "file" || cfg.Logging.Destination == "" {
			if logPath, logErr := cfg.Logging.GetLogPath(); logErr == nil {
				fmt.Printf("Logging to: %s\n", logPath)
			}
		} else if cfg.Logging.Destination == "stdout" {
			fmt.Printf("Logging to: stdout\n")
		} else if cfg.Logging.Destination == "stderr" {
			fmt.Printf("Logging to: stderr\n")
		} else {
			// Custom path
			fmt.Printf("Logging to: %s\n", cfg.Logging.Destination)
		}
	} else {
		fmt.Printf("Logging: disabled\n")
	}

	// Create server
	serverCfg := &proxy.ServerConfig{
		Host: cfg.Server.Host,
		Port: cfg.Server.Port,
	}
	server, err := proxy.NewServer(serverCfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Setup router engine
	routerEngine := router.NewEngine(cfg)
	server.SetRouter(NewRouterAdapter(routerEngine))

	// Setup transformer registry
	registry := transformer.NewRegistry()
	// New transformers (Anthropic-centric interface)
	registry.Register(transformers.NewAnthropicTransformer())
	registry.Register(transformers.NewGLMAnthropicTransformer())
	registry.Register(transformers.NewOpenRouterTransformer())
	registry.Register(transformers.NewOpenAITransformer())
	registry.Register(transformers.NewGeminiTransformer())
	// Note: Qwen and MiniMax now use the Anthropic transformer since they are Anthropic-compatible
	// GLM providers (aliyun, bigmodel) use the GLM-specific transformer which ensures signature field handling
	// OpenRouter providers use the OpenRouter-specific transformer which preserves signature fields
	server.SetTransformerRegistry(NewRegistryAdapter(registry))

	// Setup provider clients
	clients := make(map[string]proxy.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		// Validate API key is not empty or unset
		if providerCfg.APIKey == "" {
			return fmt.Errorf("provider %s: API key is empty (check environment variable)", name)
		}
		if strings.HasPrefix(providerCfg.APIKey, "${") {
			return fmt.Errorf("provider %s: API key environment variable not set: %s", name, providerCfg.APIKey)
		}

		client, err := provider.NewClient(&provider.ClientConfig{
			BaseURL:           providerCfg.BaseURL,
			APIKey:            providerCfg.APIKey,
			MaxRetries:        cfg.Router.MaxRetries,
			RetryDelay:        cfg.Router.GetRetryDelay(),
			DisableKeepAlives: providerCfg.DisableKeepAlives,
		})
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", name, err)
		}
		clients[name] = client
	}
	server.SetProviderClients(clients)

	// Create streaming clients (no timeout for long-running SSE streams)
	streamingClients := make(map[string]proxy.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		streamingClient, err := provider.NewStreamingClient(&provider.ClientConfig{
			BaseURL:           providerCfg.BaseURL,
			APIKey:            providerCfg.APIKey,
			MaxRetries:        cfg.Router.MaxRetries,
			RetryDelay:        cfg.Router.GetRetryDelay(),
			DisableKeepAlives: providerCfg.DisableKeepAlives,
		})
		if err != nil {
			return fmt.Errorf("failed to create streaming client for %s: %w", name, err)
		}
		streamingClients[name] = streamingClient
	}
	server.SetStreamingClients(streamingClients)

	server.SetConfig(cfg)

	// Initialize usage tracker
	dbPath, err := usage.DBPath()
	if err != nil {
		return fmt.Errorf("failed to get db path: %w", err)
	}

	usageDB, err := usage.InitDB(dbPath)
	if err != nil {
		return fmt.Errorf("failed to init usage db: %w", err)
	}

	tracker := usage.NewTracker(usageDB, usage.DefaultBufferSize, usage.DefaultFlushTimeout)
	server.SetUsageTracker(tracker)
	server.SetInstanceID(instanceID)

	// Add logging interceptor
	loggingInterceptor := proxy.NewLoggingInterceptor()
	server.AddRequestInterceptor(loggingInterceptor)
	server.AddResponseInterceptor(loggingInterceptor)

	// Start server
	if err := server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Save instance metadata
	meta := &daemon.InstanceMetadata{
		ID:          instanceID,
		Port:        cfg.Server.Port,
		PID:         os.Getpid(),
		ConfigType:  configType,
		ConfigPath:  configPath,
		ProjectRoot: projectRoot,
		StartTime:   time.Now(),
	}
	if err := daemon.SaveInstance(meta); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to save instance metadata: %v\n", err)
	}

	fmt.Printf("Router started with instance ID: %s\n", instanceID)
	fmt.Printf("Set ANTHROPIC_BASE_URL=http://%s to use the router\n", addr)

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Printf("\nShutting down router...\n")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Stop(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error stopping server: %v\n", err)
	}

	// Cleanup instance file
	daemon.DeleteInstance(instanceID)

	fmt.Println("Router stopped")
	return nil
}
