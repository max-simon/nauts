// Package auth provides the authentication controller for nauts.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
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
	roleProvider    provider.RoleProvider
	policyProvider  provider.PolicyProvider
	authProvider    identity.AuthenticationProvider
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
	roleProvider provider.RoleProvider,
	policyProvider provider.PolicyProvider,
	authProvider identity.AuthenticationProvider,
	opts ...ControllerOption,
) *AuthController {
	c := &AuthController{
		accountProvider: accountProvider,
		roleProvider:    roleProvider,
		policyProvider:  policyProvider,
		authProvider:    authProvider,
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

// ResolveUser verifies the identity token and returns the user.
// The token should be a JSON object with the structure: { "account"?: string, "token": string }
// Returns identity.ErrInvalidCredentials if the credentials are invalid.
// Returns identity.ErrUserNotFound if the user does not exist.
// Returns identity.ErrInvalidTokenType if the token type is wrong for the provider.
// Returns identity.ErrInvalidAccount if the requested account is not valid for the user.
// Returns identity.ErrAccountRequired if user has multiple accounts but no account was specified.
func (c *AuthController) ResolveUser(ctx context.Context, token string) (*identity.User, error) {
	authReq, err := parseAuthRequest(token)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to parse auth request", err)
	}

	user, err := c.authProvider.Verify(ctx, authReq)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to verify identity", err)
	}
	return user, nil
}

// parseAuthRequest parses the JSON token into an AuthRequest.
// Expected format: { "account"?: string, "token": string }
func parseAuthRequest(token string) (identity.AuthRequest, error) {
	var req identity.AuthRequest
	if err := json.Unmarshal([]byte(token), &req); err != nil {
		return identity.AuthRequest{}, err
	}
	if req.Token == "" {
		return identity.AuthRequest{}, errors.New("token field is required")
	}
	return req, nil
}

// ResolveNatsPermissions compiles NATS permissions for a user based on their roles and policies.
// The compilation process:
// 1. Resolve user's roles (always include "default")
// 2. For each role name, fetch both global and local roles and compile policies
// 3. Deduplicate and return permissions
func (c *AuthController) ResolveNatsPermissions(ctx context.Context, user *identity.User) (*policy.NatsPermissions, error) {
	if user == nil {
		return nil, NewAuthError("", "resolve_permissions", "user is nil", nil)
	}

	// Collect all role names (including default)
	roleNames := c.collectRoleNames(user)

	result := policy.NewNatsPermissions()

	// Convert user to context once
	userCtx := userToContext(user)

	// Process each role name
	for _, roleName := range roleNames {
		globalRole, localRole, err := c.roleProvider.GetRoles(ctx, roleName, user.Account)
		if err != nil {
			if errors.Is(err, provider.ErrRoleNotFound) {
				c.logger.Warn("role not found: %s (user: %s)", roleName, user.ID)
				continue
			}
			return nil, NewAuthError(user.ID, "resolve_permissions", err.Error(), err)
		}

		// Process global role if it exists
		if globalRole != nil {
			c.compileRolePolicies(ctx, user, userCtx, globalRole, result)
		}

		// Process local role if it exists
		if localRole != nil {
			c.compileRolePolicies(ctx, user, userCtx, localRole, result)
		}
	}

	// Deduplicate
	result.Deduplicate()

	return result, nil
}

// compileRolePolicies compiles policies for a single role.
func (c *AuthController) compileRolePolicies(ctx context.Context, user *identity.User, userCtx *policy.UserContext, role *provider.Role, result *policy.NatsPermissions) {
	// Collect policies for this role
	var policies []*policy.Policy
	for _, policyID := range role.Policies {
		pol, err := c.policyProvider.GetPolicy(ctx, policyID)
		if err != nil {
			if errors.Is(err, provider.ErrPolicyNotFound) {
				c.logger.Warn("policy not found: %s (role: %s)", policyID, role.Name)
				continue
			}
			c.logger.Warn("error fetching policy %s: %v", policyID, err)
			continue
		}
		policies = append(policies, pol)
	}

	// Compile policies with role context
	roleCtx := roleToContext(role)
	compileResult := policy.Compile(policies, userCtx, roleCtx, result)

	// Log any warnings
	for _, warning := range compileResult.Warnings {
		c.logger.Warn("%s", warning)
	}
}

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	User          *identity.User
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
	user *identity.User,
	userPublicKey string,
	permissions *policy.NatsPermissions,
	ttl time.Duration,
) (string, error) {
	if user == nil {
		return "", NewAuthError("", "create_jwt", "user is nil", nil)
	}

	// Get the account from the account provider
	account, err := c.accountProvider.GetAccount(ctx, user.Account)
	if err != nil {
		return "", NewAuthError(user.ID, "create_jwt", "failed to get account", err)
	}

	// Determine audience based on operator mode
	// In operator mode, don't set audience (account determined by auth response's IssuerAccount)
	// In non-operator mode, set audience to account name
	audienceAccount := ""
	if !c.accountProvider.IsOperatorMode() {
		audienceAccount = user.Account
	}

	// In operator mode, the issuerAccount has to be set to support signing keys
	issuerAccount := ""
	if c.accountProvider.IsOperatorMode() {
		issuerAccount = account.PublicKey()
	}

	// Issue the JWT using the account's signer
	token, err := jwt.IssueUserJWT(user.ID, userPublicKey, ttl, permissions, account.Signer(), audienceAccount, issuerAccount)
	if err != nil {
		return "", NewAuthError(user.ID, "create_jwt", "failed to issue JWT", err)
	}

	return token, nil
}

// collectRoleNames returns all role names for a user, always including "default".
func (c *AuthController) collectRoleNames(user *identity.User) []string {
	seen := make(map[string]bool)
	var roles []string

	// Always include default role first
	if !seen[provider.DefaultRoleName] {
		seen[provider.DefaultRoleName] = true
		roles = append(roles, provider.DefaultRoleName)
	}

	// Add user's roles
	for _, r := range user.Roles {
		if !seen[r] {
			seen[r] = true
			roles = append(roles, r)
		}
	}

	return roles
}

// userToContext converts an identity.User to a policy.UserContext for interpolation.
func userToContext(user *identity.User) *policy.UserContext {
	if user == nil {
		return nil
	}
	return &policy.UserContext{
		ID:         user.ID,
		Account:    user.Account,
		Attributes: user.Attributes,
	}
}

// roleToContext converts a provider.Role to a policy.RoleContext for interpolation.
func roleToContext(role *provider.Role) *policy.RoleContext {
	if role == nil {
		return nil
	}
	return &policy.RoleContext{
		Name:    role.Name,
		Account: role.Account,
	}
}
