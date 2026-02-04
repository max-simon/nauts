package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/provider"
)

// Config holds the complete configuration for the nauts authentication service.
type Config struct {
	// Entity provider configuration
	Entity EntityConfig `json:"entity"`

	// Nauts provider configuration (policies and groups)
	Nauts NautsConfig `json:"nauts"`

	// Identity provider configuration
	Identity IdentityConfig `json:"identity"`

	// Server configuration (for serve mode)
	Server ServerConfig `json:"server"`
}

// EntityConfig configures the entity provider.
type EntityConfig struct {
	// Type specifies the entity provider type: "operator" or "static".
	Type string `json:"type"`

	// Operator contains operator mode configuration.
	Operator *OperatorEntityConfig `json:"operator,omitempty"`

	// Static contains static entity provider configuration.
	Static *StaticEntityConfig `json:"static,omitempty"`
}

// OperatorEntityConfig configures the operator entity provider.
// In operator mode, the auth service runs in the AUTH account but authenticates
// users across all accounts using account signing keys.
type OperatorEntityConfig struct {
	// Accounts maps account names to their signing configuration.
	Accounts map[string]AccountSigningConfig `json:"accounts"`
}

// AccountSigningConfig holds the signing configuration for an account.
type AccountSigningConfig struct {
	// PublicKey is the account's public key (starts with 'A').
	PublicKey string `json:"publicKey"`

	// SigningKeyPath is the path to the account signing key file (.nk file).
	SigningKeyPath string `json:"signingKeyPath"`
}

// StaticEntityConfig configures the static entity provider.
type StaticEntityConfig struct {
	// PublicKey is the public key used for all accounts.
	PublicKey string `json:"publicKey"`

	// PrivateKeyPath is the path to the nkey seed file used for all accounts.
	PrivateKeyPath string `json:"privateKeyPath"`

	// Accounts is the list of account names.
	Accounts []string `json:"accounts"`
}

// NautsConfig configures the nauts provider (policies and groups).
type NautsConfig struct {
	// Type specifies the nauts provider type. Currently only "file" is supported.
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *FileNautsConfig `json:"file,omitempty"`
}

// FileNautsConfig configures the file-based nauts provider.
type FileNautsConfig struct {
	// PoliciesPath is the path to the policies JSON file.
	PoliciesPath string `json:"policiesPath"`

	// GroupsPath is the path to the groups JSON file.
	GroupsPath string `json:"groupsPath"`
}

// IdentityConfig configures the identity provider.
type IdentityConfig struct {
	// Type specifies the identity provider type. Currently only "file" is supported.
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *FileIdentityConfig `json:"file,omitempty"`
}

// FileIdentityConfig configures the file-based identity provider.
type FileIdentityConfig struct {
	// UsersPath is the path to the users JSON file.
	UsersPath string `json:"usersPath"`
}

// ServerConfig configures the auth callout service.
type ServerConfig struct {
	// NatsURL is the NATS server URL.
	NatsURL string `json:"natsUrl"`

	// NatsCredentials is the path to the NATS credentials file.
	// Mutually exclusive with NatsNkey.
	NatsCredentials string `json:"natsCredentials,omitempty"`

	// NatsNkey is the path to the nkey seed file for NATS authentication.
	// Mutually exclusive with NatsCredentials.
	NatsNkey string `json:"natsNkey,omitempty"`

	// XKeySeedFile is the path to a file containing the XKey seed for encryption/decryption.
	XKeySeedFile string `json:"xkeySeedFile,omitempty"`

	// TTL is the default JWT time-to-live as a duration string (e.g., "1h", "30m").
	TTL string `json:"ttl,omitempty"`
}

// LoadConfig reads and parses a configuration file.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &config, nil
}

