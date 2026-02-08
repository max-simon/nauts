package provider

import (
	"context"
)

// AccountProvider provides access to NATS account entities.
type AccountProvider interface {
	// GetAccount retrieves an account by name.
	// Returns ErrAccountNotFound if the account does not exist.
	GetAccount(ctx context.Context, name string) (*Account, error)

	// ListAccounts returns all accounts.
	ListAccounts(ctx context.Context) ([]*Account, error)

	// IsOperatorMode returns true if this provider operates in NATS operator mode.
	// In operator mode, the auth service runs in the AUTH account but authenticates
	// users across all accounts using account signing keys. The auth callout response
	// must include IssuerAccount to indicate which account the user belongs to.
	IsOperatorMode() bool
}
