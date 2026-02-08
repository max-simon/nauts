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
)

// AuthRequest represents the parsed authentication request from the token.
// The token is expected to be a JSON object with the following structure:
//
//	{ "account": "ACME", "token": "username:password", "ap": "provider-id" }
//
// The account field is required.
type AuthRequest struct {
	// Account is the requested account (required).
	Account string `json:"account"`
	// Token is the authentication token (e.g., "username:password").
	Token string `json:"token"`
	// AP is an optional authentication provider id.
	// If set, the authentication request is routed to that provider.
	AP string `json:"ap,omitempty"`
}

// AuthenticationProvider resolves user identity from an authentication request.
type AuthenticationProvider interface {
	// Verify validates the authentication request and returns the user.
	// Returns ErrInvalidCredentials if the credentials are invalid.
	// Returns ErrUserNotFound if the user does not exist.
	// Returns ErrInvalidTokenType if the token is the wrong type for this provider.
	// Returns ErrInvalidAccount if the requested account is not valid for the user.
	Verify(ctx context.Context, req AuthRequest) (*User, error)

	// ManageableAccounts returns the list of account patterns this provider can manage.
	// Patterns support wildcards in the form of "*" (all) or "prefix*".
	ManageableAccounts() []string
}
