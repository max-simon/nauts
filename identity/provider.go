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

	// ErrInvalidAccount is returned when the requested account is not valid for the user.
	ErrInvalidAccount = errors.New("invalid account for user")

	// ErrAccountRequired is returned when the user has multiple accounts but no account was specified.
	ErrAccountRequired = errors.New("account is required when user has multiple accounts")
)

// AuthRequest represents the parsed authentication request from the token.
// The token is expected to be a JSON object with the following structure:
//
//	{ "account": "ACME", "token": "username:password" }
//
// The account field is optional if the user has only one account.
type AuthRequest struct {
	// Account is the requested account (optional if user has single account).
	Account string `json:"account,omitempty"`
	// Token is the authentication token (e.g., "username:password").
	Token string `json:"token"`
}

// UserIdentityProvider resolves user identity from an authentication request.
type UserIdentityProvider interface {
	// Verify validates the authentication request and returns the user.
	// Returns ErrInvalidCredentials if the credentials are invalid.
	// Returns ErrUserNotFound if the user does not exist.
	// Returns ErrInvalidTokenType if the token is the wrong type for this provider.
	// Returns ErrInvalidAccount if the requested account is not valid for the user.
	// Returns ErrAccountRequired if user has multiple accounts but no account was specified.
	Verify(ctx context.Context, req AuthRequest) (*User, error)
}
