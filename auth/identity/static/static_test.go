package static

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/msimon/nauts/auth/identity"
)

func TestVerify_ValidCredentials(t *testing.T) {
	sp := createTestProvider(t)

	user, err := sp.Verify(context.Background(), UsernamePassword{
		Username: "alice",
		Password: "secret123",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "alice" {
		t.Errorf("user.ID = %q, want %q", user.ID, "alice")
	}
	if user.Account != "ACME" {
		t.Errorf("user.Account = %q, want %q", user.Account, "ACME")
	}
	if len(user.Groups) != 1 || user.Groups[0] != "workers" {
		t.Errorf("user.Groups = %v, want [workers]", user.Groups)
	}
}

func TestVerify_InvalidPassword(t *testing.T) {
	sp := createTestProvider(t)

	_, err := sp.Verify(context.Background(), UsernamePassword{
		Username: "alice",
		Password: "wrongpassword",
	})
	if !errors.Is(err, identity.ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, identity.ErrInvalidCredentials)
	}
}

func TestVerify_UserNotFound(t *testing.T) {
	sp := createTestProvider(t)

	_, err := sp.Verify(context.Background(), UsernamePassword{
		Username: "nonexistent",
		Password: "password",
	})
	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Errorf("Verify() error = %v, want %v", err, identity.ErrUserNotFound)
	}
}

func TestVerify_InvalidTokenType(t *testing.T) {
	sp := createTestProvider(t)

	// Pass a string instead of UsernamePassword
	_, err := sp.Verify(context.Background(), "invalid token")
	if !errors.Is(err, identity.ErrInvalidTokenType) {
		t.Errorf("Verify() error = %v, want %v", err, identity.ErrInvalidTokenType)
	}
}

func TestGetUser_Found(t *testing.T) {
	sp := createTestProvider(t)

	user, err := sp.GetUser(context.Background(), "bob")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.ID != "bob" {
		t.Errorf("user.ID = %q, want %q", user.ID, "bob")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	sp := createTestProvider(t)

	_, err := sp.GetUser(context.Background(), "nonexistent")
	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Errorf("GetUser() error = %v, want %v", err, identity.ErrUserNotFound)
	}
}

func TestNew_EmptyConfig(t *testing.T) {
	sp, err := New(Config{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = sp.GetUser(context.Background(), "any")
	if !errors.Is(err, identity.ErrUserNotFound) {
		t.Errorf("GetUser() error = %v, want %v", err, identity.ErrUserNotFound)
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, err := New(Config{
		UsersPath: "/nonexistent/path",
	})
	if err == nil {
		t.Error("New() expected error for nonexistent path")
	}
}

func TestNew_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := New(Config{
		UsersPath: invalidFile,
	})
	if err == nil {
		t.Error("New() expected error for invalid JSON")
	}
}

// createTestProvider creates a StaticProvider with test users.
func createTestProvider(t *testing.T) *StaticProvider {
	t.Helper()

	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	// Generate bcrypt hashes for test passwords
	aliceHash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	bobHash, _ := bcrypt.GenerateFromPassword([]byte("password456"), bcrypt.DefaultCost)

	content := `{
  "users": {
    "alice": {
      "account": "ACME",
      "groups": ["workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {
        "department": "engineering"
      }
    },
    "bob": {
      "account": "ACME",
      "groups": ["viewers"],
      "passwordHash": "` + string(bobHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	sp, err := New(Config{UsersPath: usersFile})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return sp
}
