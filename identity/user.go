// Package identity provides user identity types and providers for nauts.
package identity

import (
	"errors"
	"strings"
)

// AccountRole represents a role scoped to a specific account.
type AccountRole struct {
	Account string `json:"account"` // NATS account ID
	Role    string `json:"role"`    // Role name within the account
}

// User represents a user identity that can be authenticated.
type User struct {
	ID         string            `json:"id,omitempty"`         // user identifier (from external)
	Roles      []AccountRole     `json:"roles"`                // list of account-scoped roles
	Attributes map[string]string `json:"attributes,omitempty"` // additional user attributes
}

// ParseRoleID parses a role ID in the format "<account>.<role>" into an AccountRole.
// Returns an error if the format is invalid.
// Note: Wildcard validation is performed by the AuthController.
func ParseRoleID(roleID string) (AccountRole, error) {
	parts := strings.SplitN(roleID, ".", 2)
	if len(parts) != 2 {
		return AccountRole{}, errors.New("invalid role ID format: expected '<account>.<role>'")
	}

	account := parts[0]
	role := parts[1]

	if account == "" || role == "" {
		return AccountRole{}, errors.New("invalid role ID: account and role must not be empty")
	}

	return AccountRole{
		Account: account,
		Role:    role,
	}, nil
}
