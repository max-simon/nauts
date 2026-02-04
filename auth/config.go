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
	// Account provider configuration
	Account AccountConfig `json:"account"`

	// Role provider configuration
	Role RoleConfig `json:"role"`

	// Policy provider configuration
	Policy PolicyConfig `json:"policy"`

	// Identity provider configuration
	Identity IdentityConfig `json:"identity"`

	// Server configuration (for serve mode)
	Server ServerConfig `json:"server"`
}

// AccountConfig configures the account provider.
type AccountConfig struct {
	// Type specifies the account provider type: "operator" or "static".
	Type string `json:"type"`

	// Operator contains operator mode configuration.
	Operator *OperatorAccountConfig `json:"operator,omitempty"`

	// Static contains static account provider configuration.
	Static *StaticAccountConfig `json:"static,omitempty"`
}

// OperatorAccountConfig configures the operator account provider.
// In operator mode, the auth service runs in the AUTH account but authenticates
// users across all accounts using account signing keys.
type OperatorAccountConfig struct {
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

// StaticAccountConfig configures the static account provider.
type StaticAccountConfig struct {
	// PublicKey is the public key used for all accounts.
	PublicKey string `json:"publicKey"`

	// PrivateKeyPath is the path to the nkey seed file used for all accounts.
	PrivateKeyPath string `json:"privateKeyPath"`

	// Accounts is the list of account names.
	Accounts []string `json:"accounts"`
}

// RoleConfig configures the role provider.
type RoleConfig struct {
	// Type specifies the role provider type. Currently only "file" is supported.
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *FileRoleConfig `json:"file,omitempty"`
}

// FileRoleConfig configures the file-based role provider.
type FileRoleConfig struct {
	// Path is the path to the roles JSON file.
	Path string `json:"path"`
}

// PolicyConfig configures the policy provider.
type PolicyConfig struct {
	// Type specifies the policy provider type. Currently only "file" is supported.
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *FilePolicyConfig `json:"file,omitempty"`
}

// FilePolicyConfig configures the file-based policy provider.
type FilePolicyConfig struct {
	// Path is the path to the policies JSON file.
	Path string `json:"path"`
}

// IdentityConfig configures the identity provider.
type IdentityConfig struct {
	// Type specifies the identity provider type: "file" or "jwt".
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *FileIdentityConfig `json:"file,omitempty"`

	// JWT contains JWT-based provider configuration.
	JWT *JwtIdentityConfig `json:"jwt,omitempty"`
}

// FileIdentityConfig configures the file-based identity provider.
type FileIdentityConfig struct {
	// UsersPath is the path to the users JSON file.
	UsersPath string `json:"usersPath"`
}

// JwtIdentityConfig configures the JWT-based identity provider.
type JwtIdentityConfig struct {
	// Issuers maps JWT issuer (iss claim) to their configuration.
	Issuers map[string]JwtIssuerConfig `json:"issuers"`
}

// JwtIssuerConfig holds configuration for a single JWT issuer.
type JwtIssuerConfig struct {
	// PublicKey is the PEM-encoded public key for JWT signature verification.
	PublicKey string `json:"publicKey"`

	// Accounts is the list of NATS accounts this issuer can manage.
	// Supports wildcards: "*" matches any account, "tenant-a-*" matches accounts starting with "tenant-a-".
	Accounts []string `json:"accounts"`

	// RolesClaimPath is the path to roles in JWT claims (dot-separated).
	// Default: "resource_access.nauts.roles"
	RolesClaimPath string `json:"rolesClaimPath,omitempty"`
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
	// Validate account config
	if c.Account.Type == "" {
		c.Account.Type = "static" // default to static
	}
	switch c.Account.Type {
	case "operator":
		if c.Account.Operator == nil {
			return fmt.Errorf("account.operator configuration is required when type is 'operator'")
		}
		if len(c.Account.Operator.Accounts) == 0 {
			return fmt.Errorf("account.operator.accounts must contain at least one account")
		}
		for name, accCfg := range c.Account.Operator.Accounts {
			if name == "" {
				return fmt.Errorf("account.operator.accounts contains an empty account name")
			}
			if accCfg.PublicKey == "" {
				return fmt.Errorf("account.operator.accounts[%s].publicKey is required", name)
			}
			if accCfg.SigningKeyPath == "" {
				return fmt.Errorf("account.operator.accounts[%s].signingKeyPath is required", name)
			}
		}
	case "static":
		if c.Account.Static == nil {
			return fmt.Errorf("account.static configuration is required when type is 'static'")
		}
		if c.Account.Static.PublicKey == "" {
			return fmt.Errorf("account.static.publicKey is required")
		}
		if c.Account.Static.PrivateKeyPath == "" {
			return fmt.Errorf("account.static.privateKeyPath is required")
		}
		if len(c.Account.Static.Accounts) == 0 {
			return fmt.Errorf("account.static.accounts must contain at least one account")
		}
		for i, name := range c.Account.Static.Accounts {
			if name == "" {
				return fmt.Errorf("account.static.accounts[%d] cannot be empty", i)
			}
		}
	default:
		return fmt.Errorf("unsupported account provider type: %s", c.Account.Type)
	}

	// Validate role config
	if c.Role.Type == "" {
		c.Role.Type = "file" // default to file
	}
	switch c.Role.Type {
	case "file":
		if c.Role.File == nil {
			return fmt.Errorf("role.file configuration is required when type is 'file'")
		}
		if c.Role.File.Path == "" {
			return fmt.Errorf("role.file.path is required")
		}
	default:
		return fmt.Errorf("unsupported role provider type: %s", c.Role.Type)
	}

	// Validate policy config
	if c.Policy.Type == "" {
		c.Policy.Type = "file" // default to file
	}
	switch c.Policy.Type {
	case "file":
		if c.Policy.File == nil {
			return fmt.Errorf("policy.file configuration is required when type is 'file'")
		}
		if c.Policy.File.Path == "" {
			return fmt.Errorf("policy.file.path is required")
		}
	default:
		return fmt.Errorf("unsupported policy provider type: %s", c.Policy.Type)
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
	case "jwt":
		if c.Identity.JWT == nil {
			return fmt.Errorf("identity.jwt configuration is required when type is 'jwt'")
		}
		if len(c.Identity.JWT.Issuers) == 0 {
			return fmt.Errorf("identity.jwt.issuers must contain at least one issuer")
		}
		for issuer, issuerCfg := range c.Identity.JWT.Issuers {
			if issuer == "" {
				return fmt.Errorf("identity.jwt.issuers contains an empty issuer name")
			}
			if issuerCfg.PublicKey == "" {
				return fmt.Errorf("identity.jwt.issuers[%s].publicKey is required", issuer)
			}
			if len(issuerCfg.Accounts) == 0 {
				return fmt.Errorf("identity.jwt.issuers[%s].accounts must contain at least one account", issuer)
			}
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

	// Initialize account provider
	var accountProvider provider.AccountProvider
	var err error

	switch config.Account.Type {
	case "operator":
		accounts := make(map[string]provider.AccountSigningConfig)
		for name, accCfg := range config.Account.Operator.Accounts {
			accounts[name] = provider.AccountSigningConfig{
				PublicKey:      accCfg.PublicKey,
				SigningKeyPath: accCfg.SigningKeyPath,
			}
		}
		accountProvider, err = provider.NewOperatorAccountProvider(provider.OperatorAccountProviderConfig{
			Accounts: accounts,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing operator account provider: %w", err)
		}
	case "static":
		accountProvider, err = provider.NewStaticAccountProvider(provider.StaticAccountProviderConfig{
			PublicKey:      config.Account.Static.PublicKey,
			PrivateKeyPath: config.Account.Static.PrivateKeyPath,
			Accounts:       config.Account.Static.Accounts,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing static account provider: %w", err)
		}
	}

	// Initialize role provider
	var roleProvider provider.RoleProvider

	switch config.Role.Type {
	case "file":
		roleProvider, err = provider.NewFileRoleProvider(provider.FileRoleProviderConfig{
			RolesPath: config.Role.File.Path,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing file role provider: %w", err)
		}
	}

	// Initialize policy provider
	var policyProvider provider.PolicyProvider

	switch config.Policy.Type {
	case "file":
		policyProvider, err = provider.NewFilePolicyProvider(provider.FilePolicyProviderConfig{
			PoliciesPath: config.Policy.File.Path,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing file policy provider: %w", err)
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
	case "jwt":
		issuers := make(map[string]identity.IssuerConfig)
		for issuer, issuerCfg := range config.Identity.JWT.Issuers {
			issuers[issuer] = identity.IssuerConfig{
				PublicKey:      issuerCfg.PublicKey,
				Accounts:       issuerCfg.Accounts,
				RolesClaimPath: issuerCfg.RolesClaimPath,
			}
		}
		identityProvider, err = identity.NewJwtUserIdentityProvider(identity.JwtUserIdentityProviderConfig{
			Issuers: issuers,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing jwt identity provider: %w", err)
		}
	}

	return NewAuthController(accountProvider, roleProvider, policyProvider, identityProvider, opts...), nil
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
