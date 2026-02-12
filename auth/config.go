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

	// Policy provider configuration
	Policy PolicyConfig `json:"policy"`

	// Auth provider configuration
	Auth AuthConfig `json:"auth"`

	// Server configuration (for serve mode)
	Server ServerConfig `json:"server"`
}

// AccountConfig configures the account provider.
type AccountConfig struct {
	// Type specifies the account provider type: "operator" or "static".
	Type string `json:"type"`

	// Operator contains operator mode configuration.
	Operator *provider.OperatorAccountProviderConfig `json:"operator,omitempty"`

	// Static contains static account provider configuration.
	Static *provider.StaticAccountProviderConfig `json:"static,omitempty"`
}

// PolicyConfig configures the policy provider.
type PolicyConfig struct {
	// Type specifies the policy provider type: "file" or "nats".
	Type string `json:"type"`

	// File contains file-based provider configuration.
	File *provider.FilePolicyProviderConfig `json:"file,omitempty"`

	// Nats contains NATS KV-based provider configuration.
	Nats *provider.NatsPolicyProviderConfig `json:"nats,omitempty"`
}

// AuthConfig configures the authentication providers.
//
// Multiple providers can be configured (file, jwt, and/or aws). Each provider must have a unique id.
type AuthConfig struct {
	JWT  []JwtAuthProviderConfig  `json:"jwt,omitempty"`
	File []FileAuthProviderConfig `json:"file,omitempty"`
	Aws  []AwsAuthProviderConfig  `json:"aws,omitempty"`
}

type JwtAuthProviderConfig struct {
	ID string `json:"id"`

	Accounts []string `json:"accounts"`
	Issuer   string   `json:"issuer"`
	// PublicKey is a base64 encoded PEM block.
	PublicKey      string `json:"publicKey"`
	RolesClaimPath string `json:"rolesClaimPath,omitempty"`
}

type FileAuthProviderConfig struct {
	ID string `json:"id"`

	Accounts []string `json:"accounts"`
	// UsersPath is the path to the users JSON file.
	UsersPath string `json:"userPath"`
}

