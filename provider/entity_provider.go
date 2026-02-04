package provider

import (
	"context"
)

// EntityProvider provides access to NATS operator and account entities.
type EntityProvider interface {
	// GetOperator returns the operator entity.
	GetOperator(ctx context.Context) (*Operator, error)

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
