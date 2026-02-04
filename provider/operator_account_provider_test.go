package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewOperatorAccountProvider(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid account signing key files
	// These are test account seeds (SA prefix)
	// Use the same valid account seed for both - the seed itself is valid,
	// and we just need two separate account entries for testing
	authSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	appSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"

	authKeyPath := filepath.Join(tmpDir, "auth-signing.nk")
	appKeyPath := filepath.Join(tmpDir, "app-signing.nk")

	if err := os.WriteFile(authKeyPath, []byte(authSeed), 0600); err != nil {
		t.Fatalf("failed to write auth key: %v", err)
	}
	if err := os.WriteFile(appKeyPath, []byte(appSeed), 0600); err != nil {
		t.Fatalf("failed to write app key: %v", err)
	}

	tests := []struct {
		name    string
		cfg     OperatorAccountProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration with multiple accounts",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{
					"AUTH": {
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						SigningKeyPath: authKeyPath,
					},
					"APP": {
						PublicKey:      "AAPP12345678901234567890123456789012345678901234567890123456",
						SigningKeyPath: appKeyPath,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid configuration with single account",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{
					"AUTH": {
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						SigningKeyPath: authKeyPath,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty accounts",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{},
			},
			wantErr: true,
			errMsg:  "at least one account is required",
		},
		{
			name: "missing public key",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{
					"AUTH": {
						PublicKey:      "",
						SigningKeyPath: authKeyPath,
					},
				},
			},
			wantErr: true,
			errMsg:  "publicKey is required",
		},
		{
			name: "missing signing key path",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{
					"AUTH": {
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						SigningKeyPath: "",
					},
				},
			},
			wantErr: true,
			errMsg:  "signingKeyPath is required",
		},
		{
			name: "non-existent key file",
			cfg: OperatorAccountProviderConfig{
				Accounts: map[string]AccountSigningConfig{
					"AUTH": {
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						SigningKeyPath: "/nonexistent/path/key.nk",
					},
				},
			},
			wantErr: true,
			errMsg:  "loading signer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewOperatorAccountProvider(tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if provider == nil {
				t.Fatal("expected provider to be non-nil")
			}
		})
	}
}

func TestOperatorAccountProvider_GetAccount(t *testing.T) {
	provider := createTestOperatorAccountProvider(t)

	ctx := context.Background()

	// Test getting existing account
	account, err := provider.GetAccount(ctx, "AUTH")
	if err != nil {
		t.Fatalf("unexpected error getting account: %v", err)
	}
	if account.Name() != "AUTH" {
		t.Errorf("expected account name 'AUTH', got %q", account.Name())
	}
	if account.PublicKey() != "AAUTH1234567890123456789012345678901234567890123456789012345" {
		t.Errorf("unexpected public key: %s", account.PublicKey())
	}
	if account.Signer() == nil {
		t.Error("expected signer to be non-nil")
	}

	// Test getting non-existent account
	_, err = provider.GetAccount(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error getting non-existent account")
	}
	if !contains(err.Error(), "account not found") {
		t.Errorf("expected account not found error, got: %v", err)
	}
}

func TestOperatorAccountProvider_ListAccounts(t *testing.T) {
	provider := createTestOperatorAccountProvider(t)

	ctx := context.Background()
	accounts, err := provider.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("unexpected error listing accounts: %v", err)
	}
	if len(accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accounts))
	}

	// Check that both accounts are present
	names := make(map[string]bool)
	for _, acct := range accounts {
		names[acct.Name()] = true
	}
	if !names["AUTH"] || !names["APP"] {
		t.Errorf("expected accounts AUTH and APP, got %v", names)
	}
}

func TestOperatorAccountProvider_IsOperatorMode(t *testing.T) {
	provider := createTestOperatorAccountProvider(t)

	if !provider.IsOperatorMode() {
		t.Error("expected IsOperatorMode() to return true")
	}
}

func createTestOperatorAccountProvider(t *testing.T) *OperatorAccountProvider {
	t.Helper()

	tmpDir := t.TempDir()

	// Use the same valid account seed for both - the seed itself is valid,
	// and we just need two separate account entries for testing
	authSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	appSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"

	authKeyPath := filepath.Join(tmpDir, "auth-signing.nk")
	appKeyPath := filepath.Join(tmpDir, "app-signing.nk")

	if err := os.WriteFile(authKeyPath, []byte(authSeed), 0600); err != nil {
		t.Fatalf("failed to write auth key: %v", err)
	}
	if err := os.WriteFile(appKeyPath, []byte(appSeed), 0600); err != nil {
		t.Fatalf("failed to write app key: %v", err)
	}

	provider, err := NewOperatorAccountProvider(OperatorAccountProviderConfig{
		Accounts: map[string]AccountSigningConfig{
			"AUTH": {
				PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
				SigningKeyPath: authKeyPath,
			},
			"APP": {
				PublicKey:      "AAPP12345678901234567890123456789012345678901234567890123456",
				SigningKeyPath: appKeyPath,
			},
		},
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	return provider
}
