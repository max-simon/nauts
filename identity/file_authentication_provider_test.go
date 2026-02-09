package identity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerify_ValidCredentials(t *testing.T) {
	fp := createTestProvider(t)

	user, err := fp.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "alice:secret123"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "alice" {
		t.Errorf("user.ID = %q, want %q", user.ID, "alice")
	}
	if len(user.Roles) != 1 || user.Roles[0].Account != "ACME" || user.Roles[0].Name != "workers" {
		t.Errorf("user.Roles = %v, want [{ACME workers}]", user.Roles)
	}
}

func TestVerify_InvalidPassword(t *testing.T) {
	fp := createTestProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "alice:wrongpassword"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestVerify_UserNotFound(t *testing.T) {
	fp := createTestProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "nonexistent:password"})
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Verify() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestVerify_InvalidTokenType(t *testing.T) {
	fp := createTestProvider(t)

	// Pass a token without colon separator
	_, err := fp.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "invalid token"})
	if !errors.Is(err, ErrInvalidTokenType) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidTokenType)
	}
}

func TestNewFileAuthenticationProvider_InvalidPath(t *testing.T) {
	_, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		UsersPath: "/nonexistent/path/users.json",
	})
	if err == nil {
		t.Error("NewFileAuthenticationProvider() expected error for nonexistent path")
	}
}

func TestNewFileAuthenticationProvider_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		UsersPath: invalidFile,
	})
	if err == nil {
		t.Error("NewFileAuthenticationProvider() expected error for invalid JSON")
	}
}

func TestVerify_MultipleAccounts_WithAccountSpecified(t *testing.T) {
	fp := createMultiAccountProvider(t)

	user, err := fp.Verify(context.Background(), AuthRequest{
		Account: "CORP",
		Token:   "charlie:secret789",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "charlie" {
		t.Errorf("user.ID = %q, want %q", user.ID, "charlie")
	}
	// Should return all roles (filtering done by AuthController)
	if len(user.Roles) != 2 {
		t.Errorf("user.Roles length = %d, want 2", len(user.Roles))
	}
	// Verify both accounts are present
	accounts := make(map[string]string)
	for _, role := range user.Roles {
		accounts[role.Account] = role.Name
	}
	if accounts["ACME"] != "admin" || accounts["CORP"] != "admin" {
		t.Errorf("user.Roles = %v, want ACME.admin and CORP.admin", user.Roles)
	}
}

func TestVerify_MultipleAccounts_InvalidAccount(t *testing.T) {
	fp := createMultiAccountProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{
		Account: "INVALID",
		Token:   "charlie:secret789",
	})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidAccount)
	}
}

func TestVerify_InvalidAccount(t *testing.T) {
	fp := createTestProvider(t)

	// Requesting wrong account for user
	_, err := fp.Verify(context.Background(), AuthRequest{
		Account: "INVALID",
		Token:   "alice:secret123",
	})
	if !errors.Is(err, ErrInvalidAccount) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidAccount)
	}
}

// createMultiAccountProvider creates a provider with a user having multiple accounts.
func createMultiAccountProvider(t *testing.T) *FileAuthenticationProvider {
	t.Helper()

	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	charlieHash, _ := bcrypt.GenerateFromPassword([]byte("secret789"), bcrypt.DefaultCost)

	content := `{
  "users": {
    "charlie": {
      "accounts": ["ACME", "CORP"],
      "roles": ["ACME.admin", "CORP.admin"],
      "passwordHash": "` + string(charlieHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{UsersPath: usersFile})
	if err != nil {
		t.Fatalf("NewFileAuthenticationProvider() error = %v", err)
	}

	return fp
}

// createTestProvider creates a FileAuthenticationProvider with test users.
func createTestProvider(t *testing.T) *FileAuthenticationProvider {
	t.Helper()

	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	// Generate bcrypt hashes for test passwords
	aliceHash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	bobHash, _ := bcrypt.GenerateFromPassword([]byte("password456"), bcrypt.DefaultCost)

	content := `{
  "users": {
    "alice": {
      "accounts": ["ACME"],
      "roles": ["ACME.workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {
        "department": "engineering"
      }
    },
    "bob": {
      "accounts": ["ACME"],
      "roles": ["ACME.viewers"],
      "passwordHash": "` + string(bobHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{UsersPath: usersFile})
	if err != nil {
		t.Fatalf("NewFileAuthenticationProvider() error = %v", err)
	}

	return fp
}
