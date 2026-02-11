package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/msimon/nauts/provider"
)

func TestLoadConfig(t *testing.T) {
	// Create a temp config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	configJSON := `{
		"account": {
			"type": "operator",
			"operator": {
				"accounts": {
					"AUTH": {
						"publicKey": "AAUTH1234567890123456789012345678901234567890123456789012345",
						"signingKeyPath": "/path/to/auth-signing.nk"
					},
					"APP": {
						"publicKey": "AAPP12345678901234567890123456789012345678901234567890123456",
						"signingKeyPath": "/path/to/app-signing.nk"
					}
				}
			}
		},
		"policy": {
			"type": "file",
			"file": {
				"policiesPath": "/path/to/policies.json",
				"bindingsPath": "/path/to/bindings.json"
			}
		},
		"auth": {
			"file": [
				{
					"id": "local",
					"accounts": ["*"],
					"userPath": "/path/to/users.json"
				}
			]
		},
		"server": {
			"natsUrl": "nats://localhost:4222",
			"natsNkey": "/path/to/auth-service.nk",
			"ttl": "2h"
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify account config
	if config.Account.Type != "operator" {
		t.Errorf("Account.Type = %q, want %q", config.Account.Type, "operator")
	}
	if len(config.Account.Operator.Accounts) != 2 {
		t.Errorf("Account.Operator.Accounts count = %d, want 2", len(config.Account.Operator.Accounts))
	}
	authAcc, ok := config.Account.Operator.Accounts["AUTH"]
	if !ok {
		t.Error("Account.Operator.Accounts[AUTH] not found")
	} else {
		if authAcc.PublicKey != "AAUTH1234567890123456789012345678901234567890123456789012345" {
			t.Errorf("Account.Operator.Accounts[AUTH].PublicKey = %q, want correct value", authAcc.PublicKey)
		}
		if authAcc.SigningKeyPath != "/path/to/auth-signing.nk" {
			t.Errorf("Account.Operator.Accounts[AUTH].SigningKeyPath = %q, want %q", authAcc.SigningKeyPath, "/path/to/auth-signing.nk")
		}
	}

	// Verify policy config
	if config.Policy.Type != "file" {
		t.Errorf("Policy.Type = %q, want %q", config.Policy.Type, "file")
	}
	if config.Policy.File.PoliciesPath != "/path/to/policies.json" {
		t.Errorf("Policy.File.PoliciesPath = %q, want %q", config.Policy.File.PoliciesPath, "/path/to/policies.json")
	}
	if config.Policy.File.BindingsPath != "/path/to/bindings.json" {
		t.Errorf("Policy.File.BindingsPath = %q, want %q", config.Policy.File.BindingsPath, "/path/to/bindings.json")
	}

	// Verify auth providers config
	if len(config.Auth.File) != 1 {
		t.Errorf("Auth.File count = %d, want 1", len(config.Auth.File))
	}
	if config.Auth.File[0].ID != "local" {
		t.Errorf("Auth.File[0].ID = %q, want %q", config.Auth.File[0].ID, "local")
	}
	if config.Auth.File[0].UsersPath != "/path/to/users.json" {
		t.Errorf("Auth.File[0].UsersPath = %q, want %q", config.Auth.File[0].UsersPath, "/path/to/users.json")
	}
	if len(config.Auth.File[0].Accounts) != 1 || config.Auth.File[0].Accounts[0] != "*" {
		t.Errorf("Auth.File[0].Accounts = %v, want [\"*\"]", config.Auth.File[0].Accounts)
	}

	// Verify server config
	if config.Server.NatsURL != "nats://localhost:4222" {
		t.Errorf("Server.NatsURL = %q, want %q", config.Server.NatsURL, "nats://localhost:4222")
	}
	if config.Server.NatsNkey != "/path/to/auth-service.nk" {
		t.Errorf("Server.NatsNkey = %q, want %q", config.Server.NatsNkey, "/path/to/auth-service.nk")
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("writing config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "valid operator config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "",
		},
		{
			name: "valid jwt auth provider config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					JWT: []JwtAuthProviderConfig{{
						ID:        "jwt",
						Accounts:  []string{"*"},
						Issuer:    "https://auth.example.com",
						PublicKey: "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0tCg==",
					}},
				},
			},
			wantErr: "",
		},
		{
			name: "valid static config",
			config: Config{
				Account: AccountConfig{
					Type: "static",
					Static: &provider.StaticAccountProviderConfig{
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						PrivateKeyPath: "/path/to/account.nk",
						Accounts:       []string{"AUTH"},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "",
		},
		{
			name: "missing operator accounts",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{},
					},
				},
			},
			wantErr: "account.operator.accounts must contain at least one account",
		},
		{
			name: "missing operator account publicKey",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
			},
			wantErr: "account.operator.accounts[AUTH].publicKey is required",
		},
		{
			name: "missing operator account signingKeyPath",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey: "AAUTH1234567890123456789012345678901234567890123456789012345",
							},
						},
					},
				},
			},
			wantErr: "account.operator.accounts[AUTH].signingKeyPath is required",
		},
		{
			name: "missing operator config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
				},
			},
			wantErr: "account.operator configuration is required",
		},
		{
			name: "unsupported account type",
			config: Config{
				Account: AccountConfig{
					Type: "unknown",
				},
			},
			wantErr: "unsupported account provider type",
		},
		{
			name: "missing policy bindings path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{File: &provider.FilePolicyProviderConfig{PoliciesPath: "/path/to/policies.json"}},
				Auth:   AuthConfig{File: []FileAuthProviderConfig{{ID: "local", UsersPath: "/path/to/users.json", Accounts: []string{"*"}}}},
			},
			wantErr: "policy.file.bindingsPath is required",
		},
		{
			name: "missing policy policies path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "file",
					File: &provider.FilePolicyProviderConfig{},
				},
				Auth: AuthConfig{File: []FileAuthProviderConfig{{ID: "local", UsersPath: "/path/to/users.json", Accounts: []string{"*"}}}},
			},
			wantErr: "policy.file.policiesPath is required",
		},
		{
			name: "missing auth providers",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
			},
			wantErr: "auth must contain at least one authentication provider",
		},
		{
			name: "missing file user path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "auth.file[local].userPath is required",
		},
		{
			name: "missing jwt issuer",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					JWT: []JwtAuthProviderConfig{{
						ID:        "jwt",
						Accounts:  []string{"*"},
						Issuer:    "",
						PublicKey: "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0tCg==",
					}},
				},
			},
			wantErr: "auth.jwt[jwt].issuer is required",
		},
		{
			name: "missing jwt public key",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					JWT: []JwtAuthProviderConfig{{
						ID:        "jwt",
						Accounts:  []string{"*"},
						Issuer:    "https://auth.example.com",
						PublicKey: "",
					}},
				},
			},
			wantErr: "auth.jwt[jwt].publicKey is required",
		},
		{
			name: "valid nats policy config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "nats",
					Nats: &provider.NatsPolicyProviderConfig{
						Bucket:  "nauts-policies",
						NatsURL: "nats://localhost:4222",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "",
		},
		{
			name: "nats policy missing bucket",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "nats",
					Nats: &provider.NatsPolicyProviderConfig{
						NatsURL: "nats://localhost:4222",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "policy.nats.bucket is required",
		},
		{
			name: "nats policy missing natsUrl",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "nats",
					Nats: &provider.NatsPolicyProviderConfig{
						Bucket: "nauts-policies",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "policy.nats.natsUrl is required",
		},
		{
			name: "nats policy mutually exclusive credentials",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "nats",
					Nats: &provider.NatsPolicyProviderConfig{
						Bucket:          "nauts-policies",
						NatsURL:         "nats://localhost:4222",
						NatsCredentials: "/path/to/creds",
						NatsNkey:        "/path/to/nkey",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "policy.nats.natsCredentials and policy.nats.natsNkey are mutually exclusive",
		},
		{
			name: "nats policy missing nats config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					Type: "nats",
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "local",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
				},
			},
			wantErr: "policy.nats configuration is required when type is 'nats'",
		},
		{
			name: "duplicate auth provider ids",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &provider.OperatorAccountProviderConfig{
						Accounts: map[string]provider.AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Policy: PolicyConfig{
					File: &provider.FilePolicyProviderConfig{
						PoliciesPath: "/path/to/policies.json",
						BindingsPath: "/path/to/bindings.json",
					},
				},
				Auth: AuthConfig{
					File: []FileAuthProviderConfig{{
						ID:        "dup",
						UsersPath: "/path/to/users.json",
						Accounts:  []string{"*"},
					}},
					JWT: []JwtAuthProviderConfig{{
						ID:        "dup",
						Accounts:  []string{"*"},
						Issuer:    "https://auth.example.com",
						PublicKey: "LS0tLS1CRUdJTiBQVUJMSUMgS0VZLS0tLS0tCg==",
					}},
				},
			},
			wantErr: "auth providers contain duplicate id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("Validate() error = nil, want containing %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("Validate() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestServerConfig_GetTTL(t *testing.T) {
	tests := []struct {
		name       string
		ttl        string
		defaultTTL time.Duration
		want       time.Duration
	}{
		{
			name:       "valid duration",
			ttl:        "2h",
			defaultTTL: time.Hour,
			want:       2 * time.Hour,
		},
		{
			name:       "empty uses default",
			ttl:        "",
			defaultTTL: time.Hour,
			want:       time.Hour,
		},
		{
			name:       "invalid uses default",
			ttl:        "invalid",
			defaultTTL: time.Hour,
			want:       time.Hour,
		},
		{
			name:       "minutes",
			ttl:        "30m",
			defaultTTL: time.Hour,
			want:       30 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &ServerConfig{TTL: tt.ttl}
			got := c.GetTTL(tt.defaultTTL)
			if got != tt.want {
				t.Errorf("GetTTL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestServerConfig_GetXKeySeed(t *testing.T) {
	t.Run("from file", func(t *testing.T) {
		dir := t.TempDir()
		seedFile := filepath.Join(dir, "xkey.seed")
		if err := os.WriteFile(seedFile, []byte("file-seed\n"), 0644); err != nil {
			t.Fatalf("writing seed file: %v", err)
		}

		c := &ServerConfig{XKeySeedFile: seedFile}
		got, err := c.GetXKeySeed()
		if err != nil {
			t.Fatalf("GetXKeySeed() error = %v", err)
		}
		if got != "file-seed" {
			t.Errorf("GetXKeySeed() = %q, want %q", got, "file-seed")
		}
	})

	t.Run("empty when not set", func(t *testing.T) {
		c := &ServerConfig{}
		got, err := c.GetXKeySeed()
		if err != nil {
			t.Fatalf("GetXKeySeed() error = %v", err)
		}
		if got != "" {
			t.Errorf("GetXKeySeed() = %q, want empty", got)
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		c := &ServerConfig{XKeySeedFile: "/nonexistent/file"}
		_, err := c.GetXKeySeed()
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

func TestServerConfig_ToCalloutConfig(t *testing.T) {
	dir := t.TempDir()
	seedFile := filepath.Join(dir, "xkey.seed")
	if err := os.WriteFile(seedFile, []byte("xkey-seed"), 0644); err != nil {
		t.Fatalf("writing seed file: %v", err)
	}

	c := &ServerConfig{
		NatsURL:      "nats://localhost:4222",
		NatsNkey:     "/path/to/auth-service.nk",
		XKeySeedFile: seedFile,
		TTL:          "2h",
	}

	got, err := c.ToCalloutConfig()
	if err != nil {
		t.Fatalf("ToCalloutConfig() error = %v", err)
	}

	if got.NatsURL != "nats://localhost:4222" {
		t.Errorf("NatsURL = %q, want %q", got.NatsURL, "nats://localhost:4222")
	}
	if got.NatsNkey != "/path/to/auth-service.nk" {
		t.Errorf("NatsNkey = %q, want %q", got.NatsNkey, "/path/to/auth-service.nk")
	}
	if got.XKeySeed != "xkey-seed" {
		t.Errorf("XKeySeed = %q, want %q", got.XKeySeed, "xkey-seed")
	}
	if got.DefaultTTL != 2*time.Hour {
		t.Errorf("DefaultTTL = %v, want %v", got.DefaultTTL, 2*time.Hour)
	}
}
