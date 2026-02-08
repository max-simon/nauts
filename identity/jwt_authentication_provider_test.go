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
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
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

	user, err := provider.Verify(context.Background(), AuthRequest{Account: "tenant-a-acc", Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-123")
	}
	if len(user.Roles) != 2 {
		t.Errorf("len(user.Roles) = %d, want 2", len(user.Roles))
	}

	// Check roles include account
	roleSet := make(map[string]bool)
	for _, r := range user.Roles {
		if r.Account != "tenant-a-acc" {
			t.Errorf("role account = %q, want %q", r.Account, "tenant-a-acc")
		}
		roleSet[r.Role] = true
	}
	if !roleSet["admin"] {
		t.Error("expected role 'admin' not found")
	}
	if !roleSet["viewer"] {
		t.Error("expected role 'viewer' not found")
	}

	// Check attributes
	if user.Attributes["sub"] != "user-123" {
		t.Errorf("user.Attributes[sub] = %q, want %q", user.Attributes["sub"], "user-123")
	}
}

func TestJwtAuthenticationProvider_Verify_WithExplicitAccount(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
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

	// Should return all roles (filtering done by AuthController)
	if len(user.Roles) != 2 {
		t.Errorf("user.Roles length = %d, want 2", len(user.Roles))
	}
	// Verify both accounts are present
	accounts := make(map[string]string)
	for _, role := range user.Roles {
		accounts[role.Account] = role.Role
	}
	if accounts["account-a"] != "admin" || accounts["account-b"] != "viewer" {
		t.Errorf("user.Roles = %v, want account-a.admin and account-b.viewer", user.Roles)
	}
}

func TestJwtAuthenticationProvider_Verify_IssuerMismatch(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// JWT with wrong issuer
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://unknown-issuer.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Account: "any", Token: tokenString})
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("Verify() error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestJwtAuthenticationProvider_Verify_NoRoles(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
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
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
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
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
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
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM1,
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

func TestJwtAuthenticationProvider_CustomRolesPath(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		Accounts:       []string{"*"},
		Issuer:         "https://auth.example.com",
		PublicKey:      publicKeyPEM,
		RolesClaimPath: "realm_access.roles",
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

	user, err := provider.Verify(context.Background(), AuthRequest{Account: "myaccount", Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if len(user.Roles) != 1 || user.Roles[0].Account != "myaccount" || user.Roles[0].Role != "myrole" {
		t.Errorf("user.Roles = %v, want [{myaccount myrole}]", user.Roles)
	}
}

func TestParseJWTAccountRoles(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  []AccountRole
	}{
		{
			name:  "valid roles",
			roles: []string{"account.admin", "account.viewer"},
			want: []AccountRole{
				{Account: "account", Role: "admin"},
				{Account: "account", Role: "viewer"},
			},
		},
		{
			name:  "mixed valid and invalid",
			roles: []string{"account.admin", "invalid", "account.viewer", ".norole", "noaccount."},
			want: []AccountRole{
				{Account: "account", Role: "admin"},
				{Account: "account", Role: "viewer"},
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
			want: []AccountRole{
				{Account: "account", Role: "role.with.dots"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseJWTAccountRoles(tt.roles)
			if len(got) != len(tt.want) {
				t.Errorf("parseJWTAccountRoles() = %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("parseJWTAccountRoles()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
