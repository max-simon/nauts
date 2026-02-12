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

func (c *AuthController) ScopeUserToAccount(ctx context.Context, user *identity.User, account string) (*AccountScopedUser, error) {
	// Filter user roles to only include those for the requested account
	// This is the authorization step - separating it from authentication
	filteredRoles := make([]identity.Role, 0, len(user.Roles))
	for _, role := range user.Roles {
		if strings.Contains(role.Account, "*") || strings.Contains(role.Name, "*") {
			return nil, NewAuthError(user.ID, "resolve_user", "invalid role: wildcards not allowed", nil)
		}
		if role.Account == account {
			filteredRoles = append(filteredRoles, role)
		}
	}
	scoped := &AccountScopedUser{
		User:    *user,
		Account: account,
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
	if strings.Contains(req.Account, "*") {
		return identity.AuthRequest{}, errors.New("account must not contain wildcards")
	}
	return req, nil
}

type NautsCompilationResult struct {
	User           *AccountScopedUser
	Permissions    *policy.NatsPermissions
	PermissionsRaw *policy.NatsPermissions
	Warnings       []string
	Roles          []identity.Role
	Policies       map[string][]*policy.Policy
}

// CompileNatsPermissions compiles NATS permissions for a given user.
func (c *AuthController) CompileNatsPermissions(ctx context.Context, user *AccountScopedUser) (*NautsCompilationResult, error) {
	if user == nil {
		return nil, NewAuthError("", "resolve_permissions", "user is nil", nil)
	}

	roles := c.collectRoles(user)
	compiled := policy.NewNatsPermissions()
	basePolicyCtx := userToPolicyContext(user)

	warnings := make([]string, 0)
	policiesByRole := make(map[string][]*policy.Policy, len(roles))

	for _, role := range roles {
		policies, err := c.policyProvider.GetPoliciesForRole(ctx, role)
		if err != nil {
			if errors.Is(err, provider.ErrRoleNotFound) {
				warnings = append(warnings, fmt.Sprintf("role not found: %s.%s (user: %s)", role.Account, role.Name, user.ID))
				policiesByRole[role.Account+"."+role.Name] = []*policy.Policy{}
				continue
			}
			return nil, NewAuthError(user.ID, "resolve_permissions", err.Error(), err)
		}
		policiesByRole[role.Account+"."+role.Name] = policies

		ctxCopy := basePolicyCtx.Clone()
		if ctxCopy == nil {
			ctxCopy = &policy.PolicyContext{}
		}
		ctxCopy.Role = role.Name
		compileResult := policy.Compile(policies, ctxCopy, compiled)
		if len(compileResult.Warnings) > 0 {
			warnings = append(warnings, compileResult.Warnings...)
		}
	}

	preDedup := compiled.Clone()
	postDedup := compiled.Clone()
	if postDedup != nil {
		postDedup.Deduplicate()
	}

	return &NautsCompilationResult{
		User:           user,
		Permissions:    postDedup,
		PermissionsRaw: preDedup,
		Warnings:       warnings,
		Roles:          roles,
		Policies:       policiesByRole,
	}, nil
}

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	User              *AccountScopedUser
	UserPublicKey     string
	CompilationResult *NautsCompilationResult
	AuthProviderId    string
	JWT               string
}

// Authenticate performs the complete authentication flow
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
	// Step 1: Parse AuthRequest
	authReq, err := parseAuthRequest(connectOptions.Token)
	if err != nil {
		return nil, err
	}

	// Step 2: select auth provider
	providerID, provider, err := c.authProviders.SelectProvider(authReq)
	if err != nil {
		return nil, err
	}

	// Step 3: Verify user
	user, err := provider.Verify(ctx, authReq)
	if err != nil {
		return nil, err
	}

	// Step 4: scope user to account
	userScoped, err := c.ScopeUserToAccount(ctx, user, authReq.Account)
	if err != nil {
		return nil, err
	}

	// Step 5: compile NATS permissions
	compilationResult, err := c.CompileNatsPermissions(ctx, userScoped)
	if err != nil {
		return nil, err
	}

	// Step 6: Generate ephemeral key if not provided
	if userPublicKey == "" {
		userPublicKey, err = generateEphemeralUserKey()
		if err != nil {
			return nil, NewAuthError(user.ID, "authenticate", "failed to generate ephemeral key", err)
		}
	}

	// Step 7: Create JWT
	jwtToken, err := c.CreateUserJWT(ctx, userScoped, userPublicKey, compilationResult.Permissions, ttl)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:              userScoped,
		UserPublicKey:     userPublicKey,
		CompilationResult: compilationResult,
		AuthProviderId:    providerID,
		JWT:               jwtToken,
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

// collectRoles returns all roles for a user, always including the default role.
func (c *AuthController) collectRoles(user *AccountScopedUser) []identity.Role {
	seen := make(map[string]bool)
	roles := make([]identity.Role, 0, 8)

	// Always include default role first.
	defaultRole := identity.Role{Account: user.Account, Name: DefaultRoleName}
	seen[defaultRole.Account+"."+defaultRole.Name] = true
	roles = append(roles, defaultRole)

	// Add user's roles.
	for _, r := range user.Roles {
		key := r.Account + "." + r.Name
		if seen[key] {
			continue
		}
		seen[key] = true
		roles = append(roles, r)
	}

	return roles
}

// userToPolicyContext converts an AccountScopedUser to a policy.PolicyContext for policy compilation.
func userToPolicyContext(user *AccountScopedUser) *policy.PolicyContext {
	if user == nil {
		return nil
	}
	ctx := &policy.PolicyContext{User: user.ID, Account: user.Account}
	if len(user.Attributes) == 0 {
		return ctx
	}
	ctx.UserClaims = make(map[string]string, len(user.Attributes))
	for k, v := range user.Attributes {
		ctx.UserClaims[k] = v
	}
	return ctx
}