// Validate checks that the configuration is valid and complete.
func (c *Config) Validate() error {
	// Validate entity config
	if c.Entity.Type == "" {
		c.Entity.Type = "static" // default to static
	}
	switch c.Entity.Type {
	case "operator":
		if c.Entity.Operator == nil {
			return fmt.Errorf("entity.operator configuration is required when type is 'operator'")
		}
		if len(c.Entity.Operator.Accounts) == 0 {
			return fmt.Errorf("entity.operator.accounts must contain at least one account")
		}
		for name, accCfg := range c.Entity.Operator.Accounts {
			if name == "" {
				return fmt.Errorf("entity.operator.accounts contains an empty account name")
			}
			if accCfg.PublicKey == "" {
				return fmt.Errorf("entity.operator.accounts[%s].publicKey is required", name)
			}
			if accCfg.SigningKeyPath == "" {
				return fmt.Errorf("entity.operator.accounts[%s].signingKeyPath is required", name)
			}
		}
	case "static":
		if c.Entity.Static == nil {
			return fmt.Errorf("entity.static configuration is required when type is 'static'")
		}
		if c.Entity.Static.PublicKey == "" {
			return fmt.Errorf("entity.static.publicKey is required")
		}
		if c.Entity.Static.PrivateKeyPath == "" {
			return fmt.Errorf("entity.static.privateKeyPath is required")
		}
		if len(c.Entity.Static.Accounts) == 0 {
			return fmt.Errorf("entity.static.accounts must contain at least one account")
		}
		for i, name := range c.Entity.Static.Accounts {
			if name == "" {
				return fmt.Errorf("entity.static.accounts[%d] cannot be empty", i)
			}
		}
	default:
		return fmt.Errorf("unsupported entity provider type: %s", c.Entity.Type)
	}

	// Validate nauts config
	if c.Nauts.Type == "" {
		c.Nauts.Type = "file" // default to file
	}
	switch c.Nauts.Type {
	case "file":
		if c.Nauts.File == nil {
			return fmt.Errorf("nauts.file configuration is required when type is 'file'")
		}
		if c.Nauts.File.PoliciesPath == "" {
			return fmt.Errorf("nauts.file.policiesPath is required")
		}
		if c.Nauts.File.GroupsPath == "" {
			return fmt.Errorf("nauts.file.groupsPath is required")
		}
	default:
		return fmt.Errorf("unsupported nauts provider type: %s", c.Nauts.Type)
	}

	// Validate identity config
	if c.Identity.Type == "" {
		c.Identity.Type = "file" // default to file
	}
	switch c.Identity.Type {
	case "file":
		if c.Identity.File == nil {
			return fmt.Errorf("identity.file configuration is required when type is 'file'")
		}
		if c.Identity.File.UsersPath == "" {
			return fmt.Errorf("identity.file.usersPath is required")
		}
	default:
		return fmt.Errorf("unsupported identity provider type: %s", c.Identity.Type)
	}

	return nil
}

// GetTTL returns the TTL as a time.Duration, or the default if not set.
func (c *ServerConfig) GetTTL(defaultTTL time.Duration) time.Duration {
	if c.TTL == "" {
		return defaultTTL
	}
	d, err := time.ParseDuration(c.TTL)
	if err != nil {
		return defaultTTL
	}
	return d
}

// GetXKeySeed returns the XKey seed, reading from file.
func (c *ServerConfig) GetXKeySeed() (string, error) {
	if c.XKeySeedFile == "" {
		return "", nil
	}
	data, err := os.ReadFile(c.XKeySeedFile)
	if err != nil {
		return "", fmt.Errorf("reading xkey seed file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// NewAuthControllerWithConfig creates a new AuthController from a Config.
// It initializes all providers based on the configuration.
func NewAuthControllerWithConfig(config *Config, opts ...ControllerOption) (*AuthController, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Initialize entity provider
	var entityProvider provider.EntityProvider
	var err error

	switch config.Entity.Type {
	case "operator":
		accounts := make(map[string]provider.AccountSigningConfig)
		for name, accCfg := range config.Entity.Operator.Accounts {
			accounts[name] = provider.AccountSigningConfig{
				PublicKey:      accCfg.PublicKey,
				SigningKeyPath: accCfg.SigningKeyPath,
			}
		}
		entityProvider, err = provider.NewOperatorEntityProvider(provider.OperatorEntityProviderConfig{
			Accounts: accounts,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing operator entity provider: %w", err)
		}
	case "static":
		entityProvider, err = provider.NewStaticEntityProvider(provider.StaticEntityProviderConfig{
			PublicKey:      config.Entity.Static.PublicKey,
			PrivateKeyPath: config.Entity.Static.PrivateKeyPath,
			Accounts:       config.Entity.Static.Accounts,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing static entity provider: %w", err)
		}
	}

	// Initialize nauts provider
	var nautsProvider provider.NautsProvider

	switch config.Nauts.Type {
	case "file":
		nautsProvider, err = provider.NewFileNautsProvider(provider.FileNautsProviderConfig{
			PoliciesPath: config.Nauts.File.PoliciesPath,
			GroupsPath:   config.Nauts.File.GroupsPath,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing file nauts provider: %w", err)
		}
	}

	// Initialize identity provider
	var identityProvider identity.UserIdentityProvider

	switch config.Identity.Type {
	case "file":
		identityProvider, err = identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
			UsersPath: config.Identity.File.UsersPath,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing file identity provider: %w", err)
		}
	}

	return NewAuthController(entityProvider, nautsProvider, identityProvider, opts...), nil
}

// ToCalloutConfig converts the server configuration to a CalloutConfig.
func (c *ServerConfig) ToCalloutConfig() (CalloutConfig, error) {
	xkeySeed, err := c.GetXKeySeed()
	if err != nil {
		return CalloutConfig{}, err
	}

	return CalloutConfig{
		NatsURL:         c.NatsURL,
		NatsCredentials: c.NatsCredentials,
		NatsNkey:        c.NatsNkey,
		XKeySeed:        xkeySeed,
		DefaultTTL:      c.GetTTL(time.Hour),
	}, nil
}
