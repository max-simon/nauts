package identity

import (
	"context"
	"errors"
	"strings"
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

	// ErrAccountRequired is returned when no account was specified.
	ErrAccountRequired = errors.New("account is required")
)

// AuthRequest represents the parsed authentication request from the token.
// The token is expected to be a JSON object with the following structure:
//
//	{ "account": "ACME", "token": "username:password", "ap": "provider-id" }
//
// The "ap" field is optional and used to explicitly select an authentication provider
// when multiple providers can manage the same account.
type AuthRequest struct {
	// Account is the requested account.
	Account string `json:"account"`
	// Token is the authentication token (e.g., "username:password").
	Token string `json:"token"`
}

// AuthenticationProvider verifies user credentials and returns identity information.
// Providers are account-scoped: each provider declares which accounts it can manage.
type AuthenticationProvider interface {
	// ID returns the unique identifier for this provider.
	// Used for explicit provider selection when multiple providers match an account.
	ID() string

	// CanManageAccount returns true if this provider is authorized to authenticate
	// users for the given account. Supports wildcard patterns ("*", "prefix-*").
	// Special accounts "SYS" and "AUTH" require exact match, never matched by wildcards.
	CanManageAccount(account string) bool

	// Verify validates the authentication request and returns the authenticated user.
	// The returned User contains all roles the user has across all accounts.
	// Role filtering for the target account is performed by the AuthController.
	//
	// Returns:
	//   - ErrInvalidCredentials: Authentication failed (wrong password, invalid token, etc.)
	//   - ErrUserNotFound: User does not exist
	//   - ErrInvalidTokenType: Token format is wrong for this provider
	Verify(ctx context.Context, req AuthRequest) (*User, error)
}

// MatchAccountPattern checks if an account name matches a pattern.
// Pattern syntax:
//   - "*" matches any account
//   - "prefix-*" matches accounts starting with "prefix-"
//   - "exact" matches only "exact"
//
// Special cases:
//   - "SYS" and "AUTH" accounts require exact match, never matched by wildcards
func MatchAccountPattern(pattern, account string) bool {
	// SYS and AUTH are protected
	if account == "SYS" || account == "AUTH" {
		return pattern == account
	}

	// Wildcard match all
	if pattern == "*" {
		return true
	}

	// Prefix wildcard
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(account, prefix)
	}

	// Exact match
	return pattern == account
}
