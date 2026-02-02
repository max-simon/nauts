package auth

import "fmt"

// AuthError represents an error during authentication or permission compilation.
type AuthError struct {
	UserID  string
	Phase   string
	Message string
	Err     error
}

func (e *AuthError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("auth error for user %q in %s: %s: %v", e.UserID, e.Phase, e.Message, e.Err)
	}
	return fmt.Sprintf("auth error for user %q in %s: %s", e.UserID, e.Phase, e.Message)
}

func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError creates a new AuthError.
func NewAuthError(userID, phase, message string, err error) *AuthError {
	return &AuthError{
		UserID:  userID,
		Phase:   phase,
		Message: message,
		Err:     err,
	}
}
