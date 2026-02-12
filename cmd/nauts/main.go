// Package main provides a CLI for the nauts authentication service.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/msimon/nauts/auth"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "-help", "--help", "help":
			printUsage()
			return nil
		}
	}

	return runServe(os.Args[1:])
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s [options]

Run the NATS auth callout service (optionally with debug service).

Use '%s -h' for more information.
`, os.Args[0], os.Args[0])
}

// envOrDefault returns the environment variable value if set, otherwise the default.
func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// runServe handles the 'serve' subcommand for the auth callout service.
func runServe(args []string) error {
	fs := flag.NewFlagSet("nauts", flag.ExitOnError)

	var configPath string
	var enableDebugSvc bool

	fs.StringVar(&configPath, "c", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")
	fs.StringVar(&configPath, "config", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")
	fs.BoolVar(&enableDebugSvc, "enable-debug-svc", false, "Start the NATS auth debug service")

	fs.Usage = func() {
		printServiceUsage(fs, "Run the NATS auth callout service.", true)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	config, controller, err := loadConfigAndController(configPath)
	if err != nil {
		return err
	}

	// Create callout config
	calloutConfig, err := config.Server.ToCalloutConfig()
	if err != nil {
		return fmt.Errorf("creating callout config: %w", err)
	}

	// Create callout service
	service, err := auth.NewCalloutService(controller, calloutConfig)
	if err != nil {
		return fmt.Errorf("creating callout service: %w", err)
	}

	var debugService *auth.DebugService
	if enableDebugSvc {
		debugService, err = auth.NewDebugService(controller, config.Server)
		if err != nil {
			return fmt.Errorf("creating debug service: %w", err)
		}
	}

	ctx, cancel := setupSignalHandler(func() {
		service.Stop()
		if debugService != nil {
			debugService.Stop()
		}
	})
	defer cancel()

	debugErrCh := make(chan error, 1)
	if debugService != nil {
		go func() {
			if err := debugService.Start(ctx); err != nil {
				debugErrCh <- err
				cancel()
				return
			}
			debugErrCh <- nil
		}()
	}

	// Start the callout service (blocks until shutdown)
	if err := service.Start(ctx); err != nil {
		return fmt.Errorf("running callout service: %w", err)
	}

	if debugService != nil {
		if err := <-debugErrCh; err != nil {
			return fmt.Errorf("running debug service: %w", err)
		}
	}

	return nil
}

// validateServerConfig validates the server configuration for serve mode.
func validateServerConfig(c *auth.ServerConfig) error {
	hasCredentials := c.NatsCredentials != ""
	hasNkey := c.NatsNkey != ""

	if !hasCredentials && !hasNkey {
		return fmt.Errorf("server.natsCredentials or server.natsNkey is required")
	}
	if hasCredentials && hasNkey {
		return fmt.Errorf("server.natsCredentials and server.natsNkey are mutually exclusive")
	}

	return nil
}

func loadConfigAndController(configPath string) (*auth.Config, *auth.AuthController, error) {
	if configPath == "" {
		return nil, nil, fmt.Errorf("-c/--config is required")
	}

	config, err := auth.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("loading configuration: %w", err)
	}

	if err := validateServerConfig(&config.Server); err != nil {
		return nil, nil, err
	}

	controller, err := auth.NewAuthControllerWithConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("creating auth controller: %w", err)
	}

	return config, controller, nil
}

func setupSignalHandler(onStop func()) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, shutting down...\n", sig)
		cancel()
		if onStop != nil {
			onStop()
		}
	}()

	return ctx, cancel
}

func printServiceUsage(fs *flag.FlagSet, description string, includeTTL bool) {
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "%s\n\n", description)
	fmt.Fprintf(os.Stderr, "Options:\n")
	fs.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
	fmt.Fprintf(os.Stderr, "  NAUTS_CONFIG       Path to configuration file\n")
	fmt.Fprintf(os.Stderr, "\nExample:\n")
	fmt.Fprintf(os.Stderr, "  %s -c config.json\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nConfiguration file format (JSON):\n")
	fmt.Fprintf(os.Stderr, `  {
	"account": {
	  "type": "static",
	  "static": { "publicKey": "AXXXXX...", "privateKeyPath": "account.nk", "accounts": ["APP"] }
	},
	"policy": {
	  "type": "file",
	  "file": { "policiesPath": "policies.json", "bindingsPath": "bindings.json" }
	},
	"auth": {
	  "file": [
	    { "id": "local", "accounts": ["APP"], "userPath": "users.json" }
	  ]
	},
    "server": {
      "natsUrl": "nats://localhost:4222",
      "natsNkey": "auth-service.nk"`)
	if includeTTL {
		fmt.Fprintf(os.Stderr, `,
      "xkeySeedFile": "xkey.seed",
      "ttl": "1h"`)
	}
	fmt.Fprintf(os.Stderr, `
    }
  }
`)
}
