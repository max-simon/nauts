package identity

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// generateTestKeyPair generates an RSA key pair for testing.
func generateTestKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshaling public key: %v", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	pubKeyPEMB64 := base64.StdEncoding.EncodeToString(pubKeyPEM)

	return privateKey, pubKeyPEMB64
}

// createTestJWT creates a signed JWT for testing.
func createTestJWT(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}

	return tokenString
}

func TestJwtAuthenticationProvider_Verify_Success(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"tenant-a-*", "dev"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"tenant-a-acc.admin", "tenant-a-acc.viewer"},
			},
		},
		"email": "user@example.com",
		"name":  "Test User",
	})

	user, err := provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-123")
	}
	if user.Account != "tenant-a-acc" {
		t.Errorf("user.Account = %q, want %q", user.Account, "tenant-a-acc")
	}
	if len(user.Roles) != 2 {
		t.Errorf("len(user.Roles) = %d, want 2", len(user.Roles))
	}

	// Check roles are stripped of account prefix
	roleSet := make(map[string]bool)
	for _, r := range user.Roles {
		roleSet[r] = true
	}
	if !roleSet["admin"] {
		t.Error("expected role 'admin' not found")
	}
	if !roleSet["viewer"] {
		t.Error("expected role 'viewer' not found")
	}

	// Check attributes
	if user.Attributes["email"] != "user@example.com" {
		t.Errorf("user.Attributes[email] = %q, want %q", user.Attributes["email"], "user@example.com")
	}
	if user.Attributes["name"] != "Test User" {
		t.Errorf("user.Attributes[name] = %q, want %q", user.Attributes["name"], "Test User")
	}
}

func TestJwtAuthenticationProvider_Verify_WithExplicitAccount(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"}, // Allow all accounts
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// User has roles in multiple accounts
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin", "account-b.viewer"},
			},
		},
	})

	// Explicitly request account-a
	user, err := provider.Verify(context.Background(), AuthRequest{
		Token:   tokenString,
		Account: "account-a",
	})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.Account != "account-a" {
		t.Errorf("user.Account = %q, want %q", user.Account, "account-a")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "admin" {
		t.Errorf("user.Roles = %v, want [admin]", user.Roles)
	}
}

func TestJwtAuthenticationProvider_Verify_IssuerNotConfigured(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// JWT from unknown issuer
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://unknown-issuer.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrIssuerNotConfigured) {
		t.Errorf("Verify() error = %v, want %v", err, ErrIssuerNotConfigured)
	}
}

func TestJwtAuthenticationProvider_Verify_IssuerNotAllowed(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"tenant-a-*"}, // Only tenant-a accounts
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// User tries to access tenant-b account
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"tenant-b-acc.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrIssuerNotAllowed) {
		t.Errorf("Verify() error = %v, want %v", err, ErrIssuerNotAllowed)
	}
}

func TestJwtAuthenticationProvider_Verify_AmbiguousAccount(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Roles span multiple accounts, no account specified
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin", "account-b.viewer"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrAmbiguousAccount) {
		t.Errorf("Verify() error = %v, want %v", err, ErrAmbiguousAccount)
	}
}

func TestJwtAuthenticationProvider_Verify_NoRoles(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// No roles in token
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrNoRolesFound) {
		t.Errorf("Verify() error = %v, want %v", err, ErrNoRolesFound)
	}
}

func TestJwtAuthenticationProvider_Verify_InvalidRoleFormat(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Only invalid role formats (no dot separator)
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"invalid-role", "another-invalid"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrNoRolesFound) {
		t.Errorf("Verify() error = %v, want %v", err, ErrNoRolesFound)
	}
}

func TestJwtAuthenticationProvider_Verify_ExpiredToken(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Expired token
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(-time.Hour).Unix(), // Expired
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestJwtAuthenticationProvider_Verify_InvalidSignature(t *testing.T) {
	_, publicKeyPEM1 := generateTestKeyPair(t)
	privateKey2, _ := generateTestKeyPair(t)

	// Provider configured with key1
	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM1,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Token signed with key2 (different key)
	tokenString := createTestJWT(t, privateKey2, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestJwtAuthenticationProvider_Verify_WildcardInAccount(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey: publicKeyPEM,
				Accounts:  []string{"*"},
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account.admin"},
			},
		},
	})

	// Request with wildcard in account
	_, err = provider.Verify(context.Background(), AuthRequest{
		Token:   tokenString,
		Account: "tenant-*",
	})
	if !errors.Is(err, ErrWildcardInAccount) {
		t.Errorf("Verify() error = %v, want %v", err, ErrWildcardInAccount)
	}
}

