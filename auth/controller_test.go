package auth

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/provider"
)

// testLogger captures log messages for testing
type testLogger struct {
	infos    []string
	warnings []string
	debugs   []string
}

func (l *testLogger) Info(msg string, args ...any) {
	l.infos = append(l.infos, msg)
}

func (l *testLogger) Warn(msg string, args ...any) {
	l.warnings = append(l.warnings, msg)
}

func (l *testLogger) Debug(msg string, args ...any) {
	l.debugs = append(l.debugs, msg)
}

func TestResolveUser_ValidCredentials(t *testing.T) {
	ctrl := createTestController(t)

	user, err := ctrl.ResolveUser(context.Background(), "alice:secret123")
	if err != nil {
		t.Fatalf("ResolveUser() error = %v", err)
	}

	if user.ID != "alice" {
		t.Errorf("user.ID = %q, want %q", user.ID, "alice")
	}
	if user.Account != "test-account" {
		t.Errorf("user.Account = %q, want %q", user.Account, "test-account")
	}
}

func TestResolveUser_InvalidCredentials(t *testing.T) {
	ctrl := createTestController(t)

	_, err := ctrl.ResolveUser(context.Background(), "alice:wrongpassword")
	if err == nil {
		t.Fatal("ResolveUser() expected error")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("error is not AuthError: %T", err)
	}
	if authErr.Phase != "resolve_user" {
		t.Errorf("AuthError.Phase = %q, want %q", authErr.Phase, "resolve_user")
	}
}

func TestResolveNatsPermissions_Basic(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "alice",
		Account: "test-account",
		Groups:  []string{"workers"},
		Attributes: map[string]string{
			"department": "engineering",
		},
	}

	perms, err := ctrl.ResolveNatsPermissions(context.Background(), user)
	if err != nil {
		t.Fatalf("ResolveNatsPermissions() error = %v", err)
	}

	if perms.IsEmpty() {
		t.Error("Expected non-empty permissions")
	}
}

func TestResolveNatsPermissions_NilUser(t *testing.T) {
	ctrl := createTestController(t)

	_, err := ctrl.ResolveNatsPermissions(context.Background(), nil)
	if err == nil {
		t.Error("Expected error for nil user")
	}
}

func TestResolveNatsPermissions_DefaultGroup(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "test",
		Account: "test-account",
		Groups:  []string{},
	}

	perms, err := ctrl.ResolveNatsPermissions(context.Background(), user)
	if err != nil {
		t.Fatalf("ResolveNatsPermissions() error = %v", err)
	}

	// Default group should be processed (even if it doesn't exist or has no policies)
	_ = perms
}

func TestCreateUserJWT(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "alice",
		Account: "test-account",
		Groups:  []string{"workers"},
	}

	perms, err := ctrl.ResolveNatsPermissions(context.Background(), user)
	if err != nil {
		t.Fatalf("ResolveNatsPermissions() error = %v", err)
	}

	// Create a user keypair for testing
	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	token, err := ctrl.CreateUserJWT(context.Background(), user, userPub, perms, time.Hour)
	if err != nil {
		t.Fatalf("CreateUserJWT() error = %v", err)
	}

	if token == "" {
		t.Error("Expected non-empty token")
	}
}

func TestCreateUserJWT_NilUser(t *testing.T) {
	ctrl := createTestController(t)

	_, err := ctrl.CreateUserJWT(context.Background(), nil, "UABC", nil, time.Hour)
	if err == nil {
		t.Error("Expected error for nil user")
	}
}

func TestCreateUserJWT_AccountNotFound(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "alice",
		Account: "nonexistent-account",
		Groups:  []string{},
	}

	_, err := ctrl.CreateUserJWT(context.Background(), user, "UABC", nil, time.Hour)
	if err == nil {
		t.Error("Expected error for nonexistent account")
	}
}

func TestAuthenticate_Success(t *testing.T) {
	ctrl := createTestController(t)

	// Create a user keypair for testing
	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	result, err := ctrl.Authenticate(context.Background(), natsjwt.ConnectOptions{
		Token: "alice:secret123",
	}, userPub, time.Hour)
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
	if result.JWT == "" {
		t.Error("result.JWT is empty")
	}
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	ctrl := createTestController(t)

	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	_, err = ctrl.Authenticate(context.Background(), natsjwt.ConnectOptions{
		Token: "alice:wrongpassword",
	}, userPub, time.Hour)
	if err == nil {
		t.Fatal("Authenticate() expected error")
	}
}

