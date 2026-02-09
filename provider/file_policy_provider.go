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
	policies map[string]*policy.Policy
	bindings map[string]*binding
}

// FilePolicyProviderConfig holds configuration for FilePolicyProvider.
type FilePolicyProviderConfig struct {
	// PoliciesPath is the path to policies JSON file.
	PoliciesPath string `json:"policiesPath"`
	// BindingsPath is the path to bindings JSON file.
	BindingsPath string `json:"bindingsPath"`
}

// binding represents a collection of policies attached to a role in an account.
//
// This struct exists to support FilePolicyProvider's role->policy mapping.
// It is not used by the rest of the system.
type binding struct {
	Role     string   `json:"role"`
	Account  string   `json:"account"`
	Policies []string `json:"policies"`
}

type roleValidationError struct {
	Field   string
	Message string
}

func (e *roleValidationError) Error() string {
	return e.Field + ": " + e.Message
}

func (b *binding) Validate() error {
	if b.Role == "" {
		return &roleValidationError{Field: "role", Message: "role is required"}
	}
	if b.Account == "" {
		return &roleValidationError{Field: "account", Message: "binding account is required"}
	}
	return nil
}

func bindingKey(account string, role string) string {
	return account + "." + role
}

// NewFilePolicyProvider creates a new FilePolicyProvider from the given configuration.
func NewFilePolicyProvider(cfg FilePolicyProviderConfig) (*FilePolicyProvider, error) {
	fp := &FilePolicyProvider{
		policies: make(map[string]*policy.Policy),
		bindings: make(map[string]*binding),
	}

	// Load policies
	if cfg.PoliciesPath != "" {
		if err := fp.loadPolicies(cfg.PoliciesPath); err != nil {
			return nil, err
		}
	}

	// Load bindings
	if cfg.BindingsPath != "" {
		if err := fp.loadBindings(cfg.BindingsPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

func (fp *FilePolicyProvider) loadBindings(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var bindings []*binding
	if err := json.Unmarshal(data, &bindings); err != nil {
		return err
	}

	for _, b := range bindings {
		if err := b.Validate(); err != nil {
			return err
		}
		fp.bindings[bindingKey(b.Account, b.Role)] = b
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
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, ErrRoleNotFound
	}

	b := fp.bindings[bindingKey(account, role)]
	if b == nil {
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

	for _, id := range b.Policies {
		addPolicyID(id)
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

// ListPolicies returns policies for the given account.
// Global policies (Account="*") are always included.
func (fp *FilePolicyProvider) ListPolicies(_ context.Context, account string) ([]*policy.Policy, error) {
	account = strings.TrimSpace(account)

	result := make([]*policy.Policy, 0, len(fp.policies))
	for _, p := range fp.policies {
		if p == nil {
			continue
		}
		if p.Account == "*" || (account != "" && p.Account == account) {
			result = append(result, p)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}
