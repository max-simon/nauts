// Package auth provides the authentication service for nauts.
package auth

import (
	"context"
	"errors"
	"log"

	"github.com/msimon/nauts/auth/model"
	"github.com/msimon/nauts/auth/provider"
	"github.com/msimon/nauts/policy"
)

// Logger is an interface for logging warnings during compilation.
type Logger interface {
	Warn(msg string, args ...any)
}

// defaultLogger wraps the standard log package.
type defaultLogger struct{}

func (l *defaultLogger) Warn(msg string, args ...any) {
	log.Printf("WARN: "+msg, args...)
}

// AuthService compiles permissions for users based on their groups and policies.
type AuthService struct {
	provider provider.GroupPolicyProvider
	logger   Logger
}

// ServiceOption configures an AuthService.
type ServiceOption func(*AuthService)

// WithLogger sets a custom logger for the service.
func WithLogger(l Logger) ServiceOption {
	return func(s *AuthService) {
		s.logger = l
	}
}

// NewAuthService creates a new AuthService with the given provider.
func NewAuthService(p provider.GroupPolicyProvider, opts ...ServiceOption) *AuthService {
	svc := &AuthService{
		provider: p,
		logger:   &defaultLogger{},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// GetNatsPermission generates NATS permissions for a user.
// The compilation process:
// 1. Resolve user's groups (always include "default")
// 2. For each group, fetch policies and compile with that group's context
// 3. Deduplicate and return permissions
func (s *AuthService) GetNatsPermission(ctx context.Context, user *model.User) (*policy.NatsPermissions, error) {
	if user == nil {
		return nil, NewAuthError("", "validate", "user is nil", nil)
	}

	// Collect all groups (including default)
	groupIDs := s.collectGroups(user)

	result := policy.NewNatsPermissions()

	// Convert user to context once
	userCtx := userToContext(user)

	// Process each group
	for _, groupID := range groupIDs {
		group, err := s.provider.GetGroup(ctx, groupID)
		if err != nil {
			if errors.Is(err, provider.ErrGroupNotFound) {
				s.logger.Warn("group not found: %s (user: %s)", groupID, user.ID)
				continue
			}
			return nil, NewAuthError(user.ID, "resolve groups", err.Error(), err)
		}

		// Collect policies for this group
		var policies []*policy.Policy
		for _, policyID := range group.Policies {
			pol, err := s.provider.GetPolicy(ctx, policyID)
			if err != nil {
				if errors.Is(err, provider.ErrPolicyNotFound) {
					s.logger.Warn("policy not found: %s (group: %s)", policyID, groupID)
					continue
				}
				return nil, NewAuthError(user.ID, "resolve policies", err.Error(), err)
			}
			policies = append(policies, pol)
		}

		// Compile policies with group context
		groupCtx := groupToContext(group)
		compileResult := policy.Compile(policies, userCtx, groupCtx, result)

		// Log any warnings
		for _, warning := range compileResult.Warnings {
			s.logger.Warn("%s", warning)
		}
	}

	// Deduplicate
	result.Deduplicate()

	return result, nil
}

// collectGroups returns all group IDs for a user, always including "default".
func (s *AuthService) collectGroups(user *model.User) []string {
	seen := make(map[string]bool)
	var groups []string

	// Always include default group first
	if !seen[model.DefaultGroupID] {
		seen[model.DefaultGroupID] = true
		groups = append(groups, model.DefaultGroupID)
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

// userToContext converts a model.User to a policy.UserContext for interpolation.
func userToContext(user *model.User) *policy.UserContext {
	if user == nil {
		return nil
	}
	return &policy.UserContext{
		ID:         user.ID,
		Account:    user.Account,
		Attributes: user.Attributes,
	}
}

// groupToContext converts a model.Group to a policy.GroupContext for interpolation.
func groupToContext(group *model.Group) *policy.GroupContext {
	if group == nil {
		return nil
	}
	return &policy.GroupContext{
		ID:   group.ID,
		Name: group.Name,
	}
}
