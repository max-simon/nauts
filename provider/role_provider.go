package provider

import (
	"context"
)

// RoleProvider provides read access to roles.
type RoleProvider interface {
	// GetRoles retrieves both global (Account="*") and local roles for a given name and account.
	// Returns the matching global role and/or local role.
	// Returns ErrRoleNotFound only if neither a global nor local role exists for the name.
	GetRoles(ctx context.Context, name string, account string) (globalRole *Role, localRole *Role, err error)

	// ListRoles returns all roles.
	ListRoles(ctx context.Context) ([]*Role, error)
}
