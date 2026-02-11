// Package policy provides policy-related types and functions for nauts.
// This file contains variable interpolation functionality.
package policy

import (
	"regexp"
	"strings"
)

// variablePattern matches {{ var.path }} style placeholders.
// Allows optional whitespace around the variable name.
var variablePattern = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*\}\}`)

// validValuePattern matches valid interpolated values: alphanumeric, dash, underscore, dot.
// Dots are allowed for multi-level identifiers (e.g., "service.orders").
var validValuePattern = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]+$`)

// InterpolationResult represents the result of an interpolation.
type InterpolationResult struct {
	Value    string // The interpolated value
	Excluded bool   // True if the resource should be excluded
	Warning  string // Warning message if excluded (for logging)
}

// InterpolateWithContext processes a template string, replacing variables with
// values from the given PolicyContext.
//
// Variables are resolved by looking up the full key inside {{ ... }}.
// Example: {{ user.id }} resolves ctx.Get("user.id").
func InterpolateWithContext(template string, ctx *PolicyContext) InterpolationResult {
	if ctx == nil {
		return InterpolationResult{Excluded: true, Warning: "nil context"}
	}

	// Find all variables in the template
	matches := variablePattern.FindAllStringSubmatchIndex(template, -1)
	if len(matches) == 0 {
		// No variables to interpolate
		return InterpolationResult{Value: template}
	}

	var result strings.Builder
	lastEnd := 0

	for _, match := range matches {
		// match[0]:match[1] is the full {{ var }} match
		// match[2]:match[3] is the variable name inside
		fullStart, fullEnd := match[0], match[1]
		varStart, varEnd := match[2], match[3]
		variable := template[varStart:varEnd]

		// Add text before this variable
		result.WriteString(template[lastEnd:fullStart])

		value, ok := ctx.Get(variable)
		if !ok {
			return InterpolationResult{
				Excluded: true,
				Warning:  "unresolved variable: " + variable,
			}
		}

		// Sanitize the resolved value
		if err := sanitizeValue(value); err != nil {
			return InterpolationResult{
				Excluded: true,
				Warning:  "invalid value for " + variable + ": " + value,
			}
		}

		result.WriteString(value)
		lastEnd = fullEnd
	}

	// Add remaining text after last variable
	result.WriteString(template[lastEnd:])

	return InterpolationResult{Value: result.String()}
}

// ContainsVariables returns true if the string contains template variables.
func ContainsVariables(s string) bool {
	return variablePattern.MatchString(s)
}

// sanitizeValue validates an interpolated value.
// Returns an error if the value is invalid.
//
// Validation rules:
// - Empty strings are rejected
// - The `*` wildcard character is not allowed
// - The `>` wildcard character is not allowed
// - Values must match [a-zA-Z0-9_\-\.]+
func sanitizeValue(value string) error {
	if value == "" {
		return ErrInvalidValue
	}

	// Reject wildcard characters
	for _, c := range value {
		if c == '*' || c == '>' {
			return ErrInvalidValue
		}
	}

	// Validate allowed characters
	if !validValuePattern.MatchString(value) {
		return ErrInvalidValue
	}

	return nil
}
