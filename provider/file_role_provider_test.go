package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileRoleProvider(t *testing.T) {
	cfg := FileRoleProviderConfig{
		RolesPath: "../test/roles.json",
	}

	fp, err := NewFileRoleProvider(cfg)
	if err != nil {
		t.Fatalf("NewFileRoleProvider() error = %v", err)
	}

	ctx := context.Background()

	// Test GetRoles - global role only
	globalRole, localRole, err := fp.GetRoles(ctx, "readonly", "APP")
	if err != nil {
		t.Fatalf("GetRoles() error = %v", err)
	}
	if globalRole == nil {
		t.Fatal("GetRoles() globalRole is nil")
	}
	if globalRole.Name != "readonly" {
		t.Errorf("GetRoles() globalRole.Name = %v, want readonly", globalRole.Name)
	}
	if !globalRole.IsGlobal() {
		t.Error("GetRoles() globalRole.IsGlobal() = false, want true")
	}
	if localRole != nil {
		t.Error("GetRoles() localRole is not nil, want nil")
	}

	// Test ListRoles
	roles, err := fp.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) < 1 {
		t.Errorf("ListRoles() returned %d roles, want at least 1", len(roles))
	}
}

func TestFileRoleProvider_GetRoles_NotFound(t *testing.T) {
	fp := &FileRoleProvider{
		localRoles:  make(map[string]*Role),
		globalRoles: make(map[string]*Role),
	}

	ctx := context.Background()
	_, _, err := fp.GetRoles(ctx, "nonexistent", "APP")
	if err != ErrRoleNotFound {
		t.Errorf("GetRoles() error = %v, want ErrRoleNotFound", err)
	}
}

func TestFileRoleProvider_GetRoles_GlobalOnly(t *testing.T) {
	fp := &FileRoleProvider{
		localRoles: make(map[string]*Role),
		globalRoles: map[string]*Role{
			"admin": {Name: "admin", Account: GlobalAccountID, Policies: []string{"admin-policy"}},
		},
	}

	ctx := context.Background()
	globalRole, localRole, err := fp.GetRoles(ctx, "admin", "APP")
	if err != nil {
		t.Fatalf("GetRoles() error = %v", err)
	}
	if globalRole == nil {
		t.Fatal("GetRoles() globalRole is nil")
	}
	if globalRole.Name != "admin" {
		t.Errorf("GetRoles() globalRole.Name = %v, want admin", globalRole.Name)
	}
	if localRole != nil {
		t.Error("GetRoles() localRole should be nil")
	}
}

func TestFileRoleProvider_GetRoles_LocalOnly(t *testing.T) {
	fp := &FileRoleProvider{
		globalRoles: make(map[string]*Role),
		localRoles: map[string]*Role{
			"admin:APP": {Name: "admin", Account: "APP", Policies: []string{"app-admin-policy"}},
		},
	}

	ctx := context.Background()
	globalRole, localRole, err := fp.GetRoles(ctx, "admin", "APP")
	if err != nil {
		t.Fatalf("GetRoles() error = %v", err)
	}
	if globalRole != nil {
		t.Error("GetRoles() globalRole should be nil")
	}
	if localRole == nil {
		t.Fatal("GetRoles() localRole is nil")
	}
	if localRole.Name != "admin" {
		t.Errorf("GetRoles() localRole.Name = %v, want admin", localRole.Name)
	}
	if localRole.Account != "APP" {
		t.Errorf("GetRoles() localRole.Account = %v, want APP", localRole.Account)
	}
}

