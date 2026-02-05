package provider

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/msimon/nauts/policy"
)

// FilePolicyProvider implements PolicyProvider using a JSON file.
// Data is loaded once during initialization and cached in memory.
type FilePolicyProvider struct {
	policies    map[string]*policy.Policy
	localRoles  map[string]*role
	globalRoles map[string]*role
}

// FilePolicyProviderConfig holds configuration for FilePolicyProvider.
type FilePolicyProviderConfig struct {
	// PoliciesPath is the path to policies JSON file.
	PoliciesPath string
	// RolesPath is the path to roles JSON file.
	RolesPath string
}

// role represents a collection of policies that can be assigned to users.
//
// This struct exists to support FilePolicyProvider's role->policy mapping.
// It is not used by the rest of the system.
type role struct {
	Name     string   `json:"name"`
	Account  string   `json:"account"`
	Policies []string `json:"policies"`
}

func (r *role) IsGlobal() bool {
	return r != nil && r.Account == GlobalAccountID
}

func (r *role) Key() string {
	return r.Name + ":" + r.Account
}

type roleValidationError struct {
	Field   string
	Message string
}

func (e *roleValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (r *role) Validate() error {
	if r.Name == "" {
		return &roleValidationError{Field: "name", Message: "role name is required"}
	}
	if r.Account == "" {
		return &roleValidationError{Field: "account", Message: "role account is required"}
	}
	return nil
}

// NewFilePolicyProvider creates a new FilePolicyProvider from the given configuration.
func NewFilePolicyProvider(cfg FilePolicyProviderConfig) (*FilePolicyProvider, error) {
	fp := &FilePolicyProvider{
		policies:    make(map[string]*policy.Policy),
		localRoles:  make(map[string]*role),
		globalRoles: make(map[string]*role),
	}

	// Load policies
	if cfg.PoliciesPath != "" {
		if err := fp.loadPolicies(cfg.PoliciesPath); err != nil {
			return nil, err
		}
	}

	// Load roles
	if cfg.RolesPath != "" {
		if err := fp.loadRoles(cfg.RolesPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

func (fp *FilePolicyProvider) loadRoles(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var roles []*role
	if err := json.Unmarshal(data, &roles); err != nil {
		return err
	}

	for _, r := range roles {
		if err := r.Validate(); err != nil {
			return err
		}
		if r.IsGlobal() {
			fp.globalRoles[r.Name] = r
			continue
		}
		fp.localRoles[r.Key()] = r
	}

	return nil
}

// loadPolicies loads policies from a JSON file.
func (fp *FilePolicyProvider) loadPolicies(path string) error {
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

// GetPolicy retrieves a policy by ID.
func (fp *FilePolicyProvider) GetPolicy(_ context.Context, id string) (*policy.Policy, error) {
	p, ok := fp.policies[id]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return p, nil
}

func (fp *FilePolicyProvider) GetPoliciesForRole(ctx context.Context, account string, role string) ([]*policy.Policy, error) {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil, ErrRoleNotFound
	}

	globalRole := fp.globalRoles[role]
	localRole := fp.localRoles[role+":"+account]
	if globalRole == nil && localRole == nil {
		return nil, ErrRoleNotFound
	}

	policyIDs := make([]string, 0, 8)
	seen := make(map[string]struct{})
	addPolicyID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		policyIDs = append(policyIDs, id)
	}

	if globalRole != nil {
		for _, id := range globalRole.Policies {
			addPolicyID(id)
		}
	}
	if localRole != nil {
		for _, id := range localRole.Policies {
			addPolicyID(id)
		}
	}

	sort.Strings(policyIDs)

	result := make([]*policy.Policy, 0, len(policyIDs))
	for _, id := range policyIDs {
		p, err := fp.GetPolicy(ctx, id)
		if err != nil {
			if errors.Is(err, ErrPolicyNotFound) {
				// Keep behavior consistent with previous behavior:
				// missing policies do not fail authentication/authorization.
				continue
			}
			return nil, err
		}
		result = append(result, p)
	}

	return result, nil
}

// ListPolicies returns all policies.
func (fp *FilePolicyProvider) ListPolicies(_ context.Context) ([]*policy.Policy, error) {
	result := make([]*policy.Policy, 0, len(fp.policies))
	for _, p := range fp.policies {
		result = append(result, p)
	}
	return result, nil
}
