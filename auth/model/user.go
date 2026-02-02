// Package model provides core domain types for nauts.
package model

// User represents a user identity that can be authenticated.
type User struct {
	ID         string            `json:"id,omitempty"`         // user identifier (from external)
	Account    string            `json:"account"`              // NATS account ID
	Groups     []string          `json:"groups"`               // list of group IDs the user belongs to
	Attributes map[string]string `json:"attributes,omitempty"` // additional user attributes
}

// GetAttribute returns an attribute value, or empty string if not found.
func (u *User) GetAttribute(key string) string {
	if u.Attributes == nil {
		return ""
	}
	return u.Attributes[key]
}
