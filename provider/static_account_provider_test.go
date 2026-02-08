package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewStaticAccountProvider(t *testing.T) {
	// Create a temp directory for test key files
	tmpDir := t.TempDir()

	// Create a valid account nkey seed file
	// This is a test account seed (SA prefix)
	accountSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	accountKeyPath := filepath.Join(tmpDir, "account.nk")
	if err := os.WriteFile(accountKeyPath, []byte(accountSeed), 0600); err != nil {
		t.Fatalf("failed to write account key: %v", err)
	}

	tests := []struct {
		name    string
		cfg     StaticAccountProviderConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				PrivateKeyPath: accountKeyPath,
				Accounts:       []string{"test-account"},
			},
			wantErr: false,
		},
		{
			name: "empty accounts",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				PrivateKeyPath: accountKeyPath,
				Accounts:       []string{},
			},
			wantErr: true,
			errMsg:  "at least one account is required",
		},
		{
			name: "empty account name",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				PrivateKeyPath: accountKeyPath,
				Accounts:       []string{""},
			},
			wantErr: true,
			errMsg:  "account name cannot be empty",
		},
		{
			name: "missing public key",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "",
				PrivateKeyPath: accountKeyPath,
				Accounts:       []string{"test-account"},
			},
			wantErr: true,
			errMsg:  "publicKey is required",
		},
		{
			name: "missing private key path",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				PrivateKeyPath: "",
				Accounts:       []string{"test-account"},
			},
			wantErr: true,
			errMsg:  "privateKeyPath is required",
		},
		{
			name: "non-existent key file",
			cfg: StaticAccountProviderConfig{
				PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				PrivateKeyPath: "/nonexistent/path/key.nk",
				Accounts:       []string{"test-account"},
			},
			wantErr: true,
			errMsg:  "loading signer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewStaticAccountProvider(tt.cfg)
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

func TestStaticAccountProvider_GetAccount(t *testing.T) {
	tmpDir := t.TempDir()
	accountSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	accountKeyPath := filepath.Join(tmpDir, "account.nk")
	if err := os.WriteFile(accountKeyPath, []byte(accountSeed), 0600); err != nil {
		t.Fatalf("failed to write account key: %v", err)
	}

	provider, err := NewStaticAccountProvider(StaticAccountProviderConfig{
		PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		PrivateKeyPath: accountKeyPath,
		Accounts:       []string{"test-account"},
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Test getting existing account
	account, err := provider.GetAccount(ctx, "test-account")
	if err != nil {
		t.Fatalf("unexpected error getting account: %v", err)
	}
	if account.Name() != "test-account" {
		t.Errorf("expected account name 'test-account', got %q", account.Name())
	}
	if account.PublicKey() != "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" {
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

func TestStaticAccountProvider_ListAccounts(t *testing.T) {
	tmpDir := t.TempDir()
	accountSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	accountKeyPath := filepath.Join(tmpDir, "account.nk")
	if err := os.WriteFile(accountKeyPath, []byte(accountSeed), 0600); err != nil {
		t.Fatalf("failed to write account key: %v", err)
	}

	provider, err := NewStaticAccountProvider(StaticAccountProviderConfig{
		PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		PrivateKeyPath: accountKeyPath,
		Accounts:       []string{"account1", "account2"},
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

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
	if !names["account1"] || !names["account2"] {
		t.Errorf("expected accounts account1 and account2, got %v", names)
	}
}

func TestStaticAccountProvider_IsOperatorMode(t *testing.T) {
	tmpDir := t.TempDir()
	accountSeed := "SAANJIBNEKGCRUWJCPIWUXFBFJLR36FJTFKGBGKAT7AQXH2LVFNQWZJMQU"
	accountKeyPath := filepath.Join(tmpDir, "account.nk")
	if err := os.WriteFile(accountKeyPath, []byte(accountSeed), 0600); err != nil {
		t.Fatalf("failed to write account key: %v", err)
	}

	provider, err := NewStaticAccountProvider(StaticAccountProviderConfig{
		PublicKey:      "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
		PrivateKeyPath: accountKeyPath,
		Accounts:       []string{"test-account"},
	})
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	if provider.IsOperatorMode() {
		t.Error("expected IsOperatorMode() to return false")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