func TestJwtAuthenticationProvider_CustomRolesPath(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth.example.com": {
				PublicKey:      publicKeyPEM,
				Accounts:       []string{"*"},
				RolesClaimPath: "realm_access.roles", // Custom path per issuer
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]any{
			"roles": []any{"myaccount.myrole"},
		},
	})

	user, err := provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.Account != "myaccount" {
		t.Errorf("user.Account = %q, want %q", user.Account, "myaccount")
	}
	if len(user.Roles) != 1 || user.Roles[0] != "myrole" {
		t.Errorf("user.Roles = %v, want [myrole]", user.Roles)
	}
}

func TestJwtAuthenticationProvider_DifferentRolesPathPerIssuer(t *testing.T) {
	privateKey1, publicKeyPEM1 := generateTestKeyPair(t)
	privateKey2, publicKeyPEM2 := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Issuers: map[string]IssuerConfig{
			"https://auth1.example.com": {
				PublicKey:      publicKeyPEM1,
				Accounts:       []string{"*"},
				RolesClaimPath: "realm_access.roles",
			},
			"https://auth2.example.com": {
				PublicKey:      publicKeyPEM2,
				Accounts:       []string{"*"},
				RolesClaimPath: "custom.path.to.roles",
			},
		},
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Test issuer 1 with realm_access.roles
	token1 := createTestJWT(t, privateKey1, jwt.MapClaims{
		"iss": "https://auth1.example.com",
		"sub": "user-1",
		"exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]any{
			"roles": []any{"account1.admin"},
		},
	})

	user1, err := provider.Verify(context.Background(), AuthRequest{Token: token1})
	if err != nil {
		t.Fatalf("Verify() issuer1 error = %v", err)
	}
	if user1.Account != "account1" || user1.Roles[0] != "admin" {
		t.Errorf("issuer1: got account=%q roles=%v, want account1/admin", user1.Account, user1.Roles)
	}

	// Test issuer 2 with custom.path.to.roles
	token2 := createTestJWT(t, privateKey2, jwt.MapClaims{
		"iss": "https://auth2.example.com",
		"sub": "user-2",
		"exp": time.Now().Add(time.Hour).Unix(),
		"custom": map[string]any{
			"path": map[string]any{
				"to": map[string]any{
					"roles": []any{"account2.viewer"},
				},
			},
		},
	})

	user2, err := provider.Verify(context.Background(), AuthRequest{Token: token2})
	if err != nil {
		t.Fatalf("Verify() issuer2 error = %v", err)
	}
	if user2.Account != "account2" || user2.Roles[0] != "viewer" {
		t.Errorf("issuer2: got account=%q roles=%v, want account2/viewer", user2.Account, user2.Roles)
	}
}

func TestMatchAccountPattern(t *testing.T) {
	tests := []struct {
		pattern string
		account string
		want    bool
	}{
		// Exact match
		{"dev", "dev", true},
		{"dev", "prod", false},

		// Wildcard all
		{"*", "anything", true},
		{"*", "dev", true},

		// Prefix wildcard
		{"tenant-a-*", "tenant-a-acc-1", true},
		{"tenant-a-*", "tenant-a-", true},
		{"tenant-a-*", "tenant-b-acc-1", false},
		{"tenant-*", "tenant-a", true},
		{"tenant-*", "tenant-", true},
		{"tenant-*", "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.account, func(t *testing.T) {
			got := matchAccountPattern(tt.pattern, tt.account)
			if got != tt.want {
				t.Errorf("matchAccountPattern(%q, %q) = %v, want %v", tt.pattern, tt.account, got, tt.want)
			}
		})
	}
}

func TestParseAccountRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  []accountRole
	}{
		{
			name:  "valid roles",
			roles: []string{"account.admin", "account.viewer"},
			want: []accountRole{
				{account: "account", role: "admin"},
				{account: "account", role: "viewer"},
			},
		},
		{
			name:  "mixed valid and invalid",
			roles: []string{"account.admin", "invalid", "account.viewer", ".norole", "noaccount."},
			want: []accountRole{
				{account: "account", role: "admin"},
				{account: "account", role: "viewer"},
			},
		},
		{
			name:  "all invalid",
			roles: []string{"invalid", "noDot", ".empty", "empty."},
			want:  nil,
		},
		{
			name:  "role with multiple dots",
			roles: []string{"account.role.with.dots"},
			want: []accountRole{
				{account: "account", role: "role.with.dots"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAccountRoles(tt.roles)
			if len(got) != len(tt.want) {
				t.Errorf("parseAccountRoles() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseAccountRoles()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
