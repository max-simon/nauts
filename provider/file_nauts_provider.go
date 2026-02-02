package provider

import (
	"context"
	"encoding/json"
	"os"

	"github.com/msimon/nauts/policy"
)

// FileNautsProvider implements NautsProvider using JSON files.
// Data is loaded once during initialization and cached in memory.
type FileNautsProvider struct {
	policies map[string]*policy.Policy
	groups   map[string]*Group
}

// FileNautsProviderConfig holds configuration for FileNautsProvider.
type FileNautsProviderConfig struct {
	// PoliciesPath is the path to policies JSON file.
	PoliciesPath string

	// GroupsPath is the path to groups JSON file.
	GroupsPath string
}

// NewFileNautsProvider creates a new FileNautsProvider from the given configuration.
func NewFileNautsProvider(cfg FileNautsProviderConfig) (*FileNautsProvider, error) {
	fp := &FileNautsProvider{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*Group),
	}

	// Load policies
	if cfg.PoliciesPath != "" {
		if err := fp.loadPolicies(cfg.PoliciesPath); err != nil {
			return nil, err
		}
	}

	// Load groups
	if cfg.GroupsPath != "" {
		if err := fp.loadGroups(cfg.GroupsPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

// loadPolicies loads policies from a JSON file.
func (fp *FileNautsProvider) loadPolicies(path string) error {
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
		fp.policies[p.ID] = p
	}

	return nil
}

// loadGroups loads groups from a JSON file.
// The file should contain an array of groups.
func (fp *FileNautsProvider) loadGroups(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var groups []*Group
	if err := json.Unmarshal(data, &groups); err != nil {
		return err
	}

	for _, g := range groups {
		if err := g.Validate(); err != nil {
			return err
		}
		fp.groups[g.ID] = g
	}

	return nil
}

// GetPolicy retrieves a policy by ID.
func (fp *FileNautsProvider) GetPolicy(_ context.Context, id string) (*policy.Policy, error) {
	p, ok := fp.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

// GetGroup retrieves a group by ID.
func (fp *FileNautsProvider) GetGroup(_ context.Context, id string) (*Group, error) {
	g, ok := fp.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

// ListPolicies returns all policies.
func (fp *FileNautsProvider) ListPolicies(_ context.Context) ([]*policy.Policy, error) {
	result := make([]*policy.Policy, 0, len(fp.policies))
	for _, p := range fp.policies {
		result = append(result, p)
	}
	return result, nil
}

// ListGroups returns all groups.
func (fp *FileNautsProvider) ListGroups(_ context.Context) ([]*Group, error) {
	result := make([]*Group, 0, len(fp.groups))
	for _, g := range fp.groups {
		result = append(result, g)
	}
	return result, nil
}
