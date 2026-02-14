package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/policy"
)

func TestNewFilePolicyProvider(t *testing.T) {
	tmpDir := t.TempDir()

	policiesContent := `[
  {
    "id": "read-access",
		"account": "APP",
    "name": "Read-Only Access",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:public.>"]
      }
    ]
  },
  {
    "id": "write-access",
		"account": "APP",
    "name": "Write Access",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:public.>"]
      }
    ]
  }
]`
	policiesPath := filepath.Join(tmpDir, "policies.json")
	if err := os.WriteFile(policiesPath, []byte(policiesContent), 0644); err != nil {
		t.Fatalf("Failed to write policies file: %v", err)
	}

	bindingsContent := `[
  {
    "role": "default",
    "account": "APP",
    "policies": []
  },
  {
    "role": "readonly",
    "account": "APP",
    "policies": ["read-access"]
  },
  {
    "role": "full",
    "account": "APP",
    "policies": ["read-access", "write-access"]
  }
]`
	bindingsPath := filepath.Join(tmpDir, "bindings.json")
	if err := os.WriteFile(bindingsPath, []byte(bindingsContent), 0644); err != nil {
		t.Fatalf("Failed to write bindings file: %v", err)
	}

	cfg := FilePolicyProviderConfig{
		PoliciesPath: policiesPath,
		BindingsPath: bindingsPath,
	}

	fp, err := NewFilePolicyProvider(cfg)
	if err != nil {
		t.Fatalf("NewFilePolicyProvider() error = %v", err)
	}

	ctx := context.Background()

	// Test GetPolicy
	pol, err := fp.GetPolicy(ctx, "APP", "read-access")
	if err != nil {
		t.Fatalf("GetPolicy() error = %v", err)
	}
	if pol.ID != "read-access" {
		t.Errorf("GetPolicy() ID = %v, want read-access", pol.ID)
	}

	// Test GetPolicies
	policies, err := fp.GetPolicies(ctx, "APP")
	if err != nil {
		t.Fatalf("GetPolicies() error = %v", err)
	}
	if len(policies) < 1 {
		t.Errorf("GetPolicies() returned %d policies, want at least 1", len(policies))
	}

	// Test GetPoliciesForRole
	rolePolicies, err := fp.GetPoliciesForRole(ctx, identity.Role{Account: "APP", Name: "readonly"})
	if err != nil {
		t.Fatalf("GetPoliciesForRole() error = %v", err)
	}
	if len(rolePolicies) != 1 {
		t.Fatalf("GetPoliciesForRole() returned %d policies, want 1", len(rolePolicies))
	}
	if rolePolicies[0].ID != "read-access" {
		t.Errorf("GetPoliciesForRole() first policy ID = %q, want %q", rolePolicies[0].ID, "read-access")
	}
}

