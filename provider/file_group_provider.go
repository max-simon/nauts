package provider

import (
	"context"
	"encoding/json"
	"os"
)

// FileGroupProvider implements GroupProvider using a JSON file.
// Data is loaded once during initialization and cached in memory.
type FileGroupProvider struct {
	groups map[string]*Group
}

// FileGroupProviderConfig holds configuration for FileGroupProvider.
type FileGroupProviderConfig struct {
	// GroupsPath is the path to groups JSON file.
	GroupsPath string
}

// NewFileGroupProvider creates a new FileGroupProvider from the given configuration.
func NewFileGroupProvider(cfg FileGroupProviderConfig) (*FileGroupProvider, error) {
	fp := &FileGroupProvider{
		groups: make(map[string]*Group),
	}

	// Load groups
	if cfg.GroupsPath != "" {
		if err := fp.loadGroups(cfg.GroupsPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

// loadGroups loads groups from a JSON file.
// The file should contain an array of groups.
func (fp *FileGroupProvider) loadGroups(path string) error {
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

// GetGroup retrieves a group by ID.
func (fp *FileGroupProvider) GetGroup(_ context.Context, id string) (*Group, error) {
	g, ok := fp.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

// ListGroups returns all groups.
func (fp *FileGroupProvider) ListGroups(_ context.Context) ([]*Group, error) {
	result := make([]*Group, 0, len(fp.groups))
	for _, g := range fp.groups {
		result = append(result, g)
	}
	return result, nil
}
