package identity

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// JwtAuthenticationProvider errors.
var (
	// ErrIssuerNotConfigured is returned when the JWT issuer is not in the configuration.
	ErrIssuerNotConfigured = errors.New("issuer not configured")

	// ErrNoRolesFound is returned when no valid roles are found in the JWT.
	ErrNoRolesFound = errors.New("no valid roles found in token")
)

// JwtAuthenticationProviderConfig holds configuration for JwtAuthenticationProvider.
type JwtAuthenticationProviderConfig struct {
	// ID is the provider identifier.
	ID string
	// Accounts is the list of accounts this provider manages (supports wildcards).
	Accounts []string
	// Issuer is the expected JWT issuer (iss claim).
	Issuer string
	// PublicKey is the PEM-encoded public key for JWT signature verification.
	PublicKey string
	// RolesClaimPath is the path to roles in JWT claims (dot-separated).
	// Default: "resource_access.nauts.roles"
	RolesClaimPath string
}

// JwtAuthenticationProvider implements AuthenticationProvider using external JWTs.
// It verifies JWTs from a configured issuer and extracts user information from claims.
//
// Roles in the JWT must follow the format "<account>.<role>" (e.g., "APP.admin").
type JwtAuthenticationProvider struct {
	id             string
	accounts       []string
	issuer         string
	publicKey      any      // *rsa.PublicKey, *ecdsa.PublicKey, or ed25519.PublicKey
	rolesClaimPath []string // path to roles in JWT claims (split by ".")
}

// NewJwtAuthenticationProvider creates a new JwtAuthenticationProvider from the given configuration.
func NewJwtAuthenticationProvider(cfg JwtAuthenticationProviderConfig) (*JwtAuthenticationProvider, error) {
	// Validate required fields
	if cfg.ID == "" {
		return nil, errors.New("ID is required")
	}
	if len(cfg.Accounts) == 0 {
		return nil, errors.New("at least one account is required")
	}
	if cfg.Issuer == "" {
		return nil, errors.New("issuer is required")
	}
	if cfg.PublicKey == "" {
		return nil, errors.New("public key is required")
	}

	pubKey, err := parsePublicKey(cfg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	rolesPath := cfg.RolesClaimPath
	if rolesPath == "" {
		rolesPath = "resource_access.nauts.roles"
	}

	return &JwtAuthenticationProvider{
		id:             cfg.ID,
		accounts:       cfg.Accounts,
		issuer:         cfg.Issuer,
		publicKey:      pubKey,
		rolesClaimPath: strings.Split(rolesPath, "."),
	}, nil
}

// ID returns the unique identifier for this provider.
func (p *JwtAuthenticationProvider) ID() string {
	return p.id
}

// CanManageAccount returns true if this provider is authorized to authenticate
// users for the given account.
func (p *JwtAuthenticationProvider) CanManageAccount(account string) bool {
	for _, pattern := range p.accounts {
		if MatchAccountPattern(pattern, account) {
			return true
		}
	}
	return false
}

// parsePublicKey parses a PEM-encoded public key.
// pemDataB64 is base64 encoded
func parsePublicKey(pemDataB64 string) (any, error) {
	pemData, err := base64.StdEncoding.DecodeString(pemDataB64)
	if err != nil {
		return nil, errors.New("failed to decode base64 PEM block")
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Try parsing as PKIX public key (most common format)
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		return pub, nil
	}

	// Try parsing as PKCS1 RSA public key
	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err == nil {
		return rsaPub, nil
	}

	return nil, errors.New("unsupported public key format")
}

// Verify validates the JWT and returns the user.
//
// The verification process:
//  1. Parse and verify JWT signature using the provider's public key
//  2. Validate issuer matches configuration
//  3. Extract roles from claims at the configured path (default: resource_access.nauts.roles)
//  4. Parse roles: each role must be in format "<account>.<role>", invalid formats are skipped
//  5. Return user with all roles (no filtering)
func (p *JwtAuthenticationProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
	// Parse and verify JWT
	token, err := p.parseAndVerifyJWT(req.Token)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenType
	}

	// Extract user ID from subject claim
	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID = "unknown"
	}

	// Extract roles from claims using configured path
	rawRoles, err := extractRoles(claims, p.rolesClaimPath)
	if err != nil {
		return nil, err
	}

	// Parse roles into AccountRole objects
	roles := parseAccountRoles(rawRoles)
	if len(roles) == 0 {
		return nil, ErrNoRolesFound
	}

	// Extract additional attributes from standard claims
	attributes := extractAttributes(claims)

	return &User{
		ID:         userID,
		Roles:      roles,
		Attributes: attributes,
	}, nil
}

// parseAndVerifyJWT parses the JWT and verifies the signature.
func (p *JwtAuthenticationProvider) parseAndVerifyJWT(tokenString string) (*jwt.Token, error) {
	// Parse with verification
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		// Validate signing method matches key type
		switch p.publicKey.(type) {
		case *rsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		case *ecdsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		}
		return p.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	if !token.Valid {
		return nil, ErrInvalidCredentials
	}

	// Verify issuer matches configuration
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenType
	}

	issuer, _ := claims["iss"].(string)
	if issuer != p.issuer {
		return nil, fmt.Errorf("%w: expected %q, got %q", ErrIssuerNotConfigured, p.issuer, issuer)
	}

	return token, nil
}

// extractRoles extracts roles from JWT claims at the given path.
func extractRoles(claims jwt.MapClaims, rolesClaimPath []string) ([]string, error) {
	// Navigate to the roles claim using the configured path
	var current any = map[string]any(claims)

	for i, key := range rolesClaimPath {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid claim path at %q", strings.Join(rolesClaimPath[:i], "."))
		}
		current, ok = m[key]
		if !ok {
			// Roles claim not present - return empty
			return nil, nil
		}
	}

	// Convert to string slice
	rolesSlice, ok := current.([]any)
	if !ok {
		return nil, fmt.Errorf("roles claim is not an array")
	}

	var roles []string
	for _, r := range rolesSlice {
		if s, ok := r.(string); ok {
			roles = append(roles, s)
		}
	}

	return roles, nil
}

// parseAccountRoles parses roles in format "<account>.<role>".
// Invalid formats are skipped.
func parseAccountRoles(roles []string) []AccountRole {
	var parsed []AccountRole
	for _, r := range roles {
		parts := strings.SplitN(r, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			// Invalid format, skip
			continue
		}
		parsed = append(parsed, AccountRole{
			Account: parts[0],
			Role:    parts[1],
		})
	}
	return parsed
}

// extractAttributes extracts user attributes from standard JWT claims.
func extractAttributes(claims jwt.MapClaims) map[string]string {
	attrs := make(map[string]string)

	// Extract common claims as attributes
	if email, ok := claims["email"].(string); ok && email != "" {
		attrs["email"] = email
	}
	if name, ok := claims["name"].(string); ok && name != "" {
		attrs["name"] = name
	}
	if preferredUsername, ok := claims["preferred_username"].(string); ok && preferredUsername != "" {
		attrs["preferred_username"] = preferredUsername
	}

	return attrs
}
