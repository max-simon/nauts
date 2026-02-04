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
		"entity": {
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
		"nauts": {
			"type": "file",
			"file": {
				"policiesPath": "/path/to/policies.json",
				"groupsPath": "/path/to/groups.json"
			}
		},
		"identity": {
			"type": "file",
			"file": {
				"usersPath": "/path/to/users.json"
			}
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

	// Verify entity config
	if config.Entity.Type != "operator" {
		t.Errorf("Entity.Type = %q, want %q", config.Entity.Type, "operator")
	}
	if len(config.Entity.Operator.Accounts) != 2 {
		t.Errorf("Entity.Operator.Accounts count = %d, want 2", len(config.Entity.Operator.Accounts))
	}
	authAcc, ok := config.Entity.Operator.Accounts["AUTH"]
	if !ok {
		t.Error("Entity.Operator.Accounts[AUTH] not found")
	} else {
		if authAcc.PublicKey != "AAUTH1234567890123456789012345678901234567890123456789012345" {
			t.Errorf("Entity.Operator.Accounts[AUTH].PublicKey = %q, want correct value", authAcc.PublicKey)
		}
		if authAcc.SigningKeyPath != "/path/to/auth-signing.nk" {
			t.Errorf("Entity.Operator.Accounts[AUTH].SigningKeyPath = %q, want %q", authAcc.SigningKeyPath, "/path/to/auth-signing.nk")
		}
	}

	// Verify nauts config
	if config.Nauts.Type != "file" {
		t.Errorf("Nauts.Type = %q, want %q", config.Nauts.Type, "file")
	}
	if config.Nauts.File.PoliciesPath != "/path/to/policies.json" {
		t.Errorf("Nauts.File.PoliciesPath = %q, want %q", config.Nauts.File.PoliciesPath, "/path/to/policies.json")
	}

	// Verify identity config
	if config.Identity.Type != "file" {
		t.Errorf("Identity.Type = %q, want %q", config.Identity.Type, "file")
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
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Nauts: NautsConfig{
					File: &FileNautsConfig{
						PoliciesPath: "/path/to/policies.json",
						GroupsPath:   "/path/to/groups.json",
					},
				},
				Identity: IdentityConfig{
					File: &FileIdentityConfig{
						UsersPath: "/path/to/users.json",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid static config",
			config: Config{
				Entity: EntityConfig{
					Type: "static",
					Static: &StaticEntityConfig{
						PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
						PrivateKeyPath: "/path/to/account.nk",
						Accounts:       []string{"AUTH"},
					},
				},
				Nauts: NautsConfig{
					File: &FileNautsConfig{
						PoliciesPath: "/path/to/policies.json",
						GroupsPath:   "/path/to/groups.json",
					},
				},
				Identity: IdentityConfig{
					File: &FileIdentityConfig{
						UsersPath: "/path/to/users.json",
					},
				},
			},
			wantErr: "",
		},
		{
			name: "missing operator accounts",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{},
					},
				},
			},
			wantErr: "entity.operator.accounts must contain at least one account",
		},
		{
			name: "missing operator account publicKey",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
			},
			wantErr: "entity.operator.accounts[AUTH].publicKey is required",
		},
		{
			name: "missing operator account signingKeyPath",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey: "AAUTH1234567890123456789012345678901234567890123456789012345",
							},
						},
					},
				},
			},
			wantErr: "entity.operator.accounts[AUTH].signingKeyPath is required",
		},
		{
			name: "missing operator config",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
				},
			},
			wantErr: "entity.operator configuration is required",
		},
		{
			name: "unsupported entity type",
			config: Config{
				Entity: EntityConfig{
					Type: "unknown",
				},
			},
			wantErr: "unsupported entity provider type",
		},
		{
			name: "missing policies path",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Nauts: NautsConfig{
					Type: "file",
					File: &FileNautsConfig{
						GroupsPath: "/path/to/groups.json",
					},
				},
			},
			wantErr: "nauts.file.policiesPath is required",
		},
		{
			name: "missing users path",
			config: Config{
				Entity: EntityConfig{
					Type: "operator",
					Operator: &OperatorEntityConfig{
						Accounts: map[string]AccountSigningConfig{
							"AUTH": {
								PublicKey:      "AAUTH1234567890123456789012345678901234567890123456789012345",
								SigningKeyPath: "/path/to/auth-signing.nk",
							},
						},
					},
				},
				Nauts: NautsConfig{
					File: &FileNautsConfig{
						PoliciesPath: "/path/to/policies.json",
						GroupsPath:   "/path/to/groups.json",
					},
				},
				Identity: IdentityConfig{
					Type: "file",
					File: &FileIdentityConfig{},
				},
			},
			wantErr: "identity.file.usersPath is required",
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
