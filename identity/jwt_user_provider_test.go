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
// Returns the private key and base64-encoded PEM public key (as expected by JwtAuthenticationProvider).
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

	// The JWT provider expects base64-encoded PEM
	return privateKey, base64.StdEncoding.EncodeToString(pubKeyPEM)
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
		ID:        "test-okta",
		Accounts:  []string{"tenant-a-*", "dev"},
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

	user, err := provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if user.ID != "user-123" {
		t.Errorf("user.ID = %q, want %q", user.ID, "user-123")
	}

	// Check roles as AccountRole objects
	if len(user.Roles) != 2 {
		t.Fatalf("len(user.Roles) = %d, want 2", len(user.Roles))
	}

	// Check both roles are present
	hasAdmin := false
	hasViewer := false
	for _, r := range user.Roles {
		if r.Account == "tenant-a-acc" && r.Role == "admin" {
			hasAdmin = true
		}
		if r.Account == "tenant-a-acc" && r.Role == "viewer" {
			hasViewer = true
		}
	}
	if !hasAdmin {
		t.Error("expected role 'tenant-a-acc.admin' not found")
	}
	if !hasViewer {
		t.Error("expected role 'tenant-a-acc.viewer' not found")
	}

	// Check attributes
	if user.Attributes["email"] != "user@example.com" {
		t.Errorf("user.Attributes[email] = %q, want %q", user.Attributes["email"], "user@example.com")
	}
	if user.Attributes["name"] != "Test User" {
		t.Errorf("user.Attributes[name] = %q, want %q", user.Attributes["name"], "Test User")
	}
}

func TestJwtAuthenticationProvider_Verify_MultipleAccounts(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
		Accounts:  []string{"*"}, // Allow all accounts
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
				"roles": []any{"account-a.admin", "account-b.viewer", "account-a.viewer"},
			},
		},
	})

	user, err := provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	// Should return all roles from all accounts
	if len(user.Roles) != 3 {
		t.Fatalf("len(user.Roles) = %d, want 3", len(user.Roles))
	}

	// Verify role distribution
	accountA := 0
	accountB := 0
	for _, r := range user.Roles {
		if r.Account == "account-a" {
			accountA++
		}
		if r.Account == "account-b" {
			accountB++
		}
	}
	if accountA != 2 {
		t.Errorf("account-a roles = %d, want 2", accountA)
	}
	if accountB != 1 {
		t.Errorf("account-b roles = %d, want 1", accountB)
	}
}

func TestJwtAuthenticationProvider_Verify_WrongIssuer(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// JWT from different issuer
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://other-issuer.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err == nil {
		t.Fatal("Verify() expected error for wrong issuer")
	}
	// Check the error message contains expected text
	if !errors.Is(err, ErrIssuerNotConfigured) {
		t.Errorf("Verify() error = %v, want ErrIssuerNotConfigured", err)
	}
}

func TestJwtAuthenticationProvider_Verify_NoRoles(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
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
	if err == nil {
		t.Fatal("Verify() expected error when no roles present")
	}
	if !errors.Is(err, ErrNoRolesFound) {
		t.Errorf("Verify() error = %v, want ErrNoRolesFound", err)
	}
}

func TestJwtAuthenticationProvider_Verify_InvalidRoleFormat(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Roles without proper format (no dot separator)
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
	if err == nil {
		t.Fatal("Verify() expected error when all roles are invalid")
	}
	if !errors.Is(err, ErrNoRolesFound) {
		t.Errorf("Verify() error = %v, want ErrNoRolesFound", err)
	}
}

func TestJwtAuthenticationProvider_Verify_MixedValidInvalidRoles(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Mix of valid and invalid role formats
	tokenString := createTestJWT(t, privateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin", "invalid-role", "account-b.viewer"},
			},
		},
	})

	user, err := provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	// Should only include valid roles
	if len(user.Roles) != 2 {
		t.Fatalf("len(user.Roles) = %d, want 2", len(user.Roles))
	}

	// Check valid roles are present
	hasAdminA := false
	hasViewerB := false
	for _, r := range user.Roles {
		if r.Account == "account-a" && r.Role == "admin" {
			hasAdminA = true
		}
		if r.Account == "account-b" && r.Role == "viewer" {
			hasViewerB = true
		}
	}
	if !hasAdminA {
		t.Error("expected role 'account-a.admin' not found")
	}
	if !hasViewerB {
		t.Error("expected role 'account-b.viewer' not found")
	}
}

