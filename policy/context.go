// Package policy provides policy-related types and functions for nauts.
// This file contains context types for variable interpolation.
package policy

import "strings"

// Context provides values for variable interpolation.
// Implementations resolve variable paths like "user.id" or "role.name".
type Context interface {
	// HasAttribute returns true if the attribute exists.
	HasAttribute(path string) bool
	// GetAttribute returns the attribute value and whether it was found.
	GetAttribute(path string) (string, bool)
}

// UserContext contains user attributes needed for policy interpolation.
// Implements Context interface for user.* variables.
type UserContext struct {
	ID         string            // user identifier
	Account    string            // NATS account ID
	Attributes map[string]string // additional user attributes
}

// HasAttribute returns true if the user attribute exists.
func (u *UserContext) HasAttribute(path string) bool {
	_, ok := u.GetAttribute(path)
	return ok
}

// GetAttribute returns a user attribute value.
// Supported paths: "id", "account", "attr.<key>"
func (u *UserContext) GetAttribute(path string) (string, bool) {
	if u == nil {
		return "", false
	}

	switch path {
	case "id":
		if u.ID == "" {
			return "", false
		}
		return u.ID, true
	case "account":
		if u.Account == "" {
			return "", false
		}
		return u.Account, true
	default:
		// Check for attr.<key>
		if strings.HasPrefix(path, "attr.") {
			key := strings.TrimPrefix(path, "attr.")
			if u.Attributes == nil {
				return "", false
			}
			value, ok := u.Attributes[key]
			if !ok || value == "" {
				return "", false
			}
			return value, true
		}
		return "", false
	}
}

// RoleContext contains role attributes needed for policy interpolation.
// Implements Context interface for role.* variables.
type RoleContext struct {
	Name    string // role name
	Account string // role account ("*" for global)
}

// HasAttribute returns true if the role attribute exists.
func (r *RoleContext) HasAttribute(path string) bool {
	_, ok := r.GetAttribute(path)
	return ok
}

// GetAttribute returns a role attribute value.
// Supported paths: "name", "account"
func (r *RoleContext) GetAttribute(path string) (string, bool) {
	if r == nil {
		return "", false
	}

	switch path {
	case "name":
		if r.Name == "" {
			return "", false
		}
		return r.Name, true
	case "account":
		if r.Account == "" {
			return "", false
		}
		return r.Account, true
	default:
		return "", false
	}
}

// InterpolationContext combines multiple contexts with prefixed namespaces.
// For example, "user.id" would look up "id" in the "user" context.
type InterpolationContext struct {
	contexts map[string]Context
}

// Add adds a context with the given prefix.
func (c *InterpolationContext) Add(prefix string, ctx Context) {
	if c.contexts == nil {
		c.contexts = make(map[string]Context)
	}
	c.contexts[prefix] = ctx
}

// HasAttribute returns true if the attribute exists in any registered context.
func (c *InterpolationContext) HasAttribute(path string) bool {
	_, ok := c.GetAttribute(path)
	return ok
}

// GetAttribute looks up a path like "user.id" by finding the "user" context
// and calling GetAttribute("id") on it.
func (c *InterpolationContext) GetAttribute(path string) (string, bool) {
	if c == nil || c.contexts == nil {
		return "", false
	}

	parts := strings.SplitN(path, ".", 2)
	if len(parts) < 2 {
		return "", false
	}

	prefix := parts[0]
	rest := parts[1]

	ctx, ok := c.contexts[prefix]
	if !ok {
		return "", false
	}

	return ctx.GetAttribute(rest)
}
