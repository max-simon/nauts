package auth

import (
	"context"
	"testing"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/store/filestore"
)

// testLogger captures warnings for testing
type testLogger struct {
	warnings []string
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.warnings = append(l.warnings, msg)
}

func newTestStore(t *testing.T) *filestore.FileStore {
	t.Helper()

	cfg := filestore.Config{
		PoliciesPath: "../testdata/policies.json",
		GroupsPath:   "../testdata/groups/groups.json",
	}

	fs, err := filestore.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create filestore: %v", err)
	}
	return fs
}

func TestGetNatsPermission_Basic(t *testing.T) {
	store := newTestStore(t)
	logger := &testLogger{}
	svc := NewAuthService(store, WithLogger(logger))

	// Create test user
	user := &model.User{
		ID:      "alice",
		Account: "ACME",
		Groups:  []string{"workers"},
		Attributes: map[string]string{
			"department": "engineering",
		},
	}

	perms, err := svc.GetNatsPermission(context.Background(), user)
	if err != nil {
		t.Fatalf("GetNatsPermission error: %v", err)
	}

	// Should have some permissions
	if perms.IsEmpty() {
		t.Error("Expected non-empty permissions")
	}
}

func TestGetNatsPermission_NilUser(t *testing.T) {
	store := newTestStore(t)
	svc := NewAuthService(store)

	_, err := svc.GetNatsPermission(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil user")
	}
}

func TestGetNatsPermission_DefaultGroup(t *testing.T) {
	store := newTestStore(t)
	svc := NewAuthService(store)

	// User with no explicit groups should still get default group
	user := &model.User{
		ID:      "test",
		Account: "test-account",
		Groups:  []string{},
	}

	perms, err := svc.GetNatsPermission(context.Background(), user)
	if err != nil {
		t.Fatalf("GetNatsPermission error: %v", err)
	}

	// Default group should be processed (even if it doesn't exist or has no policies)
	// The result may be empty, but should not error
	_ = perms
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
