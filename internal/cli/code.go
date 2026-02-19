package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/iimmutable/cc-modelrouter/internal/config"
	"github.com/iimmutable/cc-modelrouter/internal/daemon"
	"github.com/iimmutable/cc-modelrouter/internal/provider"
	"github.com/iimmutable/cc-modelrouter/internal/proxy"
	"github.com/iimmutable/cc-modelrouter/internal/router"
	"github.com/iimmutable/cc-modelrouter/internal/transformer"
	"github.com/spf13/cobra"
)

// NewCodeCommand creates the code command.
func NewCodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Start router and launch Claude Code",
		Long:  "Starts the router server and launches Claude Code with the router configured as the API endpoint.",
		RunE:  runCode,
	}

	cmd.Flags().StringP("config", "c", "", "Path to config file")

	return cmd
}

func runCode(cmd *cobra.Command, args []string) error {
	// Get flags
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

	// Start the router
	instanceID := daemon.GenerateInstanceID()
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Start in foreground mode
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

	// Set environment variable for Claude Code
	os.Setenv("ANTHROPIC_BASE_URL", fmt.Sprintf("http://%s", addr))

	// Find claude binary
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		// Stop server and return error
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Stop(ctx)
		daemon.DeleteInstance(instanceID)
		return fmt.Errorf("claude binary not found in PATH: %w", err)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Launch Claude Code
	claudeCmd := exec.Command(claudePath)
	claudeCmd.Stdin = os.Stdin
	claudeCmd.Stdout = os.Stdout
	claudeCmd.Stderr = os.Stderr
	claudeCmd.Env = os.Environ()

	// Start Claude in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- claudeCmd.Run()
	}()

	// Wait for either Claude to finish or a signal
	select {
	case err := <-errChan:
		// Claude finished
		if err != nil {
			fmt.Fprintf(os.Stderr, "Claude exited with error: %v\n", err)
		}
	case sig := <-sigChan:
		// Signal received
		fmt.Printf("\nReceived signal %v, shutting down...\n", sig)
		// Kill Claude process
		if claudeCmd.Process != nil {
			claudeCmd.Process.Signal(sig)
		}
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Stop(ctx)
	daemon.DeleteInstance(instanceID)

	fmt.Println("Router stopped")
	return nil
}
