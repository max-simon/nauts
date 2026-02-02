// Package static provides a file-based user identity provider with bcrypt passwords.
package static

import (
	"context"
	"encoding/json"
	"os"

	"golang.org/x/crypto/bcrypt"

	"github.com/msimon/nauts/auth/identity"
	"github.com/msimon/nauts/auth/model"
)

// UsernamePassword is the identity token type for the static provider.
type UsernamePassword struct {
	Username string
	Password string
}

// staticUser represents a user stored in the JSON file.
type staticUser struct {
	Account      string            `json:"account"`
	Groups       []string          `json:"groups"`
	PasswordHash string            `json:"passwordHash"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// usersFile represents the JSON file structure.
type usersFile struct {
	Users map[string]*staticUser `json:"users"`
}

// StaticProvider implements identity.UserIdentityProvider using a JSON file.
type StaticProvider struct {
	users map[string]*staticUser
}

// Config holds configuration for StaticProvider.
type Config struct {
	// UsersPath is the path to the users JSON file.
	UsersPath string
}

// New creates a new StaticProvider from the given configuration.
func New(cfg Config) (*StaticProvider, error) {
	sp := &StaticProvider{
		users: make(map[string]*staticUser),
	}

	if cfg.UsersPath != "" {
		if err := sp.loadUsers(cfg.UsersPath); err != nil {
			return nil, err
		}
	}

	return sp, nil
}

// loadUsers loads users from a JSON file.
func (sp *StaticProvider) loadUsers(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var file usersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}

	sp.users = file.Users
	return nil
}

// Verify validates UsernamePassword token and returns the user.
// Returns identity.ErrInvalidTokenType if token is not UsernamePassword.
// Returns identity.ErrUserNotFound if the user does not exist.
// Returns identity.ErrInvalidCredentials if the password is incorrect.
func (sp *StaticProvider) Verify(_ context.Context, token identity.IdentityToken) (*model.User, error) {
	creds, ok := token.(UsernamePassword)
	if !ok {
		return nil, identity.ErrInvalidTokenType
	}

	su, ok := sp.users[creds.Username]
	if !ok {
		return nil, identity.ErrUserNotFound
	}

	// Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(su.PasswordHash), []byte(creds.Password)); err != nil {
		return nil, identity.ErrInvalidCredentials
	}

	return sp.toModelUser(creds.Username, su), nil
}

// GetUser retrieves a user by ID without verification.
func (sp *StaticProvider) GetUser(_ context.Context, userID string) (*model.User, error) {
	su, ok := sp.users[userID]
	if !ok {
		return nil, identity.ErrUserNotFound
	}

	return sp.toModelUser(userID, su), nil
}

// toModelUser converts a staticUser to a model.User.
func (sp *StaticProvider) toModelUser(id string, su *staticUser) *model.User {
	return &model.User{
		ID:         id,
		Account:    su.Account,
		Groups:     su.Groups,
		Attributes: su.Attributes,
	}
}
