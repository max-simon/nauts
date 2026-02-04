package provider

import "errors"

var (
	// ErrAccountNotFound is returned when an account cannot be found.
	ErrAccountNotFound = errors.New("account not found")

	// ErrPolicyNotFound is returned when a policy cannot be found.
	ErrPolicyNotFound = errors.New("policy not found")

	// ErrRoleNotFound is returned when a role cannot be found.
	ErrRoleNotFound = errors.New("role not found")
)
