package callout

import "fmt"

// CalloutError represents an error during the authentication callout flow.
type CalloutError struct {
	Phase   string // "authenticate" or "authorize"
	Message string
	Err     error
}

func (e *CalloutError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("callout error in %s: %s: %v", e.Phase, e.Message, e.Err)
	}
	return fmt.Sprintf("callout error in %s: %s", e.Phase, e.Message)
}

func (e *CalloutError) Unwrap() error {
	return e.Err
}

// NewCalloutError creates a new CalloutError.
func NewCalloutError(phase, message string, err error) *CalloutError {
	return &CalloutError{
		Phase:   phase,
		Message: message,
		Err:     err,
	}
}
