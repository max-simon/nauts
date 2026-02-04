package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/msimon/nauts/policy"
)

func TestNewFilePolicyProvider(t *testing.T) {
	cfg := FilePolicyProviderConfig{
		PoliciesPath: "../test/policies.json",
	}

	fp, err := NewFilePolicyProvider(cfg)
	if err != nil {
		t.Fatalf("NewFilePolicyProvider() error = %v", err)
	}

	ctx := context.Background()

	// Test GetPolicy
	pol, err := fp.GetPolicy(ctx, "read-access")
	if err != nil {
		t.Fatalf("GetPolicy() error = %v", err)
	}
	if pol.ID != "read-access" {
		t.Errorf("GetPolicy() ID = %v, want read-access", pol.ID)
	}

	// Test ListPolicies
	policies, err := fp.ListPolicies(ctx)
	if err != nil {
		t.Fatalf("ListPolicies() error = %v", err)
	}
	if len(policies) < 1 {
		t.Errorf("ListPolicies() returned %d policies, want at least 1", len(policies))
	}
}

func TestFilePolicyProvider_GetPolicy_NotFound(t *testing.T) {
	fp := &FilePolicyProvider{
		policies: make(map[string]*policy.Policy),
	}

	ctx := context.Background()
	_, err := fp.GetPolicy(ctx, "nonexistent")
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
	policies, err := fp.ListPolicies(ctx)
	if err != nil {
		t.Fatalf("ListPolicies() error = %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("ListPolicies() = %d, want 0 for empty config", len(policies))
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
	if err := os.WriteFile(invalidPath, []byte(`[{"id": "", "statements": []}]`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFilePolicyProvider(FilePolicyProviderConfig{
		PoliciesPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFilePolicyProvider() expected error for invalid policy")
	}
}
