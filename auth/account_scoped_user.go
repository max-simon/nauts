package auth

import "github.com/msimon/nauts/identity"

// AccountScopedUser represents an authenticated user scoped to a single NATS account.
//
// It contains the same fields as [identity.User], plus the account being requested.
// The controller filters Roles to only include roles for Account.
//
// This type is returned by [AuthController.ResolveUser] and should be passed through
// permission compilation and JWT issuance to avoid re-deriving account scope from roles.
//
// Note: This is a value-embedded identity.User to provide direct field access (ID, Roles, Attributes).
type AccountScopedUser struct {
	identity.User
	Account string
}
