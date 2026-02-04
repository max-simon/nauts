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

// JwtUserIdentityProvider errors.
var (
	// ErrIssuerNotConfigured is returned when the JWT issuer is not in the configuration.
	ErrIssuerNotConfigured = errors.New("issuer not configured")

	// ErrIssuerNotAllowed is returned when the issuer is not allowed to manage the target account.
	ErrIssuerNotAllowed = errors.New("issuer not allowed to manage account")

	// ErrNoRolesFound is returned when no valid roles are found in the JWT.
	ErrNoRolesFound = errors.New("no valid roles found in token")

	// ErrInvalidRoleFormat is returned when a role doesn't match the expected format.
	ErrInvalidRoleFormat = errors.New("invalid role format")

	// ErrWildcardInAccount is returned when the target account contains a wildcard.
	ErrWildcardInAccount = errors.New("wildcard not allowed in target account")

	// ErrAmbiguousAccount is returned when roles span multiple accounts and no account is specified.
	ErrAmbiguousAccount = errors.New("ambiguous account: roles span multiple accounts")
)

// IssuerConfig holds configuration for a single JWT issuer.
type IssuerConfig struct {
	// PublicKey is the PEM-encoded public key for JWT signature verification.
	PublicKey string `json:"publicKey"`
	// Accounts is the list of NATS accounts this issuer can manage.
	// Supports wildcards: "*" matches any account, "tenant-a-*" matches accounts starting with "tenant-a-".
	Accounts []string `json:"accounts"`
	// RolesClaimPath is the path to roles in JWT claims (dot-separated).
	// Default: "resource_access.nauts.roles"
	RolesClaimPath string `json:"rolesClaimPath,omitempty"`
}

// JwtUserIdentityProviderConfig holds configuration for JwtUserIdentityProvider.
type JwtUserIdentityProviderConfig struct {
	// Issuers maps JWT issuer (iss claim) to their configuration.
	Issuers map[string]IssuerConfig `json:"issuers"`
}

// JwtUserIdentityProvider implements UserIdentityProvider using external JWTs.
// It verifies JWTs from configured issuers and extracts user information from claims.
//
// Roles in the JWT must follow the format "<account>.<role>" (e.g., "tenant-a.admin").
// The provider validates that the issuer is allowed to manage the target account
// and filters roles to only include those for the target account.
type JwtUserIdentityProvider struct {
	issuers map[string]*issuerEntry
}

// issuerEntry holds parsed issuer configuration.
type issuerEntry struct {
	publicKey      any      // *rsa.PublicKey, *ecdsa.PublicKey, or ed25519.PublicKey
	accounts       []string // allowed accounts (with potential wildcards)
	rolesClaimPath []string // path to roles in JWT claims (split by ".")
}