func TestFileRoleProvider_GetRoles_BothGlobalAndLocal(t *testing.T) {
	fp := &FileRoleProvider{
		globalRoles: map[string]*Role{
			"admin": {Name: "admin", Account: GlobalAccountID, Policies: []string{"global-policy"}},
		},
		localRoles: map[string]*Role{
			"admin:APP": {Name: "admin", Account: "APP", Policies: []string{"local-policy"}},
		},
	}

	ctx := context.Background()
	globalRole, localRole, err := fp.GetRoles(ctx, "admin", "APP")
	if err != nil {
		t.Fatalf("GetRoles() error = %v", err)
	}
	if globalRole == nil {
		t.Fatal("GetRoles() globalRole is nil")
	}
	if localRole == nil {
		t.Fatal("GetRoles() localRole is nil")
	}
	if globalRole.Account != GlobalAccountID {
		t.Errorf("GetRoles() globalRole.Account = %v, want %v", globalRole.Account, GlobalAccountID)
	}
	if localRole.Account != "APP" {
		t.Errorf("GetRoles() localRole.Account = %v, want APP", localRole.Account)
	}
}

func TestNewFileRoleProvider_EmptyConfig(t *testing.T) {
	fp, err := NewFileRoleProvider(FileRoleProviderConfig{})
	if err != nil {
		t.Fatalf("NewFileRoleProvider() error = %v", err)
	}

	ctx := context.Background()
	roles, err := fp.ListRoles(ctx)
	if err != nil {
		t.Fatalf("ListRoles() error = %v", err)
	}
	if len(roles) != 0 {
		t.Errorf("ListRoles() = %d, want 0 for empty config", len(roles))
	}
}

func TestNewFileRoleProvider_InvalidPath(t *testing.T) {
	_, err := NewFileRoleProvider(FileRoleProviderConfig{
		RolesPath: "/nonexistent/path/roles.json",
	})
	if err == nil {
		t.Error("NewFileRoleProvider() expected error for nonexistent path")
	}
}

func TestNewFileRoleProvider_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFileRoleProvider(FileRoleProviderConfig{
		RolesPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFileRoleProvider() expected error for invalid JSON")
	}
}

func TestNewFileRoleProvider_InvalidRole(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	// Role with empty name
	if err := os.WriteFile(invalidPath, []byte(`[{"name": "", "account": "*"}]`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFileRoleProvider(FileRoleProviderConfig{
		RolesPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFileRoleProvider() expected error for invalid role")
	}
}

func TestNewFileRoleProvider_MixedGlobalAndLocalRoles(t *testing.T) {
	tmpDir := t.TempDir()
	rolesPath := filepath.Join(tmpDir, "roles.json")
	rolesContent := `[
  {"name": "admin", "account": "*", "policies": ["global-admin"]},
  {"name": "admin", "account": "APP", "policies": ["app-admin"]},
  {"name": "readonly", "account": "*", "policies": ["read-only"]}
]`
	if err := os.WriteFile(rolesPath, []byte(rolesContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	fp, err := NewFileRoleProvider(FileRoleProviderConfig{
		RolesPath: rolesPath,
	})
	if err != nil {
		t.Fatalf("NewFileRoleProvider() error = %v", err)
	}

	ctx := context.Background()

	// Test admin role - should have both global and local
	globalRole, localRole, err := fp.GetRoles(ctx, "admin", "APP")
	if err != nil {
		t.Fatalf("GetRoles(admin, APP) error = %v", err)
	}
	if globalRole == nil {
		t.Error("Expected global admin role")
	}
	if localRole == nil {
		t.Error("Expected local admin role for APP")
	}

	// Test admin role for different account - should have global only
	globalRole, localRole, err = fp.GetRoles(ctx, "admin", "OTHER")
	if err != nil {
		t.Fatalf("GetRoles(admin, OTHER) error = %v", err)
	}
	if globalRole == nil {
		t.Error("Expected global admin role")
	}
	if localRole != nil {
		t.Error("Expected no local admin role for OTHER")
	}

	// Test readonly role - should have global only
	globalRole, localRole, err = fp.GetRoles(ctx, "readonly", "APP")
	if err != nil {
		t.Fatalf("GetRoles(readonly, APP) error = %v", err)
	}
	if globalRole == nil {
		t.Error("Expected global readonly role")
	}
	if localRole != nil {
		t.Error("Expected no local readonly role")
	}
}
