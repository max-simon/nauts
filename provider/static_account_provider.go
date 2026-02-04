package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/msimon/nauts/jwt"
)

// StaticAccountProvider implements AccountProvider using a static configuration.
type StaticAccountProvider struct {
	accounts map[string]*Account
}

// StaticAccountProviderConfig holds configuration for the StaticAccountProvider.
type StaticAccountProviderConfig struct {
	// PublicKey is the public key used for all accounts.
	PublicKey string `json:"publicKey"`

	// PrivateKeyPath is the path to the nkey seed file used for all accounts.
	PrivateKeyPath string `json:"privateKeyPath"`

	// Accounts is the list of account names.
	Accounts []string `json:"accounts"`
}

// NewStaticAccountProvider creates a new StaticAccountProvider from configuration.
func NewStaticAccountProvider(cfg StaticAccountProviderConfig) (*StaticAccountProvider, error) {
	if len(cfg.Accounts) == 0 {
		return nil, fmt.Errorf("at least one account is required")
	}
	if cfg.PublicKey == "" {
		return nil, fmt.Errorf("publicKey is required")
	}
	if cfg.PrivateKeyPath == "" {
		return nil, fmt.Errorf("privateKeyPath is required")
	}

	signer, err := loadSignerFromFile(cfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("loading signer: %w", err)
	}

	provider := &StaticAccountProvider{
		accounts: make(map[string]*Account),
	}

	for _, name := range cfg.Accounts {
		if name == "" {
			return nil, fmt.Errorf("account name cannot be empty")
		}

		provider.accounts[name] = &Account{
			name:      name,
			publicKey: cfg.PublicKey,
			signer:    signer,
		}
	}

	return provider, nil
}

func loadSignerFromFile(path string) (*jwt.LocalSigner, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading key file: %w", err)
	}

	seed := strings.TrimSpace(string(data))
	return jwt.NewLocalSigner(seed)
}

// GetAccount retrieves an account by name.
func (p *StaticAccountProvider) GetAccount(ctx context.Context, name string) (*Account, error) {
	account, ok := p.accounts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAccountNotFound, name)
	}
	return account, nil
}

// ListAccounts returns all accounts.
func (p *StaticAccountProvider) ListAccounts(ctx context.Context) ([]*Account, error) {
	accounts := make([]*Account, 0, len(p.accounts))
	for _, account := range p.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// IsOperatorMode returns false as StaticAccountProvider does not operate in operator mode.
func (p *StaticAccountProvider) IsOperatorMode() bool {
	return false
}
