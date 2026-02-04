package provider

import (
	"context"
	"encoding/json"
	"os"
)

// FileRoleProvider implements RoleProvider using a JSON file.
// Data is loaded once during initialization and cached in memory.
// Roles are indexed by their composite key (name:account) for local roles,
// and by name alone for global roles.
type FileRoleProvider struct {
	// roles indexed by name:account key for local roles
	localRoles map[string]*Role
	// globalRoles indexed by name for global roles (account="*")
	globalRoles map[string]*Role
	// allRoles stores all roles for ListRoles
	allRoles []*Role
}

// FileRoleProviderConfig holds configuration for FileRoleProvider.
type FileRoleProviderConfig struct {
	// RolesPath is the path to roles JSON file.
	RolesPath string
}

// NewFileRoleProvider creates a new FileRoleProvider from the given configuration.
func NewFileRoleProvider(cfg FileRoleProviderConfig) (*FileRoleProvider, error) {
	fp := &FileRoleProvider{
		localRoles:  make(map[string]*Role),
		globalRoles: make(map[string]*Role),
		allRoles:    nil,
	}

	// Load roles
	if cfg.RolesPath != "" {
		if err := fp.loadRoles(cfg.RolesPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

// loadRoles loads roles from a JSON file.
// The file should contain an array of roles.
func (fp *FileRoleProvider) loadRoles(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var roles []*Role
	if err := json.Unmarshal(data, &roles); err != nil {
		return err
	}

	for _, r := range roles {
		if err := r.Validate(); err != nil {
			return err
		}
		if r.IsGlobal() {
			fp.globalRoles[r.Name] = r
		} else {
			fp.localRoles[r.Key()] = r
		}
	}

	fp.allRoles = roles
	return nil
}

// GetRoles retrieves both global (Account="*") and local roles for a given name and account.
// Returns the matching global role and/or local role.
// Returns ErrRoleNotFound only if neither a global nor local role exists for the name.
func (fp *FileRoleProvider) GetRoles(_ context.Context, name string, account string) (*Role, *Role, error) {
	globalRole := fp.globalRoles[name]
	localRole := fp.localRoles[name+":"+account]

	if globalRole == nil && localRole == nil {
		return nil, nil, ErrRoleNotFound
	}

	return globalRole, localRole, nil
}

// ListRoles returns all roles.
func (fp *FileRoleProvider) ListRoles(_ context.Context) ([]*Role, error) {
	result := make([]*Role, len(fp.allRoles))
	copy(result, fp.allRoles)
	return result, nil
}
