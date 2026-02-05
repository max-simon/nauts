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
	accountProvider         provider.AccountProvider
	roleProvider            provider.RoleProvider
	policyProvider          provider.PolicyProvider
	authenticationProviders []identity.AuthenticationProvider
	logger                  Logger
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
	authenticationProviders []identity.AuthenticationProvider,
	opts ...ControllerOption,
) *AuthController {
	c := &AuthController{
		accountProvider:         accountProvider,
		roleProvider:            roleProvider,
		policyProvider:          policyProvider,
		authenticationProviders: authenticationProviders,
		logger:                  &defaultLogger{},
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

// selectAuthenticationProvider selects the appropriate provider for the account.
// Returns an error if no provider matches or if multiple providers match without explicit selection.
func (c *AuthController) selectAuthenticationProvider(account, providerID string) (identity.AuthenticationProvider, error) {
	var candidates []identity.AuthenticationProvider

	for _, p := range c.authenticationProviders {
		if p.CanManageAccount(account) {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		return nil, errors.New("no authentication provider found for account \"" + account + "\"")
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	// Multiple candidates - require explicit provider ID
	if providerID == "" {
		return nil, errors.New("multiple authentication providers available for account \"" + account + "\", provider ID required")
	}

	for _, p := range candidates {
		if p.ID() == providerID {
			return p, nil
		}
	}

	return nil, errors.New("authentication provider \"" + providerID + "\" not found or not authorized for account \"" + account + "\"")
}

// filterRolesForAccount filters user roles to only include those for the target account.
// Only keeps AccountRole objects where Account matches targetAccount.
// Returns filtered AccountRole objects (preserves account associations).
func filterRolesForAccount(roles []identity.AccountRole, targetAccount string) []identity.AccountRole {
	var filtered []identity.AccountRole
	seen := make(map[string]bool) // deduplicate by role ID

	for _, r := range roles {
		// Only include roles for the target account
		if r.Account == targetAccount {
			roleID := r.String() // "<account>.<role>"
			if !seen[roleID] {
				seen[roleID] = true
				filtered = append(filtered, r)
			}
		}
	}

	return filtered
}

// parseAuthRequestWithProvider parses the JSON token and extracts optional provider ID.
func parseAuthRequestWithProvider(token string) (identity.AuthRequest, string, error) {
	var req struct {
		Account string `json:"account"`
		Token   string `json:"token"`
		AP      string `json:"ap"` // Optional authentication provider ID
	}

	if err := json.Unmarshal([]byte(token), &req); err != nil {
		return identity.AuthRequest{}, "", err
	}

	if req.Token == "" {
		return identity.AuthRequest{}, "", errors.New("token field is required")
	}

	return identity.AuthRequest{
		Account: req.Account,
		Token:   req.Token,
	}, req.AP, nil
}

// ResolveUser verifies the identity token and returns the user with filtered roles.
// The token should be a JSON object: { "account": "...", "token": "...", "ap"?: "..." }
// The "account" field is required for provider selection and role filtering.
// The "ap" field is optional and used to explicitly select an authentication provider.
// Returns identity.ErrInvalidCredentials if the credentials are invalid.
// Returns identity.ErrUserNotFound if the user does not exist.
// Returns identity.ErrInvalidTokenType if the token type is wrong for the provider.
// Returns identity.ErrAccountRequired if no account was specified.
func (c *AuthController) ResolveUser(ctx context.Context, token string) (*identity.User, error) {
	// Parse auth request
	authReq, providerID, err := parseAuthRequestWithProvider(token)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to parse auth request", err)
	}

	if authReq.Account == "" {
		return nil, NewAuthError("", "resolve_user", "account is required", identity.ErrAccountRequired)
	}

	// Select provider
	provider, err := c.selectAuthenticationProvider(authReq.Account, providerID)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", err.Error(), err)
	}

	// Verify credentials - returns User with all roles as AccountRole objects
	user, err := provider.Verify(ctx, authReq)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "authentication failed", err)
	}

	// Filter roles for target account (authorization step)
	// Only includes AccountRole objects where Account == authReq.Account
	filteredRoles := filterRolesForAccount(user.Roles, authReq.Account)

	// Return user with filtered roles
	return &identity.User{
		ID:         user.ID,
		Roles:      filteredRoles,
		Attributes: user.Attributes,
	}, nil
}

