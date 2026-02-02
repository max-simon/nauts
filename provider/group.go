package provider

// Group represents a collection of policies assigned to users.
type Group struct {
	ID       string   `json:"id"`       // unique identifier
	Name     string   `json:"name"`     // human-readable name
	Policies []string `json:"policies"` // list of policy IDs
}

// DefaultGroupID is the ID of the default group that all users belong to.
const DefaultGroupID = "default"

// GroupValidationError represents a validation error for a group field.
type GroupValidationError struct {
	Field   string
	Message string
}

func (e *GroupValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Validate validates a group for correctness.
func (g *Group) Validate() error {
	if g.ID == "" {
		return &GroupValidationError{Field: "id", Message: "group ID is required"}
	}
	return nil
}
