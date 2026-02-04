package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewFileGroupProvider(t *testing.T) {
	cfg := FileGroupProviderConfig{
		GroupsPath: "../test/groups.json",
	}

	fp, err := NewFileGroupProvider(cfg)
	if err != nil {
		t.Fatalf("NewFileGroupProvider() error = %v", err)
	}

	ctx := context.Background()

	// Test GetGroup
	group, err := fp.GetGroup(ctx, "readonly")
	if err != nil {
		t.Fatalf("GetGroup() error = %v", err)
	}
	if group.ID != "readonly" {
		t.Errorf("GetGroup() ID = %v, want readonly", group.ID)
	}
	if group.Name != "Read-Only Users" {
		t.Errorf("GetGroup() Name = %v, want Read-Only Users", group.Name)
	}

	// Test ListGroups
	groups, err := fp.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if len(groups) < 1 {
		t.Errorf("ListGroups() returned %d groups, want at least 1", len(groups))
	}
}

func TestFileGroupProvider_GetGroup_NotFound(t *testing.T) {
	fp := &FileGroupProvider{
		groups: make(map[string]*Group),
	}

	ctx := context.Background()
	_, err := fp.GetGroup(ctx, "nonexistent")
	if err != ErrGroupNotFound {
		t.Errorf("GetGroup() error = %v, want ErrGroupNotFound", err)
	}
}

func TestNewFileGroupProvider_EmptyConfig(t *testing.T) {
	fp, err := NewFileGroupProvider(FileGroupProviderConfig{})
	if err != nil {
		t.Fatalf("NewFileGroupProvider() error = %v", err)
	}

	ctx := context.Background()
	groups, err := fp.ListGroups(ctx)
	if err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("ListGroups() = %d, want 0 for empty config", len(groups))
	}
}

func TestNewFileGroupProvider_InvalidPath(t *testing.T) {
	_, err := NewFileGroupProvider(FileGroupProviderConfig{
		GroupsPath: "/nonexistent/path/groups.json",
	})
	if err == nil {
		t.Error("NewFileGroupProvider() expected error for nonexistent path")
	}
}

func TestNewFileGroupProvider_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("not json"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFileGroupProvider(FileGroupProviderConfig{
		GroupsPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFileGroupProvider() expected error for invalid JSON")
	}
}

func TestNewFileGroupProvider_InvalidGroup(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	// Group with empty ID
	if err := os.WriteFile(invalidPath, []byte(`[{"id": "", "name": "test"}]`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := NewFileGroupProvider(FileGroupProviderConfig{
		GroupsPath: invalidPath,
	})
	if err == nil {
		t.Error("NewFileGroupProvider() expected error for invalid group")
	}
}