// parseAuthRequest parses the JSON token into an AuthRequest.
// Expected format: { "account"?: string, "token": string }
// Deprecated: Use parseAuthRequestWithProvider instead.
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
// The user must have been filtered by account (via ResolveUser), so all AccountRole objects
// have the same account value. We use the account from the first role (or require it as a parameter).
// The compilation process:
// 1. Extract account from user's roles (or use provided account parameter)
// 2. Process "default" role for the account
// 3. For each AccountRole, fetch both global and local roles and compile policies
// 4. Deduplicate and return permissions
func (c *AuthController) ResolveNatsPermissions(ctx context.Context, user *identity.User, account string) (*policy.NatsPermissions, error) {
	if user == nil {
		return nil, NewAuthError("", "resolve_permissions", "user is nil", nil)
	}

	result := policy.NewNatsPermissions()

	// Convert user to context once
	userCtx := &policy.UserContext{
		ID:         user.ID,
		Account:    account,
		Attributes: user.Attributes,
	}

	// Always include default role
	c.processRole(ctx, "default", account, user, userCtx, result)

	// Process user's roles
	seen := make(map[string]bool)
	seen["default"] = true

	for _, accountRole := range user.Roles {
		roleKey := accountRole.String()
		if seen[roleKey] {
			continue
		}
		seen[roleKey] = true

		// Call GetRoles with role name and account from AccountRole
		c.processRole(ctx, accountRole.Role, accountRole.Account, user, userCtx, result)
	}

	// Deduplicate
	result.Deduplicate()

	return result, nil
}

// processRole fetches and compiles policies for a single role.
func (c *AuthController) processRole(ctx context.Context, roleName, account string, user *identity.User, userCtx *policy.UserContext, result *policy.NatsPermissions) {
	globalRole, localRole, err := c.roleProvider.GetRoles(ctx, roleName, account)
	if err != nil {
		if errors.Is(err, provider.ErrRoleNotFound) {
			c.logger.Warn("role not found: %s (account: %s, user: %s)", roleName, account, user.ID)
			return
		}
		c.logger.Warn("error fetching role %s: %v", roleName, err)
		return
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
	Account       string
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
//   - connectOptions: NATS connect options containing the token
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

	// Extract account from the resolved user's roles (after filtering, all roles have the same account)
	account := ""
	if len(user.Roles) > 0 {
		account = user.Roles[0].Account
	} else {
		// If no roles, we need to parse the account from the token
		authReq, _, err := parseAuthRequestWithProvider(connectOptions.Token)
		if err != nil {
			return nil, NewAuthError("", "authenticate", "failed to parse auth request", err)
		}
		account = authReq.Account
	}

	// Step 2: Resolve permissions
	permissions, err := c.ResolveNatsPermissions(ctx, user, account)
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
	jwtToken, err := c.CreateUserJWT(ctx, user, account, userPublicKey, permissions, ttl)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:          user,
		Account:       account,
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
//   - account: the NATS account name
//   - userPublicKey: the user's public key (subject of the JWT)
//   - permissions: NATS permissions to embed in the JWT
//   - ttl: time-to-live for the JWT (0 means no expiry)
func (c *AuthController) CreateUserJWT(
	ctx context.Context,
	user *identity.User,
	account string,
	userPublicKey string,
	permissions *policy.NatsPermissions,
	ttl time.Duration,
) (string, error) {
	if user == nil {
		return "", NewAuthError("", "create_jwt", "user is nil", nil)
	}

	// Get the account from the account provider
	accountObj, err := c.accountProvider.GetAccount(ctx, account)
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
		issuerAccount = accountObj.PublicKey()
	}

	// Issue the JWT using the account's signer
	token, err := jwt.IssueUserJWT(user.ID, userPublicKey, ttl, permissions, accountObj.Signer(), audienceAccount, issuerAccount)
	if err != nil {
		return "", NewAuthError(user.ID, "create_jwt", "failed to issue JWT", err)
	}

	return token, nil
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
