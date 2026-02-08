package provider

import (
	"context"

	"github.com/msimon/nauts/policy"
)

// PolicyProvider provides read access to policies.
type PolicyProvider interface {
	// GetPolicy retrieves a policy by ID.
	// Returns ErrPolicyNotFound if the policy does not exist.
	GetPolicy(ctx context.Context, id string) (*policy.Policy, error)

	// GetPoliciesForRole returns all policies attached to a role for the given account.
	// Implementations may support both global roles (account="*") and account-local roles.
	// Returns ErrRoleNotFound if no role definition exists for the role.
	GetPoliciesForRole(ctx context.Context, account string, role string) ([]*policy.Policy, error)

	// ListPolicies returns all policies.
	ListPolicies(ctx context.Context) ([]*policy.Policy, error)
}
