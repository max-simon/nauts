// Package identity provides user identity types and providers for nauts.
package identity

// User represents a user identity that can be authenticated.
type User struct {
	ID         string            `json:"id,omitempty"`         // user identifier (from external)
	Account    string            `json:"account"`              // NATS account ID
	Roles      []string          `json:"roles"`                // list of role names the user belongs to
	Attributes map[string]string `json:"attributes,omitempty"` // additional user attributes
}
