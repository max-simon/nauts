package provider

import (
	"context"

	"github.com/msimon/nauts/policy"
)

// NautsProvider provides read access to groups and policies.
type NautsProvider interface {
	// GetPolicy retrieves a policy by ID.
	// Returns ErrPolicyNotFound if the policy does not exist.
	GetPolicy(ctx context.Context, id string) (*policy.Policy, error)

	// GetGroup retrieves a group by ID.
	// Returns ErrGroupNotFound if the group does not exist.
	GetGroup(ctx context.Context, id string) (*Group, error)

	// ListPolicies returns all policies.
	ListPolicies(ctx context.Context) ([]*policy.Policy, error)

	// ListGroups returns all groups.
	ListGroups(ctx context.Context) ([]*Group, error)
}