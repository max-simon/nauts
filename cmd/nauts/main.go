// Package main provides a CLI for the nauts authentication service.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/msimon/nauts/auth"
	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/provider"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Define flags
	var (
		nscDir       string
		operatorName string
		policiesPath string
		groupsPath   string
		usersPath    string
		username     string
		password     string
		userPubKey   string
		ttl          time.Duration
	)

	flag.StringVar(&nscDir, "nsc-dir", "", "Path to nsc directory")
	flag.StringVar(&operatorName, "operator", "", "Operator name")
	flag.StringVar(&policiesPath, "policies", "", "Path to policies JSON file")
	flag.StringVar(&groupsPath, "groups", "", "Path to groups JSON file")
	flag.StringVar(&usersPath, "users", "", "Path to users JSON file")
	flag.StringVar(&username, "username", "", "Username to authenticate")
	flag.StringVar(&password, "password", "", "Password for authentication")
	flag.StringVar(&userPubKey, "user-pubkey", "", "User's public key for JWT subject (optional, generates ephemeral key if not provided)")
	flag.DurationVar(&ttl, "ttl", time.Hour, "JWT time-to-live")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Authenticate a user and generate a NATS JWT.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -nsc-dir ~/.nsc -operator myop -policies policies.json \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "    -groups groups.json -users users.json -username alice -password secret\n")
	}

	flag.Parse()

	// Validate required flags
	if nscDir == "" {
		return fmt.Errorf("--nsc-dir is required")
	}
	if operatorName == "" {
		return fmt.Errorf("--operator is required")
	}
	if policiesPath == "" {
		return fmt.Errorf("--policies is required")
	}
	if groupsPath == "" {
		return fmt.Errorf("--groups is required")
	}
	if usersPath == "" {
		return fmt.Errorf("--users is required")
	}
	if username == "" {
		return fmt.Errorf("--username is required")
	}
	if password == "" {
		return fmt.Errorf("--password is required")
	}

	// Initialize providers
	entityProvider, err := provider.NewNscEntityProvider(provider.NscConfig{
		Dir:          nscDir,
		OperatorName: operatorName,
	})
	if err != nil {
		return fmt.Errorf("initializing entity provider: %w", err)
	}

	nautsProvider, err := provider.NewFileNautsProvider(provider.FileNautsProviderConfig{
		PoliciesPath: policiesPath,
		GroupsPath:   groupsPath,
	})
	if err != nil {
		return fmt.Errorf("initializing nauts provider: %w", err)
	}

	identityProvider, err := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
		UsersPath: usersPath,
	})
	if err != nil {
		return fmt.Errorf("initializing identity provider: %w", err)
	}

	// Create auth controller
	controller := auth.NewAuthController(entityProvider, nautsProvider, identityProvider)

	// Authenticate
	ctx := context.Background()
	result, err := controller.Authenticate(ctx, identity.UsernamePassword{
		Username: username,
		Password: password,
	}, userPubKey, ttl)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Output the JWT
	fmt.Println(result.JWT)

	return nil
}
