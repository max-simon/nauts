// Package policy provides policy-related types and functions for nauts.
// This file contains context types for variable interpolation.
package policy

import "strings"

// PolicyContext holds interpolation variables for policy compilation.
//
// Variables are stored in a flat key/value map. Template variables reference
// keys directly, for example:
//
//	ctx.User = "myuserid"
//	ctx.Account = "APP"
//	template: "nats:user.{{ user.id }}.>" → "nats:user.myuserid.>"
//
// Empty values behave like an unset key (the variable will be unresolved).
type PolicyContext struct {
	// User is exposed to interpolation as `user.id`.
	User string
	// Account is exposed to interpolation as `account.id`.
	Account string
	// Role is exposed to interpolation as `role.id`.
	Role string
	// UserClaims provides additional user claims exposed as `user.attr.<key>`.
	UserClaims map[string]string
}

// Get returns the value for a context key.
//
// Empty values are treated as missing.
func (c *PolicyContext) Get(key string) (string, bool) {
	if c == nil {
		return "", false
	}

	switch key {
	case "user.id":
		if c.User == "" {
			return "", false
		}
		return c.User, true
	case "account.id":
		if c.Account == "" {
			return "", false
		}
		return c.Account, true
	case "role.id":
		if c.Role == "" {
			return "", false
		}
		return c.Role, true
	}

	const userAttrPrefix = "user.attr."
	if strings.HasPrefix(key, userAttrPrefix) {
		attrKey := strings.TrimPrefix(key, userAttrPrefix)
		if attrKey == "" || c.UserClaims == nil {
			return "", false
		}
		value := c.UserClaims[attrKey]
		if value == "" {
			return "", false
		}
		return value, true
	}

	return "", false
}

// Clone returns a copy of the context.
func (c *PolicyContext) Clone() *PolicyContext {
	if c == nil {
		return nil
	}
	out := &PolicyContext{
		User:    c.User,
		Account: c.Account,
		Role:    c.Role,
	}
	if len(c.UserClaims) == 0 {
		return out
	}
	out.UserClaims = make(map[string]string, len(c.UserClaims))
	for k, v := range c.UserClaims {
		out.UserClaims[k] = v
	}
	return out
}
