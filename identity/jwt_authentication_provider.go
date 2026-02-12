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
	// ErrNoRolesFound is returned when no valid roles are found in the JWT.
	ErrNoRolesFound = errors.New("no valid roles found in token")
)

// JwtAuthenticationProviderConfig holds configuration for JwtAuthenticationProvider.
type JwtAuthenticationProviderConfig struct {
	// Accounts is the list of NATS accounts this provider can manage.
	// Patterns support wildcards in the form of "*" (all) or "prefix*".
	Accounts []string `json:"accounts"`
	// Issuer is the expected JWT issuer (iss claim).
	Issuer string `json:"issuer"`
	// PublicKey is the PEM-encoded public key for JWT signature verification (base64-encoded PEM block).
	PublicKey string `json:"publicKey"`
	// RolesClaimPath is the path to roles in JWT claims (dot-separated).
	// Default: "resource_access.nauts.roles"
	RolesClaimPath string `json:"rolesClaimPath,omitempty"`
}

// JwtAuthenticationProvider implements AuthenticationProvider using external JWTs.
// It verifies JWTs from configured issuers and extracts user information from claims.
//
// Roles in the JWT must follow the format "<account>.<role>" (e.g., "tenant-a.admin").
// Account manageability validation and role filtering are performed by AuthController.
type JwtAuthenticationProvider struct {
	issuer             string
	publicKey          any
	rolesClaimPath     []string
	manageableAccounts []string
}

// NewJwtAuthenticationProvider creates a new JwtAuthenticationProvider from the given configuration.
func NewJwtAuthenticationProvider(cfg JwtAuthenticationProviderConfig) (*JwtAuthenticationProvider, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	pubKey, err := parsePublicKey(cfg.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("parsing public key: %w", err)
	}

	rolesPath := cfg.RolesClaimPath
	if rolesPath == "" {
		rolesPath = "resource_access.nauts.roles"
	}

	provider := &JwtAuthenticationProvider{
		issuer:             cfg.Issuer,
		publicKey:          pubKey,
		rolesClaimPath:     strings.Split(rolesPath, "."),
		manageableAccounts: append([]string(nil), cfg.Accounts...),
	}
	return provider, nil
}

func (p *JwtAuthenticationProvider) ManageableAccounts() []string {
	return append([]string(nil), p.manageableAccounts...)
}

// GetConfig returns a JSON-serializable configuration map for debug output.
func (p *JwtAuthenticationProvider) GetConfig() map[string]any {
	return map[string]any{
		"type":                "jwt",
		"manageable_accounts": append([]string(nil), p.manageableAccounts...),
	}
}

// parsePublicKey parses a PEM-encoded public key.
// pemDataB64 is base64 encoded.
func parsePublicKey(pemDataB64 string) (any, error) {
	pemData, err := base64.StdEncoding.DecodeString(pemDataB64)
	if err != nil {
		return nil, errors.New("failed to decode base64 PEM block")
	}
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err == nil {
		return pub, nil
	}

	rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err == nil {
		return rsaPub, nil
	}

	return nil, errors.New("unsupported public key format")
}

// Verify validates the JWT and returns the user.
//
// Role filtering and account manageability validation are performed by AuthController.
func (p *JwtAuthenticationProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
	token, err := p.parseAndVerifyJWT(req.Token)
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidTokenType
	}

	userID, _ := claims["sub"].(string)
	if userID == "" {
		userID = "unknown"
	}

	issuer, _ := claims["iss"].(string)
	if issuer != p.issuer {
		return nil, ErrInvalidCredentials
	}

	rawRoles, err := extractRoles(claims, p.rolesClaimPath)
	if err != nil {
		return nil, err
	}

	parsedRoles := parseJWTAccountRoles(rawRoles)
	if len(parsedRoles) == 0 {
		return nil, ErrNoRolesFound
	}

	attributes := extractAttributes(claims)

	return &User{
		ID:         userID,
		Roles:      parsedRoles,
		Attributes: attributes,
	}, nil
}

// parseAndVerifyJWT parses the JWT and verifies the signature.
func (p *JwtAuthenticationProvider) parseAndVerifyJWT(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
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
	return token, nil
}

// extractRoles extracts roles from JWT claims at the given path.
func extractRoles(claims jwt.MapClaims, rolesClaimPath []string) ([]string, error) {
	var current any = map[string]any(claims)
	for i, key := range rolesClaimPath {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid claim path at %q", strings.Join(rolesClaimPath[:i], "."))
		}
		current, ok = m[key]
		if !ok {
			return nil, nil
		}
	}

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

// parseJWTAccountRoles parses roles in format "<account>.<role>" to Role objects.
// Invalid formats are skipped.
func parseJWTAccountRoles(roles []string) []Role {
	var parsed []Role
	for _, roleID := range roles {
		role, err := ParseRoleID(roleID)
		if err != nil {
			continue
		}
		parsed = append(parsed, role)
	}
	return parsed
}

// extractAttributes extracts user attributes from JWT claims.
func extractAttributes(claims jwt.MapClaims) map[string]string {
	attrs := make(map[string]string)
	// For now, only extract the subject claim.
	if sub, ok := claims["sub"].(string); ok && sub != "" {
		attrs["sub"] = sub
	}
	return attrs
}