func TestFilePolicyProvider_GetPoliciesForRole_GlobalPolicy(t *testing.T) {
	tmpDir := t.TempDir()

	policiesContent := `[
  {
    "id": "app-read",
    "account": "APP",
    "name": "App Read",
    "statements": [
      { "effect": "allow", "actions": ["nats.sub"], "resources": ["nats:public.>"] }
    ]
  },
  {
    "id": "base-permissions",
    "account": "*",
    "name": "Base Permissions",
    "statements": [
      { "effect": "allow", "actions": ["nats.sub"], "resources": ["nats:status.>"] }
    ]
  }
]`
	policiesPath := filepath.Join(tmpDir, "policies.json")
	if err := os.WriteFile(policiesPath, []byte(policiesContent), 0644); err != nil {
		t.Fatalf("failed to write policies file: %v", err)
	}

	bindingsContent := `[
  {
    "role": "mixed",
    "account": "APP",
    "policies": ["app-read", "_global:base-permissions"]
  }
]`
	bindingsPath := filepath.Join(tmpDir, "bindings.json")
	if err := os.WriteFile(bindingsPath, []byte(bindingsContent), 0644); err != nil {
		t.Fatalf("failed to write bindings file: %v", err)
	}

	fp, err := NewFilePolicyProvider(FilePolicyProviderConfig{
		PoliciesPath: policiesPath,
		BindingsPath: bindingsPath,
	})
	if err != nil {
		t.Fatalf("NewFilePolicyProvider() error = %v", err)
	}

	ctx := context.Background()
	policies, err := fp.GetPoliciesForRole(ctx, identity.Role{Account: "APP", Name: "mixed"})
	if err != nil {
		t.Fatalf("GetPoliciesForRole() error = %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("GetPoliciesForRole() returned %d policies, want 2", len(policies))
	}

	ids := map[string]bool{}
	for _, p := range policies {
		ids[p.ID] = true
	}
	if !ids["app-read"] {
		t.Error("expected app-read policy to be present")
	}
	if !ids["base-permissions"] {
		t.Error("expected base-permissions policy to be present")
	}
}

func TestFilePolicyProvider_GetPolicy_NotFound(t *testing.T) {
	fp := &FilePolicyProvider{
		policies: make(map[string]*policy.Policy),
	}

	ctx := context.Background()
	_, err := fp.GetPolicy(ctx, "APP", "nonexistent")
	if err != ErrPolicyNotFound {
		t.Errorf("GetPolicy() error = %v, want ErrPolicyNotFound", err)
	}
}

func TestNewFilePolicyProvider_EmptyConfig(t *testing.T) {
	fp, err := NewFilePolicyProvider(FilePolicyProviderConfig{})
	if err != nil {
		t.Fatalf("NewFilePolicyProvider() error = %v", err)
	}

	ctx := context.Background()
	policies, err := fp.GetPolicies(ctx, "APP")
	if err != nil {
		t.Fatalf("GetPolicies() error = %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("GetPolicies() = %d, want 0 for empty config", len(policies))
	}
}

func TestFilePolicyProvider_GetPoliciesForRole_NotFound(t *testing.T) {
	fp := &FilePolicyProvider{
		policies: make(map[string]*policy.Policy),
		bindings: make(map[string]*binding),
	}

	ctx := context.Background()
	_, err := fp.GetPoliciesForRole(ctx, identity.Role{Account: "APP", Name: "nonexistent"})
	if err != ErrRoleNotFound {
		t.Errorf("GetPoliciesForRole() error = %v, want ErrRoleNotFound", err)
	}
}

func TestNewFilePolicyProvider_InvalidPath(t *testing.T) {
	_, err := NewFilePolicyProvider(FilePolicyProviderConfig{
		PoliciesPath: "/nonexistent/path/policies.json",
	})
	if err == nil {
		t.Error("NewFilePolicyProvider() expected error for nonexistent path")
	}
}

func TestNewFilePolicyProvider_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFilePolicyProvider(FilePolicyProviderConfig{
		PoliciesPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFilePolicyProvider() expected error for invalid JSON")
	}
}

func TestNewFilePolicyProvider_InvalidPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	// Policy with empty ID
	if err := os.WriteFile(invalidPath, []byte(`[{"id": "", "account": "APP", "statements": []}]`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFilePolicyProvider(FilePolicyProviderConfig{
		PoliciesPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFilePolicyProvider() expected error for invalid policy")
	}
}

func TestBinding_Validate(t *testing.T) {
	tests := []struct {
		name    string
		binding binding
		wantErr bool
	}{
		{
			name: "valid binding",
			binding: binding{
				Role:     "test-role",
				Account:  "APP",
				Policies: []string{"policy-1"},
			},
			wantErr: false,
		},
		{
			name: "valid binding without policies",
			binding: binding{
				Role:    "test-role",
				Account: "APP",
			},
			wantErr: false,
		},
		{
			name: "missing role",
			binding: binding{
				Account:  "APP",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
		{
			name: "missing account",
			binding: binding{
				Role:     "test-role",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.binding.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("binding.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBindingKey(t *testing.T) {
	if got := bindingKey("APP", "admin"); got != "APP.admin" {
		t.Errorf("bindingKey() = %v, want %v", got, "APP.admin")
	}
}

func TestBinding_JSON(t *testing.T) {
	b := binding{
		Role:     "test-role",
		Account:  "APP",
		Policies: []string{"policy-1", "policy-2"},
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed binding
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Role != b.Role {
		t.Errorf("Role mismatch: got %v, want %v", parsed.Role, b.Role)
	}
	if parsed.Account != b.Account {
		t.Errorf("Account mismatch: got %v, want %v", parsed.Account, b.Account)
	}
	if len(parsed.Policies) != 2 {
		t.Errorf("Policies length mismatch: got %d, want 2", len(parsed.Policies))
	}
}