func TestJwtAuthenticationProvider_Verify_ExpiredToken(t *testing.T) {
	privateKey, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
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
		"exp": time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err == nil {
		t.Fatal("Verify() expected error for expired token")
	}
}

func TestJwtAuthenticationProvider_Verify_InvalidSignature(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)
	differentPrivateKey, _ := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "test-okta",
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	// Token signed with different key
	tokenString := createTestJWT(t, differentPrivateKey, jwt.MapClaims{
		"iss": "https://auth.example.com",
		"sub": "user-123",
		"exp": time.Now().Add(time.Hour).Unix(),
		"resource_access": map[string]any{
			"nauts": map[string]any{
				"roles": []any{"account-a.admin"},
			},
		},
	})

	_, err = provider.Verify(context.Background(), AuthRequest{Token: tokenString})
	if err == nil {
		t.Fatal("Verify() expected error for invalid signature")
	}
}

func TestJwtAuthenticationProvider_CanManageAccount(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)

	tests := []struct {
		name           string
		configAccounts []string
		testAccount    string
		want           bool
	}{
		{
			name:           "wildcard matches all",
			configAccounts: []string{"*"},
			testAccount:    "anything",
			want:           true,
		},
		{
			name:           "exact match",
			configAccounts: []string{"APP", "DEV"},
			testAccount:    "APP",
			want:           true,
		},
		{
			name:           "no match",
			configAccounts: []string{"APP", "DEV"},
			testAccount:    "PROD",
			want:           false,
		},
		{
			name:           "pattern match",
			configAccounts: []string{"PROD*"},
			testAccount:    "PROD-US",
			want:           true,
		},
		{
			name:           "pattern no match",
			configAccounts: []string{"PROD*"},
			testAccount:    "DEV-US",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
				ID:        "test",
				Accounts:  tt.configAccounts,
				Issuer:    "https://auth.example.com",
				PublicKey: publicKeyPEM,
			})
			if err != nil {
				t.Fatalf("creating provider: %v", err)
			}

			got := provider.CanManageAccount(tt.testAccount)
			if got != tt.want {
				t.Errorf("CanManageAccount(%q) = %v, want %v", tt.testAccount, got, tt.want)
			}
		})
	}
}

func TestJwtAuthenticationProvider_ID(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)

	provider, err := NewJwtAuthenticationProvider(JwtAuthenticationProviderConfig{
		ID:        "my-okta-provider",
		Accounts:  []string{"*"},
		Issuer:    "https://auth.example.com",
		PublicKey: publicKeyPEM,
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	if provider.ID() != "my-okta-provider" {
		t.Errorf("ID() = %q, want %q", provider.ID(), "my-okta-provider")
	}
}

func TestNewJwtAuthenticationProvider_InvalidConfig(t *testing.T) {
	_, publicKeyPEM := generateTestKeyPair(t)

	tests := []struct {
		name    string
		config  JwtAuthenticationProviderConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: JwtAuthenticationProviderConfig{
				ID:        "test",
				Accounts:  []string{"*"},
				Issuer:    "https://auth.example.com",
				PublicKey: publicKeyPEM,
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			config: JwtAuthenticationProviderConfig{
				Accounts:  []string{"*"},
				Issuer:    "https://auth.example.com",
				PublicKey: publicKeyPEM,
			},
			wantErr: true,
		},
		{
			name: "missing accounts",
			config: JwtAuthenticationProviderConfig{
				ID:        "test",
				Issuer:    "https://auth.example.com",
				PublicKey: publicKeyPEM,
			},
			wantErr: true,
		},
		{
			name: "missing issuer",
			config: JwtAuthenticationProviderConfig{
				ID:        "test",
				Accounts:  []string{"*"},
				PublicKey: publicKeyPEM,
			},
			wantErr: true,
		},
		{
			name: "missing public key",
			config: JwtAuthenticationProviderConfig{
				ID:       "test",
				Accounts: []string{"*"},
				Issuer:   "https://auth.example.com",
			},
			wantErr: true,
		},
		{
			name: "invalid public key",
			config: JwtAuthenticationProviderConfig{
				ID:        "test",
				Accounts:  []string{"*"},
				Issuer:    "https://auth.example.com",
				PublicKey: "invalid-key",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewJwtAuthenticationProvider(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewJwtAuthenticationProvider() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
