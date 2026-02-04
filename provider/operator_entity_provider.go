package provider

import (
	"context"
	"fmt"

	"github.com/msimon/nauts/jwt"
)

// OperatorEntityProvider implements EntityProvider for NATS operator mode.
// In operator mode, the auth service runs in the AUTH account but authenticates
// users across all accounts using account signing keys.
type OperatorEntityProvider struct {
	accounts map[string]*Account
}

// OperatorEntityProviderConfig holds configuration for the OperatorEntityProvider.
type OperatorEntityProviderConfig struct {
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

// NewOperatorEntityProvider creates a new OperatorEntityProvider from configuration.
func NewOperatorEntityProvider(cfg OperatorEntityProviderConfig) (*OperatorEntityProvider, error) {
	if len(cfg.Accounts) == 0 {
		return nil, fmt.Errorf("at least one account is required")
	}

	provider := &OperatorEntityProvider{
		accounts: make(map[string]*Account),
	}

	for name, accCfg := range cfg.Accounts {
		if name == "" {
			return nil, fmt.Errorf("account name cannot be empty")
		}
		if accCfg.PublicKey == "" {
			return nil, fmt.Errorf("publicKey is required for account %s", name)
		}
		if accCfg.SigningKeyPath == "" {
			return nil, fmt.Errorf("signingKeyPath is required for account %s", name)
		}

		signer, err := loadSignerFromFile(accCfg.SigningKeyPath)
		if err != nil {
			return nil, fmt.Errorf("loading signer for account %s: %w", name, err)
		}

		provider.accounts[name] = &Account{
			name:      name,
			publicKey: accCfg.PublicKey,
			signer:    signer,
		}
	}

	return provider, nil
}

// GetOperator returns an error as OperatorEntityProvider does not provide operator access.
// In operator mode, only account signing keys are needed for auth callout.
func (p *OperatorEntityProvider) GetOperator(ctx context.Context) (*Operator, error) {
	return nil, fmt.Errorf("operator not available in operator entity provider")
}

// GetAccount retrieves an account by name.
func (p *OperatorEntityProvider) GetAccount(ctx context.Context, name string) (*Account, error) {
	account, ok := p.accounts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAccountNotFound, name)
	}
	return account, nil
}

// ListAccounts returns all accounts.
func (p *OperatorEntityProvider) ListAccounts(ctx context.Context) ([]*Account, error) {
	accounts := make([]*Account, 0, len(p.accounts))
	for _, account := range p.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}

// IsOperatorMode returns true as this provider operates in NATS operator mode.
func (p *OperatorEntityProvider) IsOperatorMode() bool {
	return true
}

// SigningKeyPublicKey returns the public key of the signing key for the given account.
// This is useful for setting IssuerAccount in auth callout responses.
func (p *OperatorEntityProvider) SigningKeyPublicKey(accountName string) (string, error) {
	account, ok := p.accounts[accountName]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrAccountNotFound, accountName)
	}
	return account.Signer().(jwt.Signer).PublicKey(), nil
}
