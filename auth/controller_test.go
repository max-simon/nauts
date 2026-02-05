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

	user, err := ctrl.ResolveUser(context.Background(), `{"token":"alice:secret123"}`)
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

	_, err := ctrl.ResolveUser(context.Background(), `{"token":"alice:wrongpassword"}`)
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
		Roles:   []string{"workers"},
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

func TestResolveNatsPermissions_DefaultRole(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "test",
		Account: "test-account",
		Roles:   []string{},
	}

	perms, err := ctrl.ResolveNatsPermissions(context.Background(), user)
	if err != nil {
		t.Fatalf("ResolveNatsPermissions() error = %v", err)
	}

	// Default role should be processed (even if it doesn't exist or has no policies)
	_ = perms
}

func TestCreateUserJWT(t *testing.T) {
	ctrl := createTestController(t)

	user := &identity.User{
		ID:      "alice",
		Account: "test-account",
		Roles:   []string{"workers"},
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
		Roles:   []string{},
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
		Token: `{"token":"alice:secret123"}`,
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
		Token: `{"token":"alice:wrongpassword"}`,
	}, userPub, time.Hour)
	if err == nil {
		t.Fatal("Authenticate() expected error")
	}
}

func TestAuthenticate_EphemeralKey(t *testing.T) {
	ctrl := createTestController(t)

	// Authenticate with empty userPublicKey - should generate ephemeral key
	result, err := ctrl.Authenticate(context.Background(), natsjwt.ConnectOptions{
		Token: `{"token":"alice:secret123"}`,
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

	// Create account provider
	accountProvider := createTestAccountProvider(t, tmpDir)

	// Create role provider
	roleProvider := createTestRoleProvider(t, tmpDir)

	// Create policy provider
	policyProvider := createTestPolicyProvider(t, tmpDir)

	// Create identity provider (users)
	identityProvider := createTestIdentityProvider(t, tmpDir)

	logger := &testLogger{}
	return NewAuthController(accountProvider, roleProvider, policyProvider, identityProvider, WithLogger(logger))
}

func createTestAccountProvider(t *testing.T, tmpDir string) provider.AccountProvider {
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

	ap, err := provider.NewStaticAccountProvider(provider.StaticAccountProviderConfig{
		PublicKey:      accPub,
		PrivateKeyPath: accKeyPath,
		Accounts:       []string{"test-account"},
	})
	if err != nil {
		t.Fatalf("creating account provider: %v", err)
	}

	return ap
}

func createTestRoleProvider(t *testing.T, tmpDir string) provider.RoleProvider {
	t.Helper()

	rolesFile := filepath.Join(tmpDir, "roles.json")
	rolesContent := `[
  {
    "name": "default",
    "account": "*",
    "policies": []
  },
  {
    "name": "workers",
    "account": "*",
    "policies": ["allow-basic"]
  }
]`
	if err := os.WriteFile(rolesFile, []byte(rolesContent), 0644); err != nil {
		t.Fatalf("writing roles file: %v", err)
	}

	rp, err := provider.NewFileRoleProvider(provider.FileRoleProviderConfig{
		RolesPath: rolesFile,
	})
	if err != nil {
		t.Fatalf("creating role provider: %v", err)
	}

	return rp
}

func createTestPolicyProvider(t *testing.T, tmpDir string) provider.PolicyProvider {
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

	pp, err := provider.NewFilePolicyProvider(provider.FilePolicyProviderConfig{
		PoliciesPath: policiesFile,
	})
	if err != nil {
		t.Fatalf("creating policy provider: %v", err)
	}

	return pp
}

func createTestIdentityProvider(t *testing.T, tmpDir string) identity.AuthenticationProvider {
	t.Helper()

	usersFile := filepath.Join(tmpDir, "users.json")
	aliceHash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.DefaultCost)
	usersContent := `{
  "users": {
    "alice": {
      "accounts": ["test-account"],
      "roles": ["workers"],
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

	ip, err := identity.NewFileAuthenticationProvider(identity.FileAuthenticationProviderConfig{
		UsersPath: usersFile,
	})
	if err != nil {
		t.Fatalf("creating identity provider: %v", err)
	}

	return ip
}