type AwsAuthProviderConfig struct {
	ID string `json:"id"`

	Accounts     []string      `json:"accounts"`
	Region       string        `json:"region,omitempty"`
	MaxClockSkew time.Duration `json:"maxClockSkew,omitempty"`
	AWSAccount   string        `json:"awsAccount"`
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

	// Validate policy config
	if c.Policy.Type == "" {
		c.Policy.Type = "file" // default to file
	}
	switch c.Policy.Type {
	case "file":
		if c.Policy.File == nil {
			return fmt.Errorf("policy.file configuration is required when type is 'file'")
		}
		if c.Policy.File.PoliciesPath == "" {
			return fmt.Errorf("policy.file.policiesPath is required")
		}
		if c.Policy.File.BindingsPath == "" {
			return fmt.Errorf("policy.file.bindingsPath is required")
		}
	case "nats":
		if c.Policy.Nats == nil {
			return fmt.Errorf("policy.nats configuration is required when type is 'nats'")
		}
		if c.Policy.Nats.Bucket == "" {
			return fmt.Errorf("policy.nats.bucket is required")
		}
		if c.Policy.Nats.NatsURL == "" {
			return fmt.Errorf("policy.nats.natsUrl is required")
		}
		if c.Policy.Nats.NatsCredentials != "" && c.Policy.Nats.NatsNkey != "" {
			return fmt.Errorf("policy.nats.natsCredentials and policy.nats.natsNkey are mutually exclusive")
		}
	default:
		return fmt.Errorf("unsupported policy provider type: %s", c.Policy.Type)
	}

	// Validate identity config
	providerCount := len(c.Auth.JWT) + len(c.Auth.File) + len(c.Auth.Aws)
	if providerCount == 0 {
		return fmt.Errorf("auth must contain at least one authentication provider")
	}

	ids := make(map[string]struct{}, providerCount)
	for i, p := range c.Auth.File {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("auth.file[%d].id is required", i)
		}
		if _, ok := ids[p.ID]; ok {
			return fmt.Errorf("auth providers contain duplicate id: %s", p.ID)
		}
		ids[p.ID] = struct{}{}
		if p.UsersPath == "" {
			return fmt.Errorf("auth.file[%s].userPath is required", p.ID)
		}
		if len(p.Accounts) == 0 {
			return fmt.Errorf("auth.file[%s].accounts must contain at least one account", p.ID)
		}
	}
	for i, p := range c.Auth.JWT {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("auth.jwt[%d].id is required", i)
		}
		if _, ok := ids[p.ID]; ok {
			return fmt.Errorf("auth providers contain duplicate id: %s", p.ID)
		}
		ids[p.ID] = struct{}{}
		if p.Issuer == "" {
			return fmt.Errorf("auth.jwt[%s].issuer is required", p.ID)
		}
		if p.PublicKey == "" {
			return fmt.Errorf("auth.jwt[%s].publicKey is required", p.ID)
		}
		if len(p.Accounts) == 0 {
			return fmt.Errorf("auth.jwt[%s].accounts must contain at least one account", p.ID)
		}
	}
	for i, p := range c.Auth.Aws {
		if strings.TrimSpace(p.ID) == "" {
			return fmt.Errorf("auth.aws[%d].id is required", i)
		}
		if _, ok := ids[p.ID]; ok {
			return fmt.Errorf("auth providers contain duplicate id: %s", p.ID)
		}
		ids[p.ID] = struct{}{}
		if len(p.Accounts) == 0 {
			return fmt.Errorf("auth.aws[%s].accounts must contain at least one account", p.ID)
		}
		if strings.TrimSpace(p.AWSAccount) == "" {
			return fmt.Errorf("auth.aws[%s].awsAccount is required", p.ID)
		}
		if p.AWSAccount == "*" || strings.Contains(p.AWSAccount, "*") {
			return fmt.Errorf("auth.aws[%s].awsAccount must not contain wildcards", p.ID)
		}
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
		accountProvider, err = provider.NewOperatorAccountProvider(*config.Account.Operator)
		if err != nil {
			return nil, fmt.Errorf("initializing operator account provider: %w", err)
		}
	case "static":
		accountProvider, err = provider.NewStaticAccountProvider(*config.Account.Static)
		if err != nil {
			return nil, fmt.Errorf("initializing static account provider: %w", err)
		}
	}

	// Initialize policy provider
	var policyProvider provider.PolicyProvider

	switch config.Policy.Type {
	case "file":
		policyProvider, err = provider.NewFilePolicyProvider(*config.Policy.File)
		if err != nil {
			return nil, fmt.Errorf("initializing file policy provider: %w", err)
		}
	case "nats":
		policyProvider, err = provider.NewNatsPolicyProvider(*config.Policy.Nats)
		if err != nil {
			return nil, fmt.Errorf("initializing nats policy provider: %w", err)
		}
	}

	providers := make(map[string]identity.AuthenticationProvider)
	for _, fc := range config.Auth.File {
		p, err := identity.NewFileAuthenticationProvider(identity.FileAuthenticationProviderConfig{
			UsersPath: fc.UsersPath,
			Accounts:  fc.Accounts,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing file authentication provider %q: %w", fc.ID, err)
		}
		providers[fc.ID] = p
	}
	for _, jc := range config.Auth.JWT {
		p, err := identity.NewJwtAuthenticationProvider(identity.JwtAuthenticationProviderConfig{
			Accounts:       jc.Accounts,
			Issuer:         jc.Issuer,
			PublicKey:      jc.PublicKey,
			RolesClaimPath: jc.RolesClaimPath,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing jwt authentication provider %q: %w", jc.ID, err)
		}
		providers[jc.ID] = p
	}
	for _, ac := range config.Auth.Aws {
		p, err := identity.NewAwsSigV4AuthenticationProvider(identity.AwsSigV4AuthenticationProviderConfig{
			Accounts:     ac.Accounts,
			Region:       ac.Region,
			MaxClockSkew: ac.MaxClockSkew,
			AWSAccount:   ac.AWSAccount,
		})
		if err != nil {
			return nil, fmt.Errorf("initializing aws authentication provider %q: %w", ac.ID, err)
		}
		providers[ac.ID] = p
	}

	authProviders, err := identity.NewAuthenticationProviderManager(providers)
	if err != nil {
		return nil, fmt.Errorf("initializing authentication providers: %w", err)
	}

	return NewAuthController(accountProvider, policyProvider, authProviders, opts...), nil
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

// ToDebugConfig converts the server configuration to a DebugConfig.
func (c *ServerConfig) ToDebugConfig() DebugConfig {
	return DebugConfig{
		NatsURL:         c.NatsURL,
		NatsCredentials: c.NatsCredentials,
		NatsNkey:        c.NatsNkey,
		DefaultTTL:      c.GetTTL(time.Hour),
	}
}