// NewJwtUserIdentityProvider creates a new JwtUserIdentityProvider from the given configuration.
func NewJwtUserIdentityProvider(cfg JwtUserIdentityProviderConfig) (*JwtUserIdentityProvider, error) {
	provider := &JwtUserIdentityProvider{
		issuers: make(map[string]*issuerEntry),
	}

	// Parse issuer configurations
	for issuer, issuerCfg := range cfg.Issuers {
		pubKey, err := parsePublicKey(issuerCfg.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parsing public key for issuer %q: %w", issuer, err)
		}

		// Parse roles claim path for this issuer
		rolesPath := issuerCfg.RolesClaimPath
		if rolesPath == "" {
			rolesPath = "resource_access.nauts.roles"
		}

		provider.issuers[issuer] = &issuerEntry{
			publicKey:      pubKey,
			accounts:       issuerCfg.Accounts,
			rolesClaimPath: strings.Split(rolesPath, "."),
		}
	}

	return provider, nil
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
//  1. Decode JWT header to extract the issuer (iss claim)
//  2. Look up issuer in configuration and get the public key
//  3. Verify JWT signature using the issuer's public key
//  4. Extract roles from claims at the configured path (default: resource_access.nauts.roles)
//  5. Parse roles: each role must be in format "<account>.<role>", invalid formats are skipped
//  6. Determine target account:
//     - If AuthRequest.Account is provided, use it
//     - Otherwise, derive from roles: if all roles belong to the same account, use that account
//     - If roles span multiple accounts, return ErrAmbiguousAccount
//  7. Validate issuer is allowed to manage the target account (supports wildcards in config)
//  8. Filter roles to only include those for the target account
//  9. Strip account prefix from role names (e.g., "tenant-a.admin" becomes "admin")
func (p *JwtUserIdentityProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
	// Step 1-3: Parse and verify JWT
	token, issuerEntry, err := p.parseAndVerifyJWT(req.Token)
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

	// Step 4: Extract roles from claims using issuer's configured path
	rawRoles, err := extractRoles(claims, issuerEntry.rolesClaimPath)
	if err != nil {
		return nil, err
	}

	// Step 5: Parse roles into account/role pairs
	parsedRoles := parseAccountRoles(rawRoles)
	if len(parsedRoles) == 0 {
		return nil, ErrNoRolesFound
	}

	// Step 6: Determine target account
	targetAccount, err := p.determineTargetAccount(req.Account, parsedRoles)
	if err != nil {
		return nil, err
	}

	// Step 7: Validate issuer can manage target account
	if !p.issuerCanManageAccount(issuerEntry, targetAccount) {
		return nil, ErrIssuerNotAllowed
	}

	// Step 8-9: Filter and strip roles for target account
	roles := filterAndStripRoles(parsedRoles, targetAccount)
	if len(roles) == 0 {
		return nil, ErrNoRolesFound
	}

	// Extract additional attributes from standard claims
	attributes := extractAttributes(claims)

	return &User{
		ID:         userID,
		Account:    targetAccount,
		Roles:      roles,
		Attributes: attributes,
	}, nil
}

// parseAndVerifyJWT parses the JWT, looks up the issuer, and verifies the signature.
func (p *JwtUserIdentityProvider) parseAndVerifyJWT(tokenString string) (*jwt.Token, *issuerEntry, error) {
	// First, parse without verification to get the issuer
	unverified, _, err := jwt.NewParser().ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	// Get issuer from claims
	claims, ok := unverified.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil, ErrInvalidTokenType
	}

	issuer, _ := claims["iss"].(string)
	if issuer == "" {
		return nil, nil, fmt.Errorf("%w: missing issuer claim", ErrInvalidCredentials)
	}

	// Look up issuer configuration
	entry, ok := p.issuers[issuer]
	if !ok {
		return nil, nil, ErrIssuerNotConfigured
	}

	// Now parse with verification
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		// Validate signing method matches key type
		switch entry.publicKey.(type) {
		case *rsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		case *ecdsa.PublicKey:
			if _, ok := t.Method.(*jwt.SigningMethodECDSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
		}
		return entry.publicKey, nil
	})

	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	if !token.Valid {
		return nil, nil, ErrInvalidCredentials
	}

	return token, entry, nil
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

// accountRole represents a parsed role with account and role name.
type accountRole struct {
	account string
	role    string
}

// parseAccountRoles parses roles in format "<account>.<role>".
// Invalid formats are skipped.
func parseAccountRoles(roles []string) []accountRole {
	var parsed []accountRole
	for _, r := range roles {
		parts := strings.SplitN(r, ".", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			// Invalid format, skip
			continue
		}
		parsed = append(parsed, accountRole{
			account: parts[0],
			role:    parts[1],
		})
	}
	return parsed
}

// determineTargetAccount determines the target account from the request or roles.
func (p *JwtUserIdentityProvider) determineTargetAccount(requestedAccount string, roles []accountRole) (string, error) {
	// If account is explicitly specified, use it
	if requestedAccount != "" {
		// Validate no wildcards
		if strings.Contains(requestedAccount, "*") {
			return "", ErrWildcardInAccount
		}
		return requestedAccount, nil
	}

	// Derive from roles
	accounts := make(map[string]bool)
	for _, r := range roles {
		accounts[r.account] = true
	}

	if len(accounts) == 0 {
		return "", ErrNoRolesFound
	}

	if len(accounts) == 1 {
		// All roles belong to the same account
		for acc := range accounts {
			// Validate no wildcards
			if strings.Contains(acc, "*") {
				return "", ErrWildcardInAccount
			}
			return acc, nil
		}
	}

	// Multiple accounts - ambiguous
	return "", ErrAmbiguousAccount
}

// issuerCanManageAccount checks if the issuer is allowed to manage the target account.
func (p *JwtUserIdentityProvider) issuerCanManageAccount(entry *issuerEntry, targetAccount string) bool {
	for _, pattern := range entry.accounts {
		if matchAccountPattern(pattern, targetAccount) {
			return true
		}
	}
	return false
}

// matchAccountPattern checks if an account matches a pattern.
// Patterns can be:
//   - "*" matches any account
//   - "prefix-*" matches accounts starting with "prefix-"
//   - "exact" matches only "exact"
func matchAccountPattern(pattern, account string) bool {
	if pattern == "*" {
		return true
	}

	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(account, prefix)
	}

	return pattern == account
}

// filterAndStripRoles filters roles for the target account and strips the account prefix.
func filterAndStripRoles(roles []accountRole, targetAccount string) []string {
	var result []string
	seen := make(map[string]bool)

	for _, r := range roles {
		if r.account == targetAccount {
			if !seen[r.role] {
				seen[r.role] = true
				result = append(result, r.role)
			}
		}
	}

	return result
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
