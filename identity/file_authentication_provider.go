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

// FileAuthenticationProvider implements AuthenticationProvider using a JSON file.
type FileAuthenticationProvider struct {
	users              map[string]*fileUser
	manageableAccounts []string
}

// FileAuthenticationProviderConfig holds configuration for FileAuthenticationProvider.
type FileAuthenticationProviderConfig struct {
	// UsersPath is the path to the users JSON file.
	UsersPath string
	// Accounts is the list of NATS accounts this provider can manage.
	// Patterns support wildcards in the form of "*" (all) or "prefix*".
	Accounts []string
}

// NewFileAuthenticationProvider creates a new FileAuthenticationProvider from the given configuration.
func NewFileAuthenticationProvider(cfg FileAuthenticationProviderConfig) (*FileAuthenticationProvider, error) {
	fp := &FileAuthenticationProvider{
		users:              make(map[string]*fileUser),
		manageableAccounts: append([]string(nil), cfg.Accounts...),
	}

	if cfg.UsersPath != "" {
		if err := fp.loadUsers(cfg.UsersPath); err != nil {
			return nil, err
		}
	}

	return fp, nil
}

func (fp *FileAuthenticationProvider) ManageableAccounts() []string {
	return append([]string(nil), fp.manageableAccounts...)
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

	// Validate requested account is in user's accounts list
	if !contains(fu.Accounts, req.Account) {
		return nil, ErrInvalidAccount
	}

	// Parse all roles as AccountRole objects (no filtering by account)
	// Account filtering will be done by the AuthController
	var roles []AccountRole
	for _, roleID := range fu.Roles {
		role, err := ParseRoleID(roleID)
		if err != nil {
			// Skip invalid role IDs
			continue
		}
		roles = append(roles, role)
	}

	return &User{
		ID:         creds.Username,
		Roles:      roles,
		Attributes: fu.Attributes,
	}, nil
}

// contains checks if a string slice contains a specific value.
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// GetUser retrieves a user by ID without verifying credentials.
// Returns ErrUserNotFound if the user does not exist.
// Note: This returns all roles for all accounts the user has access to.
func (fp *FileAuthenticationProvider) GetUser(_ context.Context, id string) (*User, error) {
	fu, ok := fp.users[id]
	if !ok {
		return nil, ErrUserNotFound
	}

	// Parse all roles as AccountRole objects
	var roles []AccountRole
	for _, roleID := range fu.Roles {
		role, err := ParseRoleID(roleID)
		if err != nil {
			// Skip invalid role IDs
			continue
		}
		roles = append(roles, role)
	}

	return &User{
		ID:         id,
		Roles:      roles,
		Attributes: fu.Attributes,
	}, nil
}
