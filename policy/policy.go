package policy

// Effect represents the effect of a policy statement.
type Effect string

const (
	// EffectAllow grants the specified permissions.
	EffectAllow Effect = "allow"
	// EffectDeny explicitly denies the specified permissions (not implemented).
	// EffectDeny Effect = "deny"
)

// Statement represents a permission statement within a policy.
type Statement struct {
	Effect    Effect   `json:"effect"`    // allow or deny
	Actions   []Action `json:"actions"`   // list of actions to allow/deny
	Resources []string `json:"resources"` // list of NRN patterns
}

// Policy represents a collection of permission statements.
type Policy struct {
	ID         string      `json:"id"`         // unique identifier
	Name       string      `json:"name"`       // human-readable name
	Statements []Statement `json:"statements"` // list of permission statements
}

// IsValid checks if the effect is a valid effect type.
func (e Effect) IsValid() bool {
	return e == EffectAllow
}

// Validate validates a policy for correctness.
func (p *Policy) Validate() error {
	if p.ID == "" {
		return &ValidationError{Field: "id", Message: "policy ID is required"}
	}
	if len(p.Statements) == 0 {
		return &ValidationError{Field: "statements", Message: "policy must have at least one statement"}
	}
	for i, stmt := range p.Statements {
		if err := stmt.Validate(); err != nil {
			return &ValidationError{Field: "statements", Index: i, Message: err.Error()}
		}
	}
	return nil
}

// Validate validates a statement for correctness.
func (s *Statement) Validate() error {
	if !s.Effect.IsValid() {
		return &ValidationError{Field: "effect", Message: "invalid effect: " + string(s.Effect)}
	}
	if len(s.Actions) == 0 {
		return &ValidationError{Field: "actions", Message: "statement must have at least one action"}
	}
	for _, action := range s.Actions {
		if !action.IsValid() {
			return &ValidationError{Field: "actions", Message: "invalid action: " + string(action)}
		}
	}
	if len(s.Resources) == 0 {
		return &ValidationError{Field: "resources", Message: "statement must have at least one resource"}
	}
	return nil
}
