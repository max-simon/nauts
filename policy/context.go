// Package policy provides policy-related types and functions for nauts.
// This file contains context types for variable interpolation.
package policy

// PolicyContext holds interpolation variables for policy compilation.
//
// Variables are stored in a flat key/value map. Template variables reference
// keys directly, for example:
//
//	ctx.Set("user.id", "myuserid")
//	ctx.Set("account.id", "APP")
//	template: "nats:user.{{ user.id }}.>" → "nats:user.myuserid.>"
//
// Empty values behave like an unset key (the variable will be unresolved).
type PolicyContext struct {
	claims map[string]string
}

// Set sets a context key to a value.
//
// If value is empty, the key is removed so that interpolation treats it as
// unresolved (resource excluded).
func (c *PolicyContext) Set(key, value string) {
	if c == nil {
		return
	}
	if value == "" {
		if c.claims != nil {
			delete(c.claims, key)
		}
		return
	}
	if c.claims == nil {
		c.claims = make(map[string]string)
	}
	c.claims[key] = value
}

// Get returns the value for a context key.
//
// Empty values are treated as missing.
func (c *PolicyContext) Get(key string) (string, bool) {
	if c == nil || c.claims == nil {
		return "", false
	}
	value, ok := c.claims[key]
	if !ok || value == "" {
		return "", false
	}
	return value, true
}

// Clone returns a shallow copy of the context.
func (c *PolicyContext) Clone() *PolicyContext {
	if c == nil {
		return nil
	}
	out := &PolicyContext{}
	if len(c.claims) == 0 {
		return out
	}
	out.claims = make(map[string]string, len(c.claims))
	for k, v := range c.claims {
		out.claims[k] = v
	}
	return out
}
