package store

import "errors"

// Sentinel errors for store operations.
var (
	ErrPolicyNotFound = errors.New("policy not found")
	ErrGroupNotFound  = errors.New("group not found")
	ErrUserNotFound   = errors.New("user not found")
)
