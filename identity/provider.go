package identity

import (
	"context"
	"errors"
)

// Sentinel errors for identity operations.
var (
	// ErrInvalidCredentials is returned when credentials fail verification.
	ErrInvalidCredentials = errors.New("invalid credentials")

	// ErrUserNotFound is returned when the user does not exist.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidTokenType is returned when the token type is wrong for the provider.
	ErrInvalidTokenType = errors.New("invalid token type for provider")
)

// IdentityToken is a marker interface for identity tokens.
// Each UserIdentityProvider implementation defines its own token type.
type IdentityToken interface{}

// UserIdentityProvider resolves user identity from an identity token.
// The token type is implementation-specific (e.g., UsernamePassword for file provider).
type UserIdentityProvider interface {
	// Verify validates the identity token and returns the user.
	// Returns ErrInvalidCredentials if the credentials are invalid.
	// Returns ErrUserNotFound if the user does not exist.
	// Returns ErrInvalidTokenType if the token is the wrong type for this provider.
	Verify(ctx context.Context, token IdentityToken) (*User, error)
}
