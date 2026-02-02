package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	natsjwt "github.com/nats-io/jwt/v2"

	"github.com/msimon/nauts/jwt"
)

// NscEntityProvider implements NatsEntityProvider using an nsc directory structure.
type NscEntityProvider struct {
	operator *Operator
	accounts map[string]*Account
}

// NscConfig holds configuration for the NscEntityProvider.
type NscConfig struct {
	// Dir is the path to the nsc directory (containing "nats" and "keys" subdirectories).
	Dir string

	// OperatorName is the name of the operator to use.
	OperatorName string
}

// NewNscEntityProvider creates a new NscEntityProvider by discovering entities
// from the given nsc directory.
func NewNscEntityProvider(cfg NscConfig) (*NscEntityProvider, error) {
	if cfg.Dir == "" {
		return nil, fmt.Errorf("nsc directory path is required")
	}
	if cfg.OperatorName == "" {
		return nil, fmt.Errorf("operator name is required")
	}

	provider := &NscEntityProvider{
		accounts: make(map[string]*Account),
	}

	if err := provider.discoverOperator(cfg); err != nil {
		return nil, fmt.Errorf("discovering operator: %w", err)
	}

	if err := provider.discoverAccounts(cfg); err != nil {
		return nil, fmt.Errorf("discovering accounts: %w", err)
	}

	return provider, nil
}

func (p *NscEntityProvider) discoverOperator(cfg NscConfig) error {
	operatorJWTPath := filepath.Join(cfg.Dir, "nats", cfg.OperatorName, cfg.OperatorName+".jwt")

	jwtData, err := os.ReadFile(operatorJWTPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", ErrOperatorNotFound, cfg.OperatorName)
		}
		return fmt.Errorf("reading operator JWT: %w", err)
	}

	claims, err := natsjwt.DecodeOperatorClaims(string(jwtData))
	if err != nil {
		return fmt.Errorf("decoding operator JWT: %w", err)
	}

	publicKey := claims.Subject

	signer, err := p.loadSigner(cfg.Dir, publicKey)
	if err != nil {
		return fmt.Errorf("loading operator signer: %w", err)
	}

	p.operator = &Operator{
		name:      cfg.OperatorName,
		publicKey: publicKey,
		signer:    signer,
	}

	return nil
}

func (p *NscEntityProvider) discoverAccounts(cfg NscConfig) error {
	accountsDir := filepath.Join(cfg.Dir, "nats", cfg.OperatorName, "accounts")

	entries, err := os.ReadDir(accountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading accounts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		accountName := entry.Name()
		accountJWTPath := filepath.Join(accountsDir, accountName, accountName+".jwt")

		jwtData, err := os.ReadFile(accountJWTPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("reading account JWT for %s: %w", accountName, err)
		}

		claims, err := natsjwt.DecodeAccountClaims(string(jwtData))
		if err != nil {
			return fmt.Errorf("decoding account JWT for %s: %w", accountName, err)
		}

		publicKey := claims.Subject

		signer, err := p.loadSigner(cfg.Dir, publicKey)
		if err != nil {
			return fmt.Errorf("loading signer for account %s: %w", accountName, err)
		}

		p.accounts[accountName] = &Account{
			name:      accountName,
			publicKey: publicKey,
			signer:    signer,
		}
	}

	return nil
}

func (p *NscEntityProvider) loadSigner(nscDir, publicKey string) (*jwt.LocalSigner, error) {
	keyType := getKeyTypePrefix(publicKey)
	if keyType == "" {
		return nil, fmt.Errorf("unknown key type for public key: %s", publicKey)
	}

	keyPrefix := publicKey[1:3]

	seedPath := filepath.Join(nscDir, "keys", "keys", keyType, keyPrefix, publicKey+".nk")

	seedData, err := os.ReadFile(seedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s", ErrSigningKeyNotFound, publicKey)
		}
		return nil, fmt.Errorf("reading seed file: %w", err)
	}

	seed := strings.TrimSpace(string(seedData))

	return jwt.NewLocalSigner(seed)
}

func getKeyTypePrefix(publicKey string) string {
	if len(publicKey) == 0 {
		return ""
	}

	switch publicKey[0] {
	case 'O':
		return "O"
	case 'A':
		return "A"
	case 'U':
		return "U"
	case 'N':
		return "N"
	case 'C':
		return "C"
	default:
		return ""
	}
}

// GetOperator returns the operator entity.
func (p *NscEntityProvider) GetOperator(ctx context.Context) (*Operator, error) {
	if p.operator == nil {
		return nil, ErrOperatorNotFound
	}
	return p.operator, nil
}

// GetAccount retrieves an account by name.
func (p *NscEntityProvider) GetAccount(ctx context.Context, name string) (*Account, error) {
	account, ok := p.accounts[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrAccountNotFound, name)
	}
	return account, nil
}

// ListAccounts returns all accounts.
func (p *NscEntityProvider) ListAccounts(ctx context.Context) ([]*Account, error) {
	accounts := make([]*Account, 0, len(p.accounts))
	for _, account := range p.accounts {
		accounts = append(accounts, account)
	}
	return accounts, nil
}
