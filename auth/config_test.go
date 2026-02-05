package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		"role": {
			"type": "file",
			"file": {
				"path": "/path/to/roles.json"
			}
		},
		"policy": {
			"type": "file",
			"file": {
				"path": "/path/to/policies.json"
			}
		},
		"authentication": {
			"file": [
				{
					"id": "local",
					"accounts": ["*"],
					"usersPath": "/path/to/users.json"
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

	// Verify role config
	if config.Role.Type != "file" {
		t.Errorf("Role.Type = %q, want %q", config.Role.Type, "file")
	}
	if config.Role.File.Path != "/path/to/roles.json" {
		t.Errorf("Role.File.Path = %q, want %q", config.Role.File.Path, "/path/to/roles.json")
	}

	// Verify policy config
	if config.Policy.Type != "file" {
		t.Errorf("Policy.Type = %q, want %q", config.Policy.Type, "file")
	}
	if config.Policy.File.Path != "/path/to/policies.json" {
		t.Errorf("Policy.File.Path = %q, want %q", config.Policy.File.Path, "/path/to/policies.json")
	}

	// Verify authentication config
	if len(config.Authentication.File) != 1 {
		t.Fatalf("Authentication.File count = %d, want 1", len(config.Authentication.File))
	}
	if config.Authentication.File[0].ID != "local" {
		t.Errorf("Authentication.File[0].ID = %q, want %q", config.Authentication.File[0].ID, "local")
	}
	if config.Authentication.File[0].UsersPath != "/path/to/users.json" {
		t.Errorf("Authentication.File[0].UsersPath = %q, want %q", config.Authentication.File[0].UsersPath, "/path/to/users.json")
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
			name: "valid file authentication config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:        "local",
							Accounts:  []string{"*"},
							UsersPath: "/path/to/users.json",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid jwt authentication config",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					JWT: []JwtAuthenticationConfig{
						{
							ID:        "okta",
							Accounts:  []string{"*"},
							Issuer:    "https://auth.example.com",
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid multiple authentication providers",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:        "local",
							Accounts:  []string{"DEV", "TEST"},
							UsersPath: "/path/to/users.json",
						},
					},
					JWT: []JwtAuthenticationConfig{
						{
							ID:        "okta",
							Accounts:  []string{"PROD*"},
							Issuer:    "https://auth.example.com",
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid static config",
			config: Config{
				Account: AccountConfig{
					Type: "static",
					Static: &StaticAccountConfig{
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						PrivateKeyPath: "/path/to/account.nk",
						Accounts:       []string{"AUTH"},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:        "local",
							Accounts:  []string{"*"},
							UsersPath: "/path/to/users.json",
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "missing operator accounts",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{},
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
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
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
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
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
			name: "missing role path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					Type: "file",
					File: &FileRoleConfig{},
				},
			},
			wantErr: "role.file.path is required",
		},
		{
			name: "missing policy path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					Type: "file",
					File: &FilePolicyConfig{},
				},
			},
			wantErr: "policy.file.path is required",
		},
		{
			name: "missing file authentication provider id",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							Accounts:  []string{"*"},
							UsersPath: "/path/to/users.json",
						},
					},
				},
			},
			wantErr: "authentication.file[0].id is required",
		},
		{
			name: "missing file authentication provider accounts",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:        "local",
							UsersPath: "/path/to/users.json",
						},
					},
				},
			},
			wantErr: "authentication.file[0].accounts must contain at least one account",
		},
		{
			name: "missing file authentication users path",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:       "local",
							Accounts: []string{"*"},
						},
					},
				},
			},
			wantErr: "authentication.file[0].usersPath is required",
		},
		{
			name: "missing jwt authentication provider id",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					JWT: []JwtAuthenticationConfig{
						{
							Accounts:  []string{"*"},
							Issuer:    "https://auth.example.com",
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "authentication.jwt[0].id is required",
		},
		{
			name: "missing jwt authentication provider accounts",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					JWT: []JwtAuthenticationConfig{
						{
							ID:        "okta",
							Issuer:    "https://auth.example.com",
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "authentication.jwt[0].accounts must contain at least one account",
		},
		{
			name: "missing jwt issuer",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					JWT: []JwtAuthenticationConfig{
						{
							ID:        "okta",
							Accounts:  []string{"*"},
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "authentication.jwt[0].issuer is required",
		},
		{
			name: "missing jwt public key",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					JWT: []JwtAuthenticationConfig{
						{
							ID:       "okta",
							Accounts: []string{"*"},
							Issuer:   "https://auth.example.com",
						},
					},
				},
			},
			wantErr: "authentication.jwt[0].publicKey is required",
		},
		{
			name: "duplicate provider ids",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{
					File: []FileAuthenticationConfig{
						{
							ID:        "duplicate",
							Accounts:  []string{"*"},
							UsersPath: "/path/to/users1.json",
						},
					},
					JWT: []JwtAuthenticationConfig{
						{
							ID:        "duplicate",
							Accounts:  []string{"*"},
							Issuer:    "https://auth.example.com",
							PublicKey: "-----BEGIN PUBLIC KEY-----\nMIIBIjANBg...\n-----END PUBLIC KEY-----",
						},
					},
				},
			},
			wantErr: "duplicate provider ID",
		},
		{
			name: "no authentication providers",
			config: Config{
				Account: AccountConfig{
					Type: "operator",
					Operator: &OperatorAccountConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Role: RoleConfig{
					File: &FileRoleConfig{
						Path: "/path/to/roles.json",
					},
				},
				Policy: PolicyConfig{
					File: &FilePolicyConfig{
						Path: "/path/to/policies.json",
					},
				},
				Authentication: AuthenticationConfig{},
			},
			wantErr: "at least one authentication provider must be configured",
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
