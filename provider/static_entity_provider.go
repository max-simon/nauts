package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/msimon/nauts/jwt"
)

// StaticEntityProvider implements EntityProvider using a static configuration.
// It does not support operators (GetOperator always returns an error).
type StaticEntityProvider struct {
	accounts map[string]*Account
}

// StaticEntityProviderConfig holds configuration for the StaticEntityProvider.
type StaticEntityProviderConfig struct {
	// PublicKey is the public key used for all accounts.
	PublicKey string `json:"publicKey"`

	// PrivateKeyPath is the path to the nkey seed file used for all accounts.
	PrivateKeyPath string `json:"privateKeyPath"`

	// Accounts is the list of account names.
	Accounts []string `json:"accounts"`
}

// NewStaticEntityProvider creates a new StaticEntityProvider from configuration.
func NewStaticEntityProvider(cfg StaticEntityProviderConfig) (*StaticEntityProvider, error) {
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

	provider := &StaticEntityProvider{
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

// GetOperator returns an error as StaticEntityProvider does not support operators.
func (p *StaticEntityProvider) GetOperator(ctx context.Context) (*Operator, error) {
	return nil, fmt.Errorf("operator not supported by static entity provider")
}

// GetAccount retrieves an account by name.
func (p *StaticEntityProvider) GetAccount(ctx context.Context, name string) (*Account, error) {
	account, ok := p.accounts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAccountNotFound, name)
	}
	return account, nil
}

// ListAccounts returns all accounts.
func (p *StaticEntityProvider) ListAccounts(ctx context.Context) ([]*Account, error) {
	accounts := make([]*Account, 0, len(p.accounts))
	for _, account := range p.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// IsOperatorMode returns false as StaticEntityProvider does not operate in operator mode.
func (p *StaticEntityProvider) IsOperatorMode() bool {
	return false
}
