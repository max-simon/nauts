// Package auth provides the authentication controller for nauts.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/jwt"
	"github.com/msimon/nauts/policy"
	"github.com/msimon/nauts/provider"
)

// Logger is an interface for logging during authentication.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
}

// defaultLogger wraps the standard log package.
type defaultLogger struct{}

func (l *defaultLogger) Info(msg string, args ...any) {
	log.Printf("INFO: "+msg, args...)
}

func (l *defaultLogger) Warn(msg string, args ...any) {
	log.Printf("WARN: "+msg, args...)
}

func (l *defaultLogger) Debug(msg string, args ...any) {
	log.Printf("DEBUG: "+msg, args...)
}

// AuthController orchestrates user authentication, permission compilation, and JWT issuance.
type AuthController struct {
	accountProvider provider.AccountProvider
	policyProvider  provider.PolicyProvider
	authProviders   *identity.AuthenticationProviderManager
	logger          Logger
}

// ControllerOption configures an AuthController.
type ControllerOption func(*AuthController)

// WithLogger sets a custom logger for the controller.
func WithLogger(l Logger) ControllerOption {
	return func(c *AuthController) {
		c.logger = l
	}
}

// NewAuthController creates a new AuthController with the given providers.
func NewAuthController(
	accountProvider provider.AccountProvider,
	policyProvider provider.PolicyProvider,
	authProviders *identity.AuthenticationProviderManager,
	opts ...ControllerOption,
) *AuthController {
	c := &AuthController{
		accountProvider: accountProvider,
		policyProvider:  policyProvider,
		authProviders:   authProviders,
		logger:          &defaultLogger{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// AccountProvider returns the account provider used by this controller.
func (c *AuthController) AccountProvider() provider.AccountProvider {
	return c.accountProvider
}

// ResolveUser verifies the identity token and returns the user scoped to a single account.
// The token should be a JSON object with the structure: { "account": string, "token": string }
// Returns identity.ErrInvalidCredentials if the credentials are invalid.
// Returns identity.ErrUserNotFound if the user does not exist.
// Returns identity.ErrInvalidTokenType if the token type is wrong for the provider.
// Returns identity.ErrInvalidAccount if the requested account is not valid for the user.
func (c *AuthController) ResolveUser(ctx context.Context, token string) (*AccountScopedUser, error) {
	authReq, err := parseAuthRequest(token)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to parse auth request", err)
	}
	if strings.Contains(authReq.Account, "*") {
		return nil, NewAuthError("", "resolve_user", "account must not contain wildcards", nil)
	}

	user, err := c.authProviders.Verify(ctx, authReq)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to verify identity", err)
	}

	// Validate that roles do not contain wildcards
	for _, role := range user.Roles {
		if strings.Contains(role.Account, "*") || strings.Contains(role.Role, "*") {
			return nil, NewAuthError(user.ID, "resolve_user", "invalid role: wildcards not allowed", nil)
		}
	}

	// Filter user roles to only include those for the requested account
	// This is the authorization step - separating it from authentication
	filteredRoles := make([]identity.AccountRole, 0, len(user.Roles))
	for _, role := range user.Roles {
		if role.Account == authReq.Account {
			filteredRoles = append(filteredRoles, role)
		}
	}

	scoped := &AccountScopedUser{
		User:    *user,
		Account: authReq.Account,
	}
	scoped.Roles = filteredRoles
	return scoped, nil
}

// parseAuthRequest parses the JSON token into an AuthRequest.
// Expected format: { "account": string, "token": string }
func parseAuthRequest(token string) (identity.AuthRequest, error) {
	var req identity.AuthRequest
	if err := json.Unmarshal([]byte(token), &req); err != nil {
		return identity.AuthRequest{}, err
	}
	if req.Token == "" {
		return identity.AuthRequest{}, errors.New("token field is required")
	}
	if req.Account == "" {
		return identity.AuthRequest{}, errors.New("account field is required")
	}
	return req, nil
}

// ResolveNatsPermissions compiles NATS permissions for a user based on their roles and policies.
// The compilation process:
// 1. Resolve user's roles (always include "default")
// 2. For each role name, fetch both global and local roles and compile policies
// 3. Deduplicate and return permissions
func (c *AuthController) ResolveNatsPermissions(ctx context.Context, user *AccountScopedUser) (*policy.NatsPermissions, error) {
	if user == nil {
		return nil, NewAuthError("", "resolve_permissions", "user is nil", nil)
	}

	account := user.Account

	// Collect all role names (including default)
	roleNames := c.collectRoleNames(user)

	result := policy.NewNatsPermissions()

	// Convert user to context once
	userCtx := userToContext(user)

	// Process each role name
	for _, roleName := range roleNames {
		policies, err := c.policyProvider.GetPoliciesForRole(ctx, account, roleName)
		if err != nil {
			if errors.Is(err, provider.ErrRoleNotFound) {
				c.logger.Warn("role not found: %s (user: %s)", roleName, user.ID)
				continue
			}
			return nil, NewAuthError(user.ID, "resolve_permissions", err.Error(), err)
		}

		c.compilePoliciesForRole(userCtx, account, roleName, policies, result)
	}

	// Deduplicate
	result.Deduplicate()

	return result, nil
}

func (c *AuthController) compilePoliciesForRole(userCtx *policy.UserContext, account string, roleName string, policies []*policy.Policy, result *policy.NatsPermissions) {
	roleCtx := &policy.RoleContext{Name: roleName, Account: account}
	compileResult := policy.Compile(policies, userCtx, roleCtx, result)
	for _, warning := range compileResult.Warnings {
		c.logger.Warn("%s", warning)
	}
}

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	User          *AccountScopedUser
	UserPublicKey string
	Permissions   *policy.NatsPermissions
	JWT           string
}

