package provider

import "errors"

var (
	// ErrAccountNotFound is returned when an account cannot be found.
	ErrAccountNotFound = errors.New("account not found")

	// ErrPolicyNotFound is returned when a policy cannot be found.
	ErrPolicyNotFound = errors.New("policy not found")

	// ErrGroupNotFound is returned when a group cannot be found.
	ErrGroupNotFound = errors.New("group not found")
)
