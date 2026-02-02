package identity

import (
	"context"
	"encoding/json"
	"os"

	"golang.org/x/crypto/bcrypt"
)

// UsernamePassword is the identity token type for the file user provider.
type UsernamePassword struct {
	Username string
	Password string
}

// fileUser represents a user stored in the JSON file.
type fileUser struct {
	Account      string            `json:"account"`
	Groups       []string          `json:"groups"`
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

// Verify validates UsernamePassword token and returns the user.
// Returns ErrInvalidTokenType if token is not UsernamePassword.
// Returns ErrUserNotFound if the user does not exist.
// Returns ErrInvalidCredentials if the password is incorrect.
func (fp *FileUserIdentityProvider) Verify(_ context.Context, token IdentityToken) (*User, error) {
	creds, ok := token.(UsernamePassword)
	if !ok {
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

	return &User{
		ID:         creds.Username,
		Account:    fu.Account,
		Groups:     fu.Groups,
		Attributes: fu.Attributes,
	}, nil
}

// GetUser retrieves a user by ID without verifying credentials.
// Returns ErrUserNotFound if the user does not exist.
func (fp *FileUserIdentityProvider) GetUser(_ context.Context, id string) (*User, error) {
	fu, ok := fp.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	return &User{
		ID:         id,
		Account:    fu.Account,
		Groups:     fu.Groups,
		Attributes: fu.Attributes,
	}, nil
}
