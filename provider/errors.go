package provider

import "errors"

var (
	// ErrAccountNotFound is returned when an account cannot be found.
	ErrAccountNotFound = errors.New("account not found")

	// ErrOperatorNotFound is returned when an operator cannot be found.
	ErrOperatorNotFound = errors.New("operator not found")

	// ErrSigningKeyNotFound is returned when a signing key cannot be found.
	ErrSigningKeyNotFound = errors.New("signing key not found")

	// ErrPolicyNotFound is returned when a policy cannot be found.
	ErrPolicyNotFound = errors.New("policy not found")

	// ErrGroupNotFound is returned when a group cannot be found.
	ErrGroupNotFound = errors.New("group not found")
)
