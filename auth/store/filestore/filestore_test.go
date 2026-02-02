package filestore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/store"
	"github.com/msimon/nauts/policy"
)

func TestNew_WithDirectory(t *testing.T) {
	// Get the testdata directory path
	testdataDir := getTestdataDir(t)

	cfg := Config{
		PoliciesPath: filepath.Join(testdataDir, "policies.json"),
		GroupsPath:   filepath.Join(testdataDir, "groups", "groups.json"),
	}

	fs, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	// Check policies loaded from file
	policies, err := fs.ListPolicies(ctx)
	if err != nil {
		t.Fatalf("ListPolicies() error = %v", err)
	}
	if len(policies) != 6 { // all policies in combined file
		t.Errorf("Expected 6 policies, got %d", len(policies))
	}

	// Check specific policy
	p, err := fs.GetPolicy(ctx, "allow-orders")
	if err != nil {
		t.Fatalf("GetPolicy(allow-orders) error = %v", err)
	}
	if p.Name != "Orders Access" {
		t.Errorf("Policy name = %q, want %q", p.Name, "Orders Access")
	}

	// Check groups loaded
	groups, err := fs.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if len(groups) != 4 { // default, workers, viewers, admins
		t.Errorf("Expected 4 groups, got %d", len(groups))
	}

	// Check specific group
	g, err := fs.GetGroup(ctx, "workers")
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
	fs := &FileStore{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*model.Group),
	}

	_, err := fs.GetPolicy(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrPolicyNotFound) {
		t.Errorf("GetPolicy() error = %v, want %v", err, store.ErrPolicyNotFound)
	}
}

func TestGetGroup_NotFound(t *testing.T) {
	fs := &FileStore{
		policies: make(map[string]*policy.Policy),
		groups:   make(map[string]*model.Group),
	}

	_, err := fs.GetGroup(context.Background(), "nonexistent")
	if !errors.Is(err, store.ErrGroupNotFound) {
		t.Errorf("GetGroup() error = %v, want %v", err, store.ErrGroupNotFound)
	}
}

func TestNew_EmptyConfig(t *testing.T) {
	fs, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx := context.Background()

	policies, _ := fs.ListPolicies(ctx)
	if len(policies) != 0 {
		t.Errorf("Expected 0 policies, got %d", len(policies))
	}

	groups, _ := fs.ListGroups(ctx)
	if len(groups) != 0 {
		t.Errorf("Expected 0 groups, got %d", len(groups))
	}
}

func TestNew_InvalidPoliciesPath(t *testing.T) {
	_, err := New(Config{
		PoliciesPath: "/nonexistent/path",
	})
	if err == nil {
		t.Error("New() expected error for nonexistent path")
	}
}

func TestNew_InvalidJSON(t *testing.T) {
	// Create a temp file with invalid JSON
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := New(Config{
		PoliciesPath: invalidFile,
	})
	if err == nil {
		t.Error("New() expected error for invalid JSON")
	}
}

func TestNew_InvalidPolicy(t *testing.T) {
	// Create a temp file with invalid policy (missing required fields)
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid_policy.json")
	content := `[{"name": "Missing ID"}]`
	if err := os.WriteFile(invalidFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := New(Config{
		PoliciesPath: invalidFile,
	})
	if err == nil {
		t.Error("New() expected error for invalid policy")
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
