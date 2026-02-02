// Package identity defines interfaces for user identity providers.
package identity

import (
	"context"

	"github.com/msimon/nauts/auth/model"
)

// IdentityToken is a marker interface for identity tokens.
// Each UserIdentityProvider implementation defines its own token type.
type IdentityToken interface{}

// UserIdentityProvider resolves user identity from an identity token.
// The token type is implementation-specific (e.g., UsernamePassword for static).
type UserIdentityProvider interface {
	// Verify validates the identity token and returns the user.
	// Returns ErrInvalidCredentials if the credentials are invalid.
	// Returns ErrUserNotFound if the user does not exist.
	// Returns ErrInvalidTokenType if the token is the wrong type for this provider.
	Verify(ctx context.Context, token IdentityToken) (*model.User, error)

	// GetUser retrieves a user by ID without verification.
	// Returns ErrUserNotFound if the user does not exist.
	GetUser(ctx context.Context, userID string) (*model.User, error)
}
