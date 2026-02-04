// Package auth provides the authentication controller for nauts.
package auth

import (
	"context"
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
	accountProvider  provider.AccountProvider
	groupProvider    provider.GroupProvider
	policyProvider   provider.PolicyProvider
	identityProvider identity.UserIdentityProvider
	logger           Logger
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
	groupProvider provider.GroupProvider,
	policyProvider provider.PolicyProvider,
	identityProvider identity.UserIdentityProvider,
	opts ...ControllerOption,
) *AuthController {
	c := &AuthController{
		accountProvider:  accountProvider,
		groupProvider:    groupProvider,
		policyProvider:   policyProvider,
		identityProvider: identityProvider,
		logger:           &defaultLogger{},
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
// Returns identity.ErrInvalidCredentials if the credentials are invalid.
// Returns identity.ErrUserNotFound if the user does not exist.
// Returns identity.ErrInvalidTokenType if the token type is wrong for the provider.
func (c *AuthController) ResolveUser(ctx context.Context, token string) (*identity.User, error) {
	user, err := c.identityProvider.Verify(ctx, token)
	if err != nil {
		return nil, NewAuthError("", "resolve_user", "failed to verify identity", err)
	}
	return user, nil
}

// ResolveNatsPermissions compiles NATS permissions for a user based on their groups and policies.
// The compilation process:
// 1. Resolve user's groups (always include "default")
// 2. For each group, fetch policies and compile with that group's context
// 3. Deduplicate and return permissions
func (c *AuthController) ResolveNatsPermissions(ctx context.Context, user *identity.User) (*policy.NatsPermissions, error) {
	if user == nil {
		return nil, NewAuthError("", "resolve_permissions", "user is nil", nil)
	}

	// Collect all groups (including default)
	groupIDs := c.collectGroups(user)

	result := policy.NewNatsPermissions()

	// Convert user to context once
	userCtx := userToContext(user)

	// Process each group
	for _, groupID := range groupIDs {
		group, err := c.groupProvider.GetGroup(ctx, groupID)
		if err != nil {
			if errors.Is(err, provider.ErrGroupNotFound) {
				c.logger.Warn("group not found: %s (user: %s)", groupID, user.ID)
				continue
			}
			return nil, NewAuthError(user.ID, "resolve_permissions", err.Error(), err)
		}

		// Collect policies for this group
		var policies []*policy.Policy
		for _, policyID := range group.Policies {
			pol, err := c.policyProvider.GetPolicy(ctx, policyID)
			if err != nil {
				if errors.Is(err, provider.ErrPolicyNotFound) {
					c.logger.Warn("policy not found: %s (group: %s)", policyID, groupID)
					continue
				}
				return nil, NewAuthError(user.ID, "resolve_permissions", err.Error(), err)
			}
			policies = append(policies, pol)
		}

		// Compile policies with group context
		groupCtx := groupToContext(group)
		compileResult := policy.Compile(policies, userCtx, groupCtx, result)

		// Log any warnings
		for _, warning := range compileResult.Warnings {
			c.logger.Warn("%s", warning)
		}
	}

	// Deduplicate
	result.Deduplicate()

	return result, nil
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

// collectGroups returns all group IDs for a user, always including "default".
func (c *AuthController) collectGroups(user *identity.User) []string {
	seen := make(map[string]bool)
	var groups []string

	// Always include default group first
	if !seen[provider.DefaultGroupID] {
		seen[provider.DefaultGroupID] = true
		groups = append(groups, provider.DefaultGroupID)
	}

	// Add user's groups
	for _, g := range user.Groups {
		if !seen[g] {
			seen[g] = true
			groups = append(groups, g)
		}
	}

	return groups
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

// groupToContext converts a provider.Group to a policy.GroupContext for interpolation.
func groupToContext(group *provider.Group) *policy.GroupContext {
	if group == nil {
		return nil
	}
	return &policy.GroupContext{
		ID:   group.ID,
		Name: group.Name,
	}
}
