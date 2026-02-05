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
	Roles        []string          `json:"roles"` // Format: "account.role"
	PasswordHash string            `json:"passwordHash"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// usersFile represents the JSON file structure.
type usersFile struct {
	Users map[string]*fileUser `json:"users"`
}

// FileAuthenticationProvider implements AuthenticationProvider using a JSON file.
type FileAuthenticationProvider struct {
	id       string
	accounts []string
	users    map[string]*fileUser
}

// FileAuthenticationProviderConfig holds configuration for FileAuthenticationProvider.
type FileAuthenticationProviderConfig struct {
	// ID is the provider identifier.
	ID string
	// Accounts is the list of accounts this provider manages (supports wildcards).
	Accounts []string
	// UsersPath is the path to the users JSON file.
	UsersPath string
}

// NewFileAuthenticationProvider creates a new FileAuthenticationProvider from the given configuration.
func NewFileAuthenticationProvider(cfg FileAuthenticationProviderConfig) (*FileAuthenticationProvider, error) {
	fp := &FileAuthenticationProvider{
		id:       cfg.ID,
		accounts: cfg.Accounts,
		users:    make(map[string]*fileUser),
	}

	if cfg.UsersPath != "" {
		if err := fp.loadUsers(cfg.UsersPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

// loadUsers loads users from a JSON file.
func (fp *FileAuthenticationProvider) loadUsers(path string) error {
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

// ID returns the unique identifier for this provider.
func (fp *FileAuthenticationProvider) ID() string {
	return fp.id
}

// CanManageAccount returns true if this provider is authorized to authenticate
// users for the given account.
func (fp *FileAuthenticationProvider) CanManageAccount(account string) bool {
	for _, pattern := range fp.accounts {
		if MatchAccountPattern(pattern, account) {
			return true
		}
	}
	return false
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
func (fp *FileAuthenticationProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
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

	// Parse role strings to AccountRole objects
	roles, errs := ParseAccountRoles(fu.Roles)
	if len(errs) > 0 {
		// Log errors but continue with valid roles
		// TODO: integrate with logging system
		for _, e := range errs {
			_ = e
		}
	}

	return &User{
		ID:         creds.Username,
		Roles:      roles,
		Attributes: fu.Attributes,
	}, nil
}

// GetUser retrieves a user by ID without verifying credentials.
// Returns ErrUserNotFound if the user does not exist.
func (fp *FileAuthenticationProvider) GetUser(_ context.Context, id string) (*User, error) {
	fu, ok := fp.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	// Parse role strings to AccountRole objects
	roles, errs := ParseAccountRoles(fu.Roles)
	if len(errs) > 0 {
		// Log errors but continue with valid roles
		for _, e := range errs {
			_ = e
		}
	}

	return &User{
		ID:         id,
		Roles:      roles,
		Attributes: fu.Attributes,
	}, nil
}
