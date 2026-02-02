package provider

import "errors"

// Sentinel errors for provider operations.
var (
	ErrPolicyNotFound = errors.New("policy not found")
	ErrGroupNotFound  = errors.New("group not found")
)
