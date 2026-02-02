package auth

import (
	"context"
	"testing"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/provider/grouppolicyprovider"
)

// testLogger captures warnings for testing
type testLogger struct {
	warnings []string
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.warnings = append(l.warnings, msg)
}

func newTestProvider(t *testing.T) *grouppolicyprovider.FileGroupPolicyProvider {
	t.Helper()

	cfg := grouppolicyprovider.FileConfig{
		PoliciesPath: "../testdata/policies.json",
		GroupsPath:   "../testdata/groups/groups.json",
	}

	fp, err := grouppolicyprovider.NewFile(cfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	return fp
}

func TestGetNatsPermission_Basic(t *testing.T) {
	provider := newTestProvider(t)
	logger := &testLogger{}
	svc := NewAuthService(provider, WithLogger(logger))

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
	provider := newTestProvider(t)
	svc := NewAuthService(provider)

	_, err := svc.GetNatsPermission(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil user")
	}
}

func TestGetNatsPermission_DefaultGroup(t *testing.T) {
	provider := newTestProvider(t)
	svc := NewAuthService(provider)

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
