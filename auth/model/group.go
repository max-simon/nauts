// Package model provides core domain types for nauts.
package model

// Group represents a collection of policies assigned to users.
type Group struct {
	ID       string   `json:"id"`       // unique identifier
	Name     string   `json:"name"`     // human-readable name
	Policies []string `json:"policies"` // list of policy IDs
}

// DefaultGroupID is the ID of the default group that all users belong to.
const DefaultGroupID = "default"

// Validate validates a group for correctness.
func (g *Group) Validate() error {
	if g.ID == "" {
		return &ValidationError{Field: "id", Message: "group ID is required"}
	}
	return nil
}

// ValidationError represents a validation error for a model field.
type ValidationError struct {
	Field   string
	Index   int
	Message string
}

func (e *ValidationError) Error() string {
	if e.Index > 0 {
		return e.Field + "[" + string(rune('0'+e.Index)) + "]: " + e.Message
	}
	return e.Field + ": " + e.Message
}