func TestAuthenticate_EphemeralKey(t *testing.T) {
	ctrl := createTestController(t)

	// Authenticate with empty userPublicKey - should generate ephemeral key
	result, err := ctrl.Authenticate(context.Background(), natsjwt.ConnectOptions{
		Token: "alice:secret123",
	}, "", time.Hour) // Empty userPublicKey
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if result.User == nil {
		t.Fatal("result.User is nil")
	}
	if result.User.ID != "alice" {
		t.Errorf("result.User.ID = %q, want %q", result.User.ID, "alice")
	}
	if result.JWT == "" {
		t.Error("result.JWT is empty")
	}

	// Verify the JWT was created (it should contain an ephemeral user public key)
	// The JWT should be decodable
	claims, err := natsjwt.DecodeUserClaims(result.JWT)
	if err != nil {
		t.Fatalf("decoding JWT: %v", err)
	}

	// The subject should be a valid user public key (starts with 'U')
	if len(claims.Subject) == 0 || claims.Subject[0] != 'U' {
		t.Errorf("JWT subject = %q, want user public key starting with 'U'", claims.Subject)
	}
}

// createTestController creates an AuthController with test providers.
func createTestController(t *testing.T) *AuthController {
	t.Helper()

	tmpDir := t.TempDir()

	// Create entity provider (nsc structure)
	entityProvider := createTestEntityProvider(t, tmpDir)

	// Create nauts provider (policies and groups)
	nautsProvider := createTestNautsProvider(t, tmpDir)

	// Create identity provider (users)
	identityProvider := createTestIdentityProvider(t, tmpDir)

	logger := &testLogger{}
	return NewAuthController(entityProvider, nautsProvider, identityProvider, WithLogger(logger))
}

func createTestEntityProvider(t *testing.T, tmpDir string) provider.EntityProvider {
	t.Helper()

	// Create account keypair
	accKp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating account keypair: %v", err)
	}
	accPub, err := accKp.PublicKey()
	if err != nil {
		t.Fatalf("getting account public key: %v", err)
	}
	accSeed, err := accKp.Seed()
	if err != nil {
		t.Fatalf("getting account seed: %v", err)
	}

	// Write account seed file
	accKeyPath := filepath.Join(tmpDir, "account.nk")
	if err := os.WriteFile(accKeyPath, accSeed, 0600); err != nil {
		t.Fatalf("writing account seed: %v", err)
	}

	ep, err := provider.NewStaticEntityProvider(provider.StaticEntityProviderConfig{
		PublicKey:      accPub,
		PrivateKeyPath: accKeyPath,
		Accounts:       []string{"test-account"},
	})
	if err != nil {
		t.Fatalf("creating entity provider: %v", err)
	}

	return ep
}

func createTestNautsProvider(t *testing.T, tmpDir string) provider.NautsProvider {
	t.Helper()

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
		t.Fatalf("writing policies file: %v", err)
	}

	groupsFile := filepath.Join(tmpDir, "groups.json")
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
		t.Fatalf("writing groups file: %v", err)
	}

	np, err := provider.NewFileNautsProvider(provider.FileNautsProviderConfig{
		PoliciesPath: policiesFile,
		GroupsPath:   groupsFile,
	})
	if err != nil {
		t.Fatalf("creating nauts provider: %v", err)
	}

	return np
}

func createTestIdentityProvider(t *testing.T, tmpDir string) identity.UserIdentityProvider {
	t.Helper()

	usersFile := filepath.Join(tmpDir, "users.json")
	aliceHash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	usersContent := `{
  "users": {
    "alice": {
      "account": "test-account",
      "groups": ["workers"],
      "passwordHash": "` + string(aliceHash) + `",
      "attributes": {
        "department": "engineering"
      }
    }
  }
}`
	if err := os.WriteFile(usersFile, []byte(usersContent), 0644); err != nil {
		t.Fatalf("writing users file: %v", err)
	}

	ip, err := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
		UsersPath: usersFile,
	})
	if err != nil {
		t.Fatalf("creating identity provider: %v", err)
	}

	return ip
}
