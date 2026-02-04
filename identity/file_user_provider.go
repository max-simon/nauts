package identity

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// usernamePassword is the identity token type for the file user provider.
type usernamePassword struct {
	Username string
	Password string
}

// fileUser represents a user stored in the JSON file.
type fileUser struct {
	Accounts     []string          `json:"accounts"`
	Roles        []string          `json:"roles"`
	PasswordHash string            `json:"passwordHash"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// usersFile represents the JSON file structure.
type usersFile struct {
	Users map[string]*fileUser `json:"users"`
}

// FileUserIdentityProvider implements UserIdentityProvider using a JSON file.
type FileUserIdentityProvider struct {
	users map[string]*fileUser
}

// FileUserIdentityProviderConfig holds configuration for FileUserIdentityProvider.
type FileUserIdentityProviderConfig struct {
	// UsersPath is the path to the users JSON file.
	UsersPath string
}

// NewFileUserIdentityProvider creates a new FileUserIdentityProvider from the given configuration.
func NewFileUserIdentityProvider(cfg FileUserIdentityProviderConfig) (*FileUserIdentityProvider, error) {
	fp := &FileUserIdentityProvider{
		users: make(map[string]*fileUser),
	}

	if cfg.UsersPath != "" {
		if err := fp.loadUsers(cfg.UsersPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

// loadUsers loads users from a JSON file.
func (fp *FileUserIdentityProvider) loadUsers(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var file usersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	fp.users = file.Users
	return nil
}

// parseUsernamePassword parses a UsernamePassword token from basic auth format.
func parseUsernamePassword(token string) (*usernamePassword, error) {
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidTokenType
	}
	return &usernamePassword{
		Username: parts[0],
		Password: parts[1],
	}, nil
}

// Verify validates the authentication request and returns the user.
// Returns ErrInvalidTokenType if token is not UsernamePassword format.
// Returns ErrUserNotFound if the user does not exist.
// Returns ErrInvalidCredentials if the password is incorrect.
// Returns ErrInvalidAccount if the requested account is not valid for the user.
// Returns ErrAccountRequired if user has multiple accounts but no account was specified.
func (fp *FileUserIdentityProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
	creds, err := parseUsernamePassword(req.Token)
	if err != nil {
		return nil, ErrInvalidTokenType
	}

	fu, ok := fp.users[creds.Username]
	if !ok {
		return nil, ErrUserNotFound
	}

	// Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(fu.PasswordHash), []byte(creds.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Determine the account
	account, err := resolveAccount(fu.Accounts, req.Account)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:         creds.Username,
		Account:    account,
		Roles:      fu.Roles,
		Attributes: fu.Attributes,
	}, nil
}

// resolveAccount determines the account based on the user's accounts and the requested account.
// If user has single account, returns it (ignoring requested account).
// If user has multiple accounts, requires and validates the requested account.
func resolveAccount(accounts []string, requestedAccount string) (string, error) {
	if len(accounts) == 0 {
		return "", ErrInvalidAccount
	}

	if len(accounts) == 1 {
		// Single account - use it regardless of request
		return accounts[0], nil
	}

	// Multiple accounts - require explicit account selection
	if requestedAccount == "" {
		return "", ErrAccountRequired
	}

	// Validate requested account is in user's accounts list
	for _, acc := range accounts {
		if acc == requestedAccount {
			return requestedAccount, nil
		}
	}

	return "", ErrInvalidAccount
}

// GetUser retrieves a user by ID without verifying credentials.
// Returns ErrUserNotFound if the user does not exist.
// Note: For users with multiple accounts, this returns the first account.
// Use Verify with AuthRequest to specify a specific account.
func (fp *FileUserIdentityProvider) GetUser(_ context.Context, id string) (*User, error) {
	fu, ok := fp.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	// For GetUser, use the first account (or empty if no accounts)
	account := ""
	if len(fu.Accounts) > 0 {
		account = fu.Accounts[0]
	}

	return &User{
		ID:         id,
		Account:    account,
		Roles:      fu.Roles,
		Attributes: fu.Attributes,
	}, nil
}
