package grouppolicyprovider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/provider"
	"github.com/msimon/nauts/policy"
)

func TestNewFile_WithDirectory(t *testing.T) {
	// Get the testdata directory path
	testdataDir := getTestdataDir(t)

	cfg := FileConfig{
		PoliciesPath: filepath.Join(testdataDir, "policies.json"),
		GroupsPath:   filepath.Join(testdataDir, "groups", "groups.json"),
	}

	fp, err := NewFile(cfg)
	if err != nil {
		t.Fatalf("NewFile() error = %v", err)
	}

	ctx := context.Background()

	// Check policies loaded from file
	policies, err := fp.ListPolicies(ctx)
	if err != nil {
		t.Fatalf("ListPolicies() error = %v", err)
	}
	if len(policies) != 6 { // all policies in combined file
		t.Errorf("Expected 6 policies, got %d", len(policies))
	}

	// Check specific policy
	p, err := fp.GetPolicy(ctx, "allow-orders")
	if err != nil {
		t.Fatalf("GetPolicy(allow-orders) error = %v", err)
	}
	if p.Name != "Orders Access" {
		t.Errorf("Policy name = %q, want %q", p.Name, "Orders Access")
	}

	// Check groups loaded
	groups, err := fp.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if len(groups) != 4 { // default, workers, viewers, admins
		t.Errorf("Expected 4 groups, got %d", len(groups))
	}

	// Check specific group
	g, err := fp.GetGroup(ctx, "workers")
	if err != nil {
		t.Fatalf("GetGroup(workers) error = %v", err)
	}
	if g.Name != "Workers" {
		t.Errorf("Group name = %q, want %q", g.Name, "Workers")
	}
	if len(g.Policies) != 2 {
		t.Errorf("Group policies count = %d, want 2", len(g.Policies))
	}
}

func TestGetPolicy_NotFound(t *testing.T) {
	fp := &FileGroupPolicyProvider{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*model.Group),
	}

	_, err := fp.GetPolicy(context.Background(), "nonexistent")
	if !errors.Is(err, provider.ErrPolicyNotFound) {
		t.Errorf("GetPolicy() error = %v, want %v", err, provider.ErrPolicyNotFound)
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	fp := &FileGroupPolicyProvider{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*model.Group),
	}

	_, err := fp.GetGroup(context.Background(), "nonexistent")
	if !errors.Is(err, provider.ErrGroupNotFound) {
		t.Errorf("GetGroup() error = %v, want %v", err, provider.ErrGroupNotFound)
	}
}

func TestNewFile_EmptyConfig(t *testing.T) {
	fp, err := NewFile(FileConfig{})
	if err != nil {
		t.Fatalf("NewFile() error = %v", err)
	}

	ctx := context.Background()

	policies, _ := fp.ListPolicies(ctx)
	if len(policies) != 0 {
		t.Errorf("Expected 0 policies, got %d", len(policies))
	}

	groups, _ := fp.ListGroups(ctx)
	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}
}

func TestNewFile_InvalidPoliciesPath(t *testing.T) {
	_, err := NewFile(FileConfig{
		PoliciesPath: "/nonexistent/path",
	})
	if err == nil {
		t.Error("NewFile() expected error for nonexistent path")
	}
}

func TestNewFile_InvalidJSON(t *testing.T) {
	// Create a temp file with invalid JSON
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := NewFile(FileConfig{
		PoliciesPath: invalidFile,
	})
	if err == nil {
		t.Error("NewFile() expected error for invalid JSON")
	}
}

func TestNewFile_InvalidPolicy(t *testing.T) {
	// Create a temp file with invalid policy (missing required fields)
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid_policy.json")
	content := `[{"name": "Missing ID"}]`
	if err := os.WriteFile(invalidFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := NewFile(FileConfig{
		PoliciesPath: invalidFile,
	})
	if err == nil {
		t.Error("NewFile() expected error for invalid policy")
	}
}

// Helper to get testdata directory
func getTestdataDir(t *testing.T) string {
	t.Helper()

	// Try relative path from test file location
	candidates := []string{
		"../../../testdata",
		"testdata",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	t.Fatal("Could not find testdata directory")
	return ""
}
