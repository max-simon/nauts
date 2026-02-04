package provider

// Role represents a collection of policies that can be assigned to users.
// Roles are uniquely identified by the combination of Name and Account.
// Global roles have Account set to GlobalAccountID ("*") and apply to all accounts.
// Local roles have a specific account value and only apply to that account.
type Role struct {
	Name     string   `json:"name"`     // part of unique key
	Account  string   `json:"account"`  // "*" for global, specific account for local
	Policies []string `json:"policies"` // list of policy IDs
}

// DefaultRoleName is the name of the default role that all users belong to.
const DefaultRoleName = "default"

// GlobalAccountID is the account value for global roles.
const GlobalAccountID = "*"

// IsGlobal returns true if this is a global role (applies to all accounts).
func (r *Role) IsGlobal() bool {
	return r.Account == GlobalAccountID
}

// Key returns the unique key for this role (name:account).
func (r *Role) Key() string {
	return r.Name + ":" + r.Account
}

// RoleValidationError represents a validation error for a role field.
type RoleValidationError struct {
	Field   string
	Message string
}

func (e *RoleValidationError) Error() string {
	return e.Field + ": " + e.Message
}

// Validate validates a role for correctness.
func (r *Role) Validate() error {
	if r.Name == "" {
		return &RoleValidationError{Field: "name", Message: "role name is required"}
	}
	if r.Account == "" {
		return &RoleValidationError{Field: "account", Message: "role account is required"}
	}
	return nil
}
