package policy

import (
	"errors"
	"fmt"
	"strings"
)

// Error codes for policy errors.
const (
	ErrCodeInvalidResource     = "invalid_resource"
	ErrCodeUnknownResourceType = "unknown_resource_type"
	ErrCodeInvalidWildcard     = "invalid_wildcard"
	ErrCodeUnresolvedVariable  = "unresolved_variable"
	ErrCodeInvalidValue        = "invalid_value"
	ErrCodeUnknownAction       = "unknown_action"
)

// Sentinel errors for common failure modes.
var (
	// Resource errors
	ErrInvalidResource     = errors.New("invalid resource format")
	ErrUnknownResourceType = errors.New("unknown resource type")
	ErrInvalidWildcard     = errors.New("invalid wildcard usage")

	// Interpolation errors
	ErrUnresolvedVariable = errors.New("unresolved variable")
	ErrInvalidValue       = errors.New("invalid interpolated value")

	// Action errors
	ErrUnknownAction = errors.New("unknown action")
)

// PolicyError represents an error during policy processing.
// It provides structured error information with a code, message, and attributes.
type PolicyError struct {
	Code    string            // Error code (e.g., "invalid_resource", "unresolved_variable")
	Message string            // Human-readable error message
	Attrs   map[string]string // Additional context (resource, variable, template, etc.)
	Err     error             // Wrapped error
}

// Error implements the error interface.
func (e *PolicyError) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Code)
	sb.WriteString(": ")
	sb.WriteString(e.Message)

	if len(e.Attrs) > 0 {
		sb.WriteString(" (")
		first := true
		for k, v := range e.Attrs {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(v)
			first = false
		}
		sb.WriteString(")")
	}

	if e.Err != nil {
		sb.WriteString(": ")
		sb.WriteString(e.Err.Error())
	}

	return sb.String()
}

// Unwrap returns the wrapped error.
func (e *PolicyError) Unwrap() error {
	return e.Err
}

// NewPolicyError creates a new PolicyError.
func NewPolicyError(code, message string, attrs map[string]string, err error) *PolicyError {
	return &PolicyError{
		Code:    code,
		Message: message,
		Attrs:   attrs,
		Err:     err,
	}
}

// NewResourceError creates a PolicyError for resource parsing/validation errors.
// This is a convenience function that maintains backward compatibility.
func NewResourceError(resource, message string, err error) *PolicyError {
	code := ErrCodeInvalidResource
	if errors.Is(err, ErrUnknownResourceType) {
		code = ErrCodeUnknownResourceType
	} else if errors.Is(err, ErrInvalidWildcard) {
		code = ErrCodeInvalidWildcard
	}

	return &PolicyError{
		Code:    code,
		Message: message,
		Attrs:   map[string]string{"resource": resource},
		Err:     err,
	}
}

// NewInterpolationError creates a PolicyError for interpolation errors.
// This is a convenience function that maintains backward compatibility.
func NewInterpolationError(template, variable, value, message string, err error) *PolicyError {
	code := ErrCodeUnresolvedVariable
	if errors.Is(err, ErrInvalidValue) {
		code = ErrCodeInvalidValue
	}

	attrs := map[string]string{"template": template}
	if variable != "" {
		attrs["variable"] = variable
	}
	if value != "" {
		attrs["value"] = value
	}

	return &PolicyError{
		Code:    code,
		Message: message,
		Attrs:   attrs,
		Err:     err,
	}
}

// ValidationError represents a validation error for a policy or statement.
type ValidationError struct {
	Field   string
	Index   int
	Message string
}

func (e *ValidationError) Error() string {
	if e.Index > 0 {
		return fmt.Sprintf("%s[%d]: %s", e.Field, e.Index, e.Message)
	}
	return e.Field + ": " + e.Message
}
