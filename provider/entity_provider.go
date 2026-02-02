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
}
