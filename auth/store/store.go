// Package store defines storage interfaces for nauts policies, groups, and users.
package store

import (
	"context"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/policy"
)

// Store provides read access to policies, groups, and users.
type Store interface {
	// GetPolicy retrieves a policy by ID.
	GetPolicy(ctx context.Context, id string) (*policy.Policy, error)

	// GetGroup retrieves a group by ID.
	GetGroup(ctx context.Context, id string) (*model.Group, error)

	// ListPolicies returns all policies.
	ListPolicies(ctx context.Context) ([]*policy.Policy, error)

	// ListGroups returns all groups.
	ListGroups(ctx context.Context) ([]*model.Group, error)
}
