package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/iimmutable/cc-modelrouter/internal/usage"
	"github.com/spf13/cobra"
)

// NewStartCommand creates the start command.
func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the router server",
		Long:  "Starts the router server in standalone mode.",
		RunE:  runStart,
	}

	cmd.Flags().StringP("config", "c", "", "Path to config file")
	cmd.Flags().IntP("port", "p", 0, "Port to listen on (overrides config)")
	cmd.Flags().StringP("host", "H", "", "Host to bind to (overrides config)")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	// Get flags
	configPath, _ := cmd.Flags().GetString("config")
	port, _ := cmd.Flags().GetInt("port")
	host, _ := cmd.Flags().GetString("host")

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

	// Generate instance ID
	instanceID := daemon.GenerateInstanceID()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	fmt.Printf("Starting router on %s\n", addr)

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
	registry.Register(transformer.NewAnthropicTransformer())
	registry.Register(transformer.NewOpenRouterTransformer())
	registry.Register(transformer.NewGeminiTransformer())
	registry.Register(transformer.NewQwenTransformer())
	registry.Register(transformer.NewGLMTransformer())
	server.SetTransformerRegistry(NewRegistryAdapter(registry))

	// Setup provider clients
	clients := make(map[string]proxy.HTTPClient)
	for name, providerCfg := range cfg.Providers {
		client, err := provider.NewClient(&provider.ClientConfig{
			BaseURL:    providerCfg.BaseURL,
			APIKey:     providerCfg.APIKey,
			MaxRetries: cfg.Router.MaxRetries,
			RetryDelay: cfg.Router.GetRetryDelay(),
		})
		if err != nil {
			return fmt.Errorf("failed to create client for %s: %w", name, err)
		}
		clients[name] = client
	}
	server.SetProviderClients(clients)
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
