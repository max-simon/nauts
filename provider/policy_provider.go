package provider

import (
	"context"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/policy"
)

// PolicyProvider provides read access to policies.
type PolicyProvider interface {
	// GetPolicy retrieves a policy by ID.
	// Returns ErrPolicyNotFound if the policy does not exist.
	GetPolicy(ctx context.Context, id string) (*policy.Policy, error)

	// GetPoliciesForRole returns all policies attached to a role for the given account.
	// Implementations may support both global roles (role.Account="*") and account-local roles.
	// Returns ErrRoleNotFound if no role definition exists for the role.
	GetPoliciesForRole(ctx context.Context, role identity.Role) ([]*policy.Policy, error)

	// GetPolicies returns policies for the given account.
	// Implementations should include global policies (policy.Account == "*")
	// in addition to account-local policies (policy.Account == account).
	GetPolicies(ctx context.Context, account string) ([]*policy.Policy, error)
}
