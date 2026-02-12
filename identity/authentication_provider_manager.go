package identity

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrAuthenticationProviderNotFound is returned when an explicit provider id cannot be resolved.
	ErrAuthenticationProviderNotFound = errors.New("authentication provider not found")

	// ErrAuthenticationProviderAmbiguous is returned when multiple providers match a request and no provider id is specified.
	ErrAuthenticationProviderAmbiguous = errors.New("authentication provider is ambiguous")

	// ErrAuthenticationProviderNotManageable is returned when a request targets an account not manageable by the selected provider.
	ErrAuthenticationProviderNotManageable = errors.New("account is not manageable by provider")
)

type registeredAuthenticationProvider struct {
	id       string
	provider AuthenticationProvider
}

// AuthenticationProviderManager routes authentication requests to the correct provider.
//
// Selection rules:
//   - If req.AP is set, the provider is selected by id.
//   - If req.AP is empty, the manager selects all providers that can manage req.Account.
//     If exactly one matches, it is used; if none or many match, an error is returned.
//
// Manageable account matching supports patterns "*" and "prefix*".
// Wildcards do not match SYS or AUTH; those accounts must be explicitly listed.
type AuthenticationProviderManager struct {
	providers   []registeredAuthenticationProvider
	providersBy map[string]AuthenticationProvider
}

// NewAuthenticationProviderManager constructs an AuthenticationProviderManager.
func NewAuthenticationProviderManager(providers map[string]AuthenticationProvider) (*AuthenticationProviderManager, error) {
	if len(providers) == 0 {
		return nil, fmt.Errorf("no authentication providers configured")
	}

	m := &AuthenticationProviderManager{
		providersBy: make(map[string]AuthenticationProvider, len(providers)),
	}

	for id, p := range providers {
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("authentication provider id cannot be empty")
		}
		if p == nil {
			return nil, fmt.Errorf("authentication provider %q is nil", id)
		}
		if _, ok := m.providersBy[id]; ok {
			return nil, fmt.Errorf("duplicate authentication provider id: %q", id)
		}
		m.providersBy[id] = p
		m.providers = append(m.providers, registeredAuthenticationProvider{id: id, provider: p})
	}

	return m, nil
}

// SelectProvider selects the provider for a request without performing verification.
// Returns the provider id and instance, or an error if selection is invalid or ambiguous.
func (m *AuthenticationProviderManager) SelectProvider(req AuthRequest) (string, AuthenticationProvider, error) {
	if req.AP != "" {
		p, ok := m.providersBy[req.AP]
		if !ok {
			return "", nil, fmt.Errorf("%w: %s", ErrAuthenticationProviderNotFound, req.AP)
		}
		if !accountIsManageableByProvider(p.ManageableAccounts(), req.Account) {
			return "", nil, fmt.Errorf("%w: %s", ErrAuthenticationProviderNotManageable, req.Account)
		}
		return req.AP, p, nil
	}

	matches := make([]registeredAuthenticationProvider, 0, 1)
	for _, rp := range m.providers {
		if accountIsManageableByProvider(rp.provider.ManageableAccounts(), req.Account) {
			matches = append(matches, rp)
		}
	}

	switch len(matches) {
	case 0:
		return "", nil, fmt.Errorf("%w: %s", ErrAuthenticationProviderNotManageable, req.Account)
	case 1:
		return matches[0].id, matches[0].provider, nil
	default:
		return "", nil, fmt.Errorf("%w: %d providers match account %q", ErrAuthenticationProviderAmbiguous, len(matches), req.Account)
	}
}

func accountIsManageableByProvider(patterns []string, account string) bool {
	if account == "" {
		return false
	}
	if account == "SYS" || account == "AUTH" {
		for _, p := range patterns {
			if p == account {
				return true
			}
		}
		return false
	}

	for _, pattern := range patterns {
		if matchAccountPattern(pattern, account) {
			return true
		}
	}
	return false
}

func matchAccountPattern(pattern, account string) bool {
	if pattern == "" || account == "" {
		return false
	}
	if pattern == account {
		return true
	}
	if pattern == "*" {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		if prefix == "" {
			return true
		}
		return strings.HasPrefix(account, prefix)
	}
	return false
}
