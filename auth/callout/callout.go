// Package callout provides the authentication callout service for NATS.
package callout

import (
	"context"

	"github.com/msimon/nauts/auth"
	"github.com/msimon/nauts/auth/identity"
	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/policy"
)

// AuthResult contains the result of a successful authentication.
type AuthResult struct {
	User        *model.User
	Permissions *policy.NatsPermissions
}

// CalloutService orchestrates user authentication and permission compilation.
type CalloutService struct {
	identityProvider identity.UserIdentityProvider
	authService      *auth.AuthService
	logger           auth.Logger
}

// Option configures a CalloutService.
type Option func(*CalloutService)

// WithLogger sets a custom logger for the service.
func WithLogger(l auth.Logger) Option {
	return func(s *CalloutService) {
		s.logger = l
	}
}

// NewCalloutService creates a new CalloutService.
func NewCalloutService(
	identityProvider identity.UserIdentityProvider,
	authService *auth.AuthService,
	opts ...Option,
) *CalloutService {
	s := &CalloutService{
		identityProvider: identityProvider,
		authService:      authService,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Authenticate verifies the identity token and compiles NATS permissions for the user.
// The flow is:
// 1. Verify the identity token to get the user
// 2. Compile NATS permissions for the user
// 3. Return the result with user and permissions
func (s *CalloutService) Authenticate(ctx context.Context, token identity.IdentityToken) (*AuthResult, error) {
	// Phase 1: Authenticate - verify identity token
	user, err := s.identityProvider.Verify(ctx, token)
	if err != nil {
		return nil, NewCalloutError("authenticate", "failed to verify identity", err)
	}

	// Phase 2: Authorize - compile permissions
	permissions, err := s.authService.GetNatsPermission(ctx, user)
	if err != nil {
		return nil, NewCalloutError("authorize", "failed to compile permissions", err)
	}

	return &AuthResult{
		User:        user,
		Permissions: permissions,
	}, nil
}
