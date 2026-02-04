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

	// ListPolicies returns all policies.
	ListPolicies(ctx context.Context) ([]*policy.Policy, error)
}
