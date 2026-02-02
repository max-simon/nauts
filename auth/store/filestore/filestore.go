package filestore

import (
	"context"
	"encoding/json"
	"os"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/store"
	"github.com/msimon/nauts/policy"
)

// FileStore implements store.Store using JSON files.
// Data is loaded once during initialization and cached in memory.
type FileStore struct {
	policies map[string]*policy.Policy
	groups   map[string]*model.Group
}

// Config holds configuration for FileStore.
type Config struct {
	// PoliciesPath is the path to policies JSON file.
	PoliciesPath string

	// GroupsPath is the path to groups JSON file.
	GroupsPath string
}

// New creates a new FileStore from the given configuration.
func New(cfg Config) (*FileStore, error) {
	fs := &FileStore{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*model.Group),
	}

	// Load policies
	if cfg.PoliciesPath != "" {
		if err := fs.loadPolicies(cfg.PoliciesPath); err != nil {
			return nil, err
		}
	}

	// Load groups
	if cfg.GroupsPath != "" {
		if err := fs.loadGroups(cfg.GroupsPath); err != nil {
			return nil, err
		}
	}

	return fs, nil
}

// loadPolicies loads policies from a JSON file.
func (fs *FileStore) loadPolicies(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var policies []*policy.Policy
	if err := json.Unmarshal(data, &policies); err != nil {
		return err
	}

	for _, p := range policies {
		if err := p.Validate(); err != nil {
			return err
		}
		fs.policies[p.ID] = p
	}

	return nil
}

// loadGroups loads groups from a JSON file.
// The file should contain an array of groups.
func (fs *FileStore) loadGroups(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var groups []*model.Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}

	for _, g := range groups {
		if err := g.Validate(); err != nil {
			return err
		}
		fs.groups[g.ID] = g
	}

	return nil
}

// GetPolicy retrieves a policy by ID.
func (fs *FileStore) GetPolicy(_ context.Context, id string) (*policy.Policy, error) {
	p, ok := fs.policies[id]
	if !ok {
		return nil, store.ErrPolicyNotFound
	}
	return p, nil
}

// GetGroup retrieves a group by ID.
func (fs *FileStore) GetGroup(_ context.Context, id string) (*model.Group, error) {
	g, ok := fs.groups[id]
	if !ok {
		return nil, store.ErrGroupNotFound
	}
	return g, nil
}

// ListPolicies returns all policies.
func (fs *FileStore) ListPolicies(_ context.Context) ([]*policy.Policy, error) {
	result := make([]*policy.Policy, 0, len(fs.policies))
	for _, p := range fs.policies {
		result = append(result, p)
	}
	return result, nil
}

// ListGroups returns all groups.
func (fs *FileStore) ListGroups(_ context.Context) ([]*model.Group, error) {
	result := make([]*model.Group, 0, len(fs.groups))
	for _, g := range fs.groups {
		result = append(result, g)
	}
	return result, nil
}
