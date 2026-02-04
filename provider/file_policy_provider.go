package provider

import (
	"context"
	"encoding/json"
	"os"

	"github.com/msimon/nauts/policy"
)

// FilePolicyProvider implements PolicyProvider using a JSON file.
// Data is loaded once during initialization and cached in memory.
type FilePolicyProvider struct {
	policies map[string]*policy.Policy
}

// FilePolicyProviderConfig holds configuration for FilePolicyProvider.
type FilePolicyProviderConfig struct {
	// PoliciesPath is the path to policies JSON file.
	PoliciesPath string
}

// NewFilePolicyProvider creates a new FilePolicyProvider from the given configuration.
func NewFilePolicyProvider(cfg FilePolicyProviderConfig) (*FilePolicyProvider, error) {
	fp := &FilePolicyProvider{
		policies: make(map[string]*policy.Policy),
	}

	// Load policies
	if cfg.PoliciesPath != "" {
		if err := fp.loadPolicies(cfg.PoliciesPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
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

// ListPolicies returns all policies.
func (fp *FilePolicyProvider) ListPolicies(_ context.Context) ([]*policy.Policy, error) {
	result := make([]*policy.Policy, 0, len(fp.policies))
	for _, p := range fp.policies {
		result = append(result, p)
	}
	return result, nil
}
