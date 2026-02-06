// Package main provides a CLI for the nauts authentication service.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/msimon/nauts/auth"
	natsjwt "github.com/nats-io/jwt/v2"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		printUsage()
		return fmt.Errorf("subcommand required")
	}

	switch os.Args[1] {
	case "auth":
		return runAuth(os.Args[2:])
	case "serve":
		return runServe(os.Args[2:])
	case "-h", "-help", "--help", "help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown subcommand: %s", os.Args[1])
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage: %s <command> [options]

Commands:
  auth    Authenticate a user and output a NATS JWT
  serve   Run the auth callout service

Run '%s <command> -h' for more information on a command.
`, os.Args[0], os.Args[0])
}

// envOrDefault returns the environment variable value if set, otherwise the default.
func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

// runAuth handles the 'auth' subcommand for one-shot authentication.
func runAuth(args []string) error {
	fs := flag.NewFlagSet("auth", flag.ExitOnError)

	var (
		configPath string
		token      string
		userPubKey string
		ttl        time.Duration
	)

	fs.StringVar(&configPath, "c", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")
	fs.StringVar(&configPath, "config", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")
	fs.StringVar(&token, "token", "", "Token to authenticate (JSON: {\"account\":\"APP\",\"token\":\"...\"} with optional \"ap\")")
	fs.StringVar(&userPubKey, "user-pubkey", "", "User's public key for JWT subject (optional, generates ephemeral key if not provided)")
	fs.DurationVar(&ttl, "ttl", time.Hour, "JWT time-to-live (overrides config file)")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s auth [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Authenticate a user and generate a NATS JWT.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  NAUTS_CONFIG       Path to configuration file\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s auth -c config.json -token '{\\\"account\\\":\\\"APP\\\",\\\"token\\\":\\\"alice:secret\\\"}'\n", os.Args[0])
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
	}
  }
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if configPath == "" {
		return fmt.Errorf("-c/--config is required")
	}
	if token == "" {
		return fmt.Errorf("--token is required")
	}

	// Load configuration
	config, err := auth.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Create auth controller from config
	controller, err := auth.NewAuthControllerWithConfig(config)
	if err != nil {
		return fmt.Errorf("creating auth controller: %w", err)
	}

	// Authenticate
	ctx := context.Background()
	result, err := controller.Authenticate(ctx, natsjwt.ConnectOptions{
		Token: token,
	}, userPubKey, ttl)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Output the JWT
	fmt.Println(result.JWT)

	return nil
}

// runServe handles the 'serve' subcommand for the auth callout service.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)

	var configPath string

	fs.StringVar(&configPath, "c", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")
	fs.StringVar(&configPath, "config", envOrDefault("NAUTS_CONFIG", ""), "Path to configuration file")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s serve [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Run the NATS auth callout service.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables:\n")
		fmt.Fprintf(os.Stderr, "  NAUTS_CONFIG       Path to configuration file\n")
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s serve -c config.json\n", os.Args[0])
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
      "natsNkey": "auth-service.nk",
      "xkeySeedFile": "xkey.seed",
      "ttl": "1h"
    }
  }
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate required flags
	if configPath == "" {
		return fmt.Errorf("-c/--config is required")
	}

	// Load configuration
	config, err := auth.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Validate server config for serve mode
	if err := validateServerConfig(&config.Server); err != nil {
		return err
	}

	// Create auth controller from config
	controller, err := auth.NewAuthControllerWithConfig(config)
	if err != nil {
		return fmt.Errorf("creating auth controller: %w", err)
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

	// Setup signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		fmt.Fprintf(os.Stderr, "\nReceived signal %v, shutting down...\n", sig)
		cancel()
		service.Stop()
	}()

	// Start the service (blocks until shutdown)
	if err := service.Start(ctx); err != nil {
		return fmt.Errorf("running callout service: %w", err)
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
