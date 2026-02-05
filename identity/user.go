// Package identity provides user identity types and providers for nauts.
package identity

import (
	"fmt"
	"strings"
)

// AccountRole associates a role name with a concrete account.
// Format matches the convention used in JWT roles: "<account>.<role>"
// Note: Account is always a concrete account name (e.g., "APP", "PROD"), never "*".
// Global roles (Account="*") only exist in Role definitions from RoleProvider.
type AccountRole struct {
	Account string `json:"account"` // NATS account name (concrete, never "*")
	Role    string `json:"role"`    // Role name within the account
}

// String returns the formatted role ID: "<account>.<role>"
func (ar AccountRole) String() string {
	return ar.Account + "." + ar.Role
}

// ParseAccountRole parses a role ID string in format "<account>.<role>".
// Returns an error if the format is invalid.
func ParseAccountRole(roleID string) (AccountRole, error) {
	parts := strings.SplitN(roleID, ".", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return AccountRole{}, fmt.Errorf("invalid role format %q: expected <account>.<role>", roleID)
	}
	return AccountRole{
		Account: parts[0],
		Role:    parts[1],
	}, nil
}

// ParseAccountRoles parses multiple role ID strings, skipping invalid formats.
// Returns successfully parsed roles and a slice of errors for invalid entries.
func ParseAccountRoles(roleIDs []string) ([]AccountRole, []error) {
	var roles []AccountRole
	var errors []error

	for _, id := range roleIDs {
		role, err := ParseAccountRole(id)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		roles = append(roles, role)
	}

	return roles, errors
}

// User represents a user identity that can be authenticated.
// Can contain all roles (from AuthenticationProvider.Verify) or filtered roles (from AuthController.ResolveUser).
type User struct {
	ID string `json:"id,omitempty"` // user identifier (from external)

	// Roles is the list of roles with concrete account associations.
	// Account is always a concrete value (e.g., "APP", "PROD"), never "*".
	// Before filtering: all roles across all accounts
	// After filtering: only roles matching target account
	// Example: [{Account: "APP", Role: "admin"}, {Account: "APP", Role: "viewer"}]
	Roles []AccountRole `json:"roles"`

	// Attributes contains additional user metadata from the identity provider.
	// Common attributes: email, name, preferred_username, department, etc.
	Attributes map[string]string `json:"attributes,omitempty"`
}
