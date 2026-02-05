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
	if len(user.Roles) != 1 {
		t.Fatalf("user.Roles length = %d, want 1", len(user.Roles))
	}
	if user.Roles[0].Account != "ACME" || user.Roles[0].Role != "workers" {
		t.Errorf("user.Roles[0] = {Account: %q, Role: %q}, want {Account: \"ACME\", Role: \"workers\"}", user.Roles[0].Account, user.Roles[0].Role)
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

func TestNewFileAuthenticationProvider_EmptyConfig(t *testing.T) {
	fp, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		ID:       "test",
		Accounts: []string{"*"},
	})
	if err != nil {
		t.Fatalf("NewFileAuthenticationProvider() error = %v", err)
	}

	_, err = fp.GetUser(context.Background(), "any")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetUser() error = %v, want %v", err, ErrUserNotFound)
	}
}

func TestNewFileAuthenticationProvider_InvalidPath(t *testing.T) {
	_, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		ID:        "test",
		Accounts:  []string{"*"},
		UsersPath: "/nonexistent/path",
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
		ID:        "test",
		Accounts:  []string{"*"},
		UsersPath: invalidFile,
	})
	if err == nil {
		t.Error("NewFileAuthenticationProvider() expected error for invalid JSON")
	}
}

func TestVerify_MultipleAccounts(t *testing.T) {
	fp := createMultiAccountProvider(t)

	user, err := fp.Verify(context.Background(), AuthRequest{
		Token: "charlie:secret789",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "charlie" {
		t.Errorf("user.ID = %q, want %q", user.ID, "charlie")
	}
	// Should have roles for both accounts
	if len(user.Roles) != 2 {
		t.Fatalf("user.Roles length = %d, want 2", len(user.Roles))
	}
	// Check both account roles are present
	hasACME := false
	hasCORP := false
	for _, role := range user.Roles {
		if role.Account == "ACME" && role.Role == "admin" {
			hasACME = true
		}
		if role.Account == "CORP" && role.Role == "admin" {
			hasCORP = true
		}
	}
	if !hasACME {
		t.Error("Expected ACME.admin role not found")
	}
	if !hasCORP {
		t.Error("Expected CORP.admin role not found")
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
      "roles": ["ACME.admin", "CORP.admin"],
      "passwordHash": "` + string(charlieHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		ID:        "test",
		Accounts:  []string{"*"},
		UsersPath: usersFile,
	})
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
      "roles": ["ACME.workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {
        "department": "engineering"
      }
    },
    "bob": {
      "roles": ["ACME.viewers"],
      "passwordHash": "` + string(bobHash) + `",
      "attributes": {}
    }
  }
}`

	if err := os.WriteFile(usersFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	fp, err := NewFileAuthenticationProvider(FileAuthenticationProviderConfig{
		ID:        "test",
		Accounts:  []string{"*"},
		UsersPath: usersFile,
	})
	if err != nil {
		t.Fatalf("NewFileAuthenticationProvider() error = %v", err)
	}

	return fp
}
