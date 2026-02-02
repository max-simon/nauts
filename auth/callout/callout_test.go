package callout

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/msimon/nauts/auth"
	"github.com/msimon/nauts/auth/identity"
	"github.com/msimon/nauts/auth/identity/static"
	"github.com/msimon/nauts/auth/provider/grouppolicyprovider"
)

func TestAuthenticate_Success(t *testing.T) {
	svc := createTestCalloutService(t)

	result, err := svc.Authenticate(context.Background(), static.UsernamePassword{
		Username: "alice",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if result.User == nil {
		t.Fatal("result.User is nil")
	}
	if result.User.ID != "alice" {
		t.Errorf("result.User.ID = %q, want %q", result.User.ID, "alice")
	}
	if result.Permissions == nil {
		t.Fatal("result.Permissions is nil")
	}
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	svc := createTestCalloutService(t)

	_, err := svc.Authenticate(context.Background(), static.UsernamePassword{
		Username: "alice",
		Password: "wrongpassword",
	})
	if err == nil {
		t.Fatal("Authenticate() expected error")
	}

	var calloutErr *CalloutError
	if !errors.As(err, &calloutErr) {
		t.Fatalf("error is not CalloutError: %T", err)
	}
	if calloutErr.Phase != "authenticate" {
		t.Errorf("CalloutError.Phase = %q, want %q", calloutErr.Phase, "authenticate")
	}
	if !errors.Is(calloutErr.Err, identity.ErrInvalidCredentials) {
		t.Errorf("underlying error = %v, want %v", calloutErr.Err, identity.ErrInvalidCredentials)
	}
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	svc := createTestCalloutService(t)

	_, err := svc.Authenticate(context.Background(), static.UsernamePassword{
		Username: "nonexistent",
		Password: "password",
	})
	if err == nil {
		t.Fatal("Authenticate() expected error")
	}

	var calloutErr *CalloutError
	if !errors.As(err, &calloutErr) {
		t.Fatalf("error is not CalloutError: %T", err)
	}
	if calloutErr.Phase != "authenticate" {
		t.Errorf("CalloutError.Phase = %q, want %q", calloutErr.Phase, "authenticate")
	}
	if !errors.Is(calloutErr.Err, identity.ErrUserNotFound) {
		t.Errorf("underlying error = %v, want %v", calloutErr.Err, identity.ErrUserNotFound)
	}
}

func createTestCalloutService(t *testing.T) *CalloutService {
	t.Helper()

	// Create temp directory with test data
	tmpDir := t.TempDir()

	// Create policies file
	policiesFile := filepath.Join(tmpDir, "policies.json")
	policiesContent := `[
  {
    "id": "allow-basic",
    "name": "Basic Access",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:test.>"]
      }
    ]
  }
]`
	if err := os.WriteFile(policiesFile, []byte(policiesContent), 0644); err != nil {
		t.Fatalf("Failed to write policies file: %v", err)
	}

	// Create groups file
	groupsDir := filepath.Join(tmpDir, "groups")
	if err := os.MkdirAll(groupsDir, 0755); err != nil {
		t.Fatalf("Failed to create groups directory: %v", err)
	}
	groupsFile := filepath.Join(groupsDir, "groups.json")
	groupsContent := `[
  {
    "id": "default",
    "name": "Default",
    "policies": []
  },
  {
    "id": "workers",
    "name": "Workers",
    "policies": ["allow-basic"]
  }
]`
	if err := os.WriteFile(groupsFile, []byte(groupsContent), 0644); err != nil {
		t.Fatalf("Failed to write groups file: %v", err)
	}

	// Create users file
	usersFile := filepath.Join(tmpDir, "users.json")
	aliceHash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	usersContent := `{
  "users": {
    "alice": {
      "account": "ACME",
      "groups": ["workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {}
    }
  }
}`
	if err := os.WriteFile(usersFile, []byte(usersContent), 0644); err != nil {
		t.Fatalf("Failed to write users file: %v", err)
	}

	// Create providers
	policyProvider, err := grouppolicyprovider.NewFile(grouppolicyprovider.FileConfig{
		PoliciesPath: policiesFile,
		GroupsPath:   groupsFile,
	})
	if err != nil {
		t.Fatalf("Failed to create policy provider: %v", err)
	}

	identityProvider, err := static.New(static.Config{
		UsersPath: usersFile,
	})
	if err != nil {
		t.Fatalf("Failed to create identity provider: %v", err)
	}

	// Create services
	authService := auth.NewAuthService(policyProvider)
	return NewCalloutService(identityProvider, authService)
}