// Authenticate performs the complete authentication flow:
// 1. Verifies the identity token to get the user
// 2. Compiles NATS permissions for the user
// 3. Creates a signed JWT with the permissions
//
// Parameters:
//   - ctx: context for the operation
//   - token: the identity token to verify
//   - userPublicKey: the user's public key (subject of the JWT). If empty, an ephemeral key is generated.
//   - ttl: time-to-live for the JWT (0 means no expiry)
func (c *AuthController) Authenticate(
	ctx context.Context,
	connectOptions natsjwt.ConnectOptions,
	userPublicKey string,
	ttl time.Duration,
) (*AuthResult, error) {
	// Step 1: Resolve user
	user, err := c.ResolveUser(ctx, connectOptions.Token)
	if err != nil {
		return nil, err
	}

	// Step 2: Resolve permissions
	permissions, err := c.ResolveNatsPermissions(ctx, user)
	if err != nil {
		return nil, err
	}

	c.logger.Debug(fmt.Sprintf("Permissions for user %s: %s", user.ID, permissions.String()))

	// Step 3: Generate ephemeral key if not provided
	if userPublicKey == "" {
		userPublicKey, err = generateEphemeralUserKey()
		if err != nil {
			return nil, NewAuthError(user.ID, "authenticate", "failed to generate ephemeral key", err)
		}
	}

	// Step 4: Create JWT
	jwtToken, err := c.CreateUserJWT(ctx, user, userPublicKey, permissions, ttl)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:          user,
		UserPublicKey: userPublicKey,
		Permissions:   permissions,
		JWT:           jwtToken,
	}, nil
}

// generateEphemeralUserKey creates a new ephemeral user keypair and returns the public key.
func generateEphemeralUserKey() (string, error) {
	kp, err := nkeys.CreateUser()
	if err != nil {
		return "", err
	}
	return kp.PublicKey()
}

// CreateUserJWT creates a signed JWT for the user with the given permissions.
// The JWT is signed by the account's signer retrieved from the AccountProvider.
// Parameters:
//   - ctx: context for the operation
//   - user: the user to create the JWT for
//   - userPublicKey: the user's public key (subject of the JWT)
//   - permissions: NATS permissions to embed in the JWT
//   - ttl: time-to-live for the JWT (0 means no expiry)
func (c *AuthController) CreateUserJWT(
	ctx context.Context,
	user *AccountScopedUser,
	userPublicKey string,
	permissions *policy.NatsPermissions,
	ttl time.Duration,
) (string, error) {
	if user == nil {
		return "", NewAuthError("", "create_jwt", "user is nil", nil)
	}

	account := user.Account

	// Get the account from the account provider
	accountEntity, err := c.accountProvider.GetAccount(ctx, account)
	if err != nil {
		return "", NewAuthError(user.ID, "create_jwt", "failed to get account", err)
	}

	// Determine audience based on operator mode
	// In operator mode, don't set audience (account determined by auth response's IssuerAccount)
	// In non-operator mode, set audience to account name
	audienceAccount := ""
	if !c.accountProvider.IsOperatorMode() {
		audienceAccount = account
	}

	// In operator mode, the issuerAccount has to be set to support signing keys
	issuerAccount := ""
	if c.accountProvider.IsOperatorMode() {
		issuerAccount = accountEntity.PublicKey()
	}

	// Issue the JWT using the account's signer
	token, err := jwt.IssueUserJWT(user.ID, userPublicKey, ttl, permissions, accountEntity.Signer(), audienceAccount, issuerAccount)
	if err != nil {
		return "", NewAuthError(user.ID, "create_jwt", "failed to issue JWT", err)
	}

	return token, nil
}

// DefaultRoleName is the implicit role applied to every user.
const DefaultRoleName = "default"

// collectRoleNames returns all role names for a user, always including "default".
func (c *AuthController) collectRoleNames(user *AccountScopedUser) []string {
	seen := make(map[string]bool)
	var roles []string

	// Always include default role first
	if !seen[DefaultRoleName] {
		seen[DefaultRoleName] = true
		roles = append(roles, DefaultRoleName)
	}

	// Add user's roles (extract role name from AccountRole)
	for _, r := range user.Roles {
		if !seen[r.Role] {
			seen[r.Role] = true
			roles = append(roles, r.Role)
		}
	}

	return roles
}

// userToContext converts an AccountScopedUser to a policy.UserContext for interpolation.
func userToContext(user *AccountScopedUser) *policy.UserContext {
	if user == nil {
		return nil
	}
	return &policy.UserContext{
		ID:         user.ID,
		Account:    user.Account,
		Attributes: user.Attributes,
	}
}
