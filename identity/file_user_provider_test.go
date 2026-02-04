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

	user, err := fp.Verify(context.Background(), AuthRequest{Token: "alice:secret123"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "alice" {
		t.Errorf("user.ID = %q, want %q", user.ID, "alice")
	}
	if user.Account != "ACME" {
		t.Errorf("user.Account = %q, want %q", user.Account, "ACME")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "workers" {
		t.Errorf("user.Roles = %v, want [workers]", user.Roles)
	}
}

func TestVerify_InvalidPassword(t *testing.T) {
	fp := createTestProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{Token: "alice:wrongpassword"})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestVerify_UserNotFound(t *testing.T) {
	fp := createTestProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{Token: "nonexistent:password"})
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("Verify() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestVerify_InvalidTokenType(t *testing.T) {
	fp := createTestProvider(t)

	// Pass a token without colon separator
	_, err := fp.Verify(context.Background(), AuthRequest{Token: "invalid token"})
	if !errors.Is(err, ErrInvalidTokenType) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidTokenType)
	}
}

func TestGetUser_Found(t *testing.T) {
	fp := createTestProvider(t)

	user, err := fp.GetUser(context.Background(), "bob")
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.ID != "bob" {
		t.Errorf("user.ID = %q, want %q", user.ID, "bob")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	fp := createTestProvider(t)

	_, err := fp.GetUser(context.Background(), "nonexistent")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetUser() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestNewFileUserIdentityProvider_EmptyConfig(t *testing.T) {
	fp, err := NewFileUserIdentityProvider(FileUserIdentityProviderConfig{})
	if err != nil {
		t.Fatalf("NewFileUserIdentityProvider() error = %v", err)
	}

	_, err = fp.GetUser(context.Background(), "any")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetUser() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestNewFileUserIdentityProvider_InvalidPath(t *testing.T) {
	_, err := NewFileUserIdentityProvider(FileUserIdentityProviderConfig{
		UsersPath: "/nonexistent/path",
	})
	if err == nil {
		t.Error("NewFileUserIdentityProvider() expected error for nonexistent path")
	}
}

func TestNewFileUserIdentityProvider_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("not json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	_, err := NewFileUserIdentityProvider(FileUserIdentityProviderConfig{
		UsersPath: invalidFile,
	})
	if err == nil {
		t.Error("NewFileUserIdentityProvider() expected error for invalid JSON")
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
	if user.Account != "CORP" {
		t.Errorf("user.Account = %q, want %q", user.Account, "CORP")
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

func TestVerify_MultipleAccounts_NoAccountSpecified(t *testing.T) {
	fp := createMultiAccountProvider(t)

	_, err := fp.Verify(context.Background(), AuthRequest{
		Token: "charlie:secret789",
	})
	if !errors.Is(err, ErrAccountRequired) {
		t.Errorf("Verify() error = %v, want %v", err, ErrAccountRequired)
	}
}

func TestVerify_SingleAccount_AccountSpecifiedIgnored(t *testing.T) {
	fp := createTestProvider(t)

	// When user has single account, the specified account is ignored
	user, err := fp.Verify(context.Background(), AuthRequest{
		Account: "IGNORED",
		Token:   "alice:secret123",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	// Should use the user's single account, not the requested one
	if user.Account != "ACME" {
		t.Errorf("user.Account = %q, want %q (single account should be used)", user.Account, "ACME")
	}
}

// createMultiAccountProvider creates a provider with a user having multiple accounts.
func createMultiAccountProvider(t *testing.T) *FileUserIdentityProvider {
	t.Helper()

	tmpDir := t.TempDir()
	usersFile := filepath.Join(tmpDir, "users.json")

	charlieHash, _ := bcrypt.GenerateFromPassword([]byte("secret789"), bcrypt.DefaultCost)

	content := `{
  "users": {
    "charlie": {
      "accounts": ["ACME", "CORP"],
      "roles": ["admin"],
      "passwordHash": "` + string(charlieHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileUserIdentityProvider(FileUserIdentityProviderConfig{UsersPath: usersFile})
	if err != nil {
		t.Fatalf("NewFileUserIdentityProvider() error = %v", err)
	}

	return fp
}

// createTestProvider creates a FileUserIdentityProvider with test users.
func createTestProvider(t *testing.T) *FileUserIdentityProvider {
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
      "roles": ["workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {
        "department": "engineering"
      }
    },
    "bob": {
      "accounts": ["ACME"],
      "roles": ["viewers"],
      "passwordHash": "` + string(bobHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileUserIdentityProvider(FileUserIdentityProviderConfig{UsersPath: usersFile})
	if err != nil {
		t.Fatalf("NewFileUserIdentityProvider() error = %v", err)
	}

	return fp
}
