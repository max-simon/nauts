package provider

import (
	"context"
)

// GroupProvider provides read access to groups.
type GroupProvider interface {
	// GetGroup retrieves a group by ID.
	// Returns ErrGroupNotFound if the group does not exist.
	GetGroup(ctx context.Context, id string) (*Group, error)

	// ListGroups returns all groups.
	ListGroups(ctx context.Context) ([]*Group, error)
}
