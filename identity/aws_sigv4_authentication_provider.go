package identity

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// AwsSigV4AuthenticationProviderConfig holds configuration for AwsSigV4AuthenticationProvider.
type AwsSigV4AuthenticationProviderConfig struct {
	// Accounts is the list of NATS account patterns this provider manages.
	// Patterns support wildcards in the form of "*" (all) or "prefix*".
	Accounts []string `json:"accounts"`

	// Region is the AWS region for STS endpoint (e.g., "us-east-1").
	// OPTIONAL: If not specified, the region is extracted from the Authorization
	// header's credential scope. If specified, validates that the signature's
	// region matches this value (security check).
	Region string `json:"region,omitempty"`

	// MaxClockSkew is the maximum allowed difference between request timestamp
	// and current time (default: 5 minutes).
	MaxClockSkew time.Duration `json:"maxClockSkew,omitempty"`

	// AWSAccount is the AWS account ID that is allowed to authenticate.
	// REQUIRED: Must be a 12-digit AWS account ID.
	// Wildcards are NOT allowed.
	AWSAccount string `json:"awsAccount"`
}

// AwsSigV4AuthenticationProvider implements AuthenticationProvider using AWS SigV4.
// It verifies AWS IAM role identities by calling AWS STS GetCallerIdentity.
//
// AWS IAM role names must follow the convention: nauts.<account>.<role>
// The provider extracts the NATS account and role from the AWS role name.
type AwsSigV4AuthenticationProvider struct {
	region             string
	maxClockSkew       time.Duration
	awsAccountID       string
	manageableAccounts []string
}

// sigV4Token represents the parsed AWS SigV4 authentication token.
type sigV4Token struct {
	Authorization string `json:"authorization"`
	Date          string `json:"date"`
	SecurityToken string `json:"securityToken,omitempty"`
}

// parsedARN represents a parsed AWS ARN.
type parsedARN struct {
	AwsAccountID string
	RoleName     string
	SessionName  string // For assumed roles
	FullARN      string
}

var (
	// ErrAWSAccountNotAllowed is returned when the AWS account is not in allowedAwsAccounts.
	ErrAWSAccountNotAllowed = errors.New("aws account not allowed")

	// ErrInvalidRoleFormat is returned when the AWS role name doesn't follow nauts.<account>.<role> pattern.
	ErrInvalidRoleFormat = errors.New("invalid aws role name format: expected nauts.<account>.<role>")

	// awsAccountIDRegex validates 12-digit AWS account IDs.
	awsAccountIDRegex = regexp.MustCompile(`^\d{12}$`)

	// natsIdentifierRegex validates NATS account and role names (alphanumeric, hyphen, underscore).
	natsIdentifierRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]+$`)
)

// NewAwsSigV4AuthenticationProvider creates a new AwsSigV4AuthenticationProvider.
func NewAwsSigV4AuthenticationProvider(cfg AwsSigV4AuthenticationProviderConfig) (*AwsSigV4AuthenticationProvider, error) {
	// Validate awsAccount is provided (REQUIRED)
	if cfg.AWSAccount == "" {
		return nil, errors.New("awsAccount is required")
	}

	// Validate no wildcards in awsAccount
	if cfg.AWSAccount == "*" || strings.Contains(cfg.AWSAccount, "*") {
		return nil, fmt.Errorf("awsAccount must not contain wildcards: %s", cfg.AWSAccount)
	}

	// Validate awsAccount format (12 digits)
	if !awsAccountIDRegex.MatchString(cfg.AWSAccount) {
		return nil, fmt.Errorf("invalid aws account ID format (expected 12 digits): %s", cfg.AWSAccount)
	}

	maxClockSkew := cfg.MaxClockSkew
	if maxClockSkew == 0 {
		maxClockSkew = 5 * time.Minute
	}

	return &AwsSigV4AuthenticationProvider{
		region:             cfg.Region,
		maxClockSkew:       maxClockSkew,
		awsAccountID:       cfg.AWSAccount,
		manageableAccounts: append([]string(nil), cfg.Accounts...),
	}, nil
}

// ManageableAccounts returns the list of account patterns this provider can manage.
func (p *AwsSigV4AuthenticationProvider) ManageableAccounts() []string {
	return append([]string(nil), p.manageableAccounts...)
}

// GetConfig returns a JSON-serializable configuration map for debug output.
func (p *AwsSigV4AuthenticationProvider) GetConfig() map[string]any {
	return map[string]any{
		"type":                "aws-sigv4",
		"manageable_accounts": append([]string(nil), p.manageableAccounts...),
	}
}

// Verify validates the authentication request and returns the user.
func (p *AwsSigV4AuthenticationProvider) Verify(ctx context.Context, req AuthRequest) (*User, error) {
	// 1. Parse token
	token, err := parseAwsSigV4Token(req.Token)
	if err != nil {
		return nil, err
	}

	// 2. Validate timestamp
	if err := validateTimestamp(token.Date, p.maxClockSkew); err != nil {
		return nil, err
	}

	// 3. Extract region from Authorization header
	extractedRegion, err := extractRegionFromAuthorization(token.Authorization)
	if err != nil {
		return nil, fmt.Errorf("extracting region: %w", err)
	}

	// 4. Validate region match if configured
	if p.region != "" && extractedRegion != p.region {
		return nil, fmt.Errorf("%w: signature uses region %s but provider requires %s",
			ErrInvalidCredentials, extractedRegion, p.region)
	}

	// Use configured region if set, otherwise use extracted region
	region := p.region
	if region == "" {
		region = extractedRegion
	}

	// 5. Call AWS STS GetCallerIdentity
	arn, err := callSTSGetCallerIdentity(ctx, token, region)
	if err != nil {
		return nil, err
	}

	// 6. Parse ARN
	parsedARN, err := parseRoleARN(arn)
	if err != nil {
		return nil, err
	}

	// 7. Validate AWS account matches configured account
	if parsedARN.AwsAccountID != p.awsAccountID {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrAWSAccountNotAllowed, p.awsAccountID, parsedARN.AwsAccountID)
	}

	// 8. Validate and extract role name
	account, role, err := validateAndParseRoleName(parsedARN.RoleName)
	if err != nil {
		return nil, err
	}

	// 9. Validate AuthRequest.Account matches extracted account
	if req.Account != account {
		return nil, fmt.Errorf("%w: requested %s but role specifies %s",
			ErrInvalidAccount, req.Account, account)
	}

	// 10. Construct User
	return constructUser(parsedARN, account, role), nil
}

// parseAwsSigV4Token parses the authentication token JSON.
func parseAwsSigV4Token(tokenStr string) (*sigV4Token, error) {
	var token sigV4Token
	if err := json.Unmarshal([]byte(tokenStr), &token); err != nil {
		return nil, ErrInvalidTokenType
	}

	if token.Authorization == "" {
		return nil, fmt.Errorf("%w: missing authorization header", ErrInvalidCredentials)
	}
	if token.Date == "" {
		return nil, fmt.Errorf("%w: missing date header", ErrInvalidCredentials)
	}

	return &token, nil
}

// validateTimestamp validates the X-Amz-Date timestamp is within acceptable clock skew.
func validateTimestamp(amzDate string, maxSkew time.Duration) error {
	// Parse X-Amz-Date format: 20260208T153045Z
	requestTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return fmt.Errorf("%w: invalid date format: %v", ErrInvalidCredentials, err)
	}

	now := time.Now()
	diff := now.Sub(requestTime)
	if diff < 0 {
		diff = -diff
	}

	if diff > maxSkew {
		return fmt.Errorf("%w: request timestamp too old or too new (diff: %v, max: %v)",
			ErrInvalidCredentials, diff, maxSkew)
	}

	return nil
}

// extractRegionFromAuthorization extracts the AWS region from the Authorization header.
// Authorization format: AWS4-HMAC-SHA256 Credential=ACCESS_KEY/20260208/us-east-1/sts/aws4_request, ...
func extractRegionFromAuthorization(authHeader string) (string, error) {
	// Find the Credential= part
	credentialPrefix := "Credential="
	credentialStart := strings.Index(authHeader, credentialPrefix)
	if credentialStart == -1 {
		return "", fmt.Errorf("%w: malformed authorization header: missing Credential", ErrInvalidCredentials)
	}

	credentialStart += len(credentialPrefix)
	credentialEnd := strings.Index(authHeader[credentialStart:], ",")
	if credentialEnd == -1 {
		credentialEnd = len(authHeader) - credentialStart
	}

	credential := authHeader[credentialStart : credentialStart+credentialEnd]

	// Parse credential scope: ACCESS_KEY/DATE/REGION/SERVICE/aws4_request
	parts := strings.Split(credential, "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("%w: malformed credential scope: expected 5 parts", ErrInvalidCredentials)
	}

	region := parts[2]
	if region == "" {
		return "", fmt.Errorf("%w: empty region in credential scope", ErrInvalidCredentials)
	}

	return region, nil
}

// stsGetCallerIdentityResponse represents the AWS STS GetCallerIdentity XML response.
type stsGetCallerIdentityResponse struct {
	XMLName xml.Name `xml:"GetCallerIdentityResponse"`
	Result  struct {
		Arn     string `xml:"Arn"`
		UserId  string `xml:"UserId"`
		Account string `xml:"Account"`
	} `xml:"GetCallerIdentityResult"`
}

// stsErrorResponse represents an AWS STS error XML response.
type stsErrorResponse struct {
	XMLName xml.Name `xml:"ErrorResponse"`
	Error   struct {
		Code    string `xml:"Code"`
		Message string `xml:"Message"`
	} `xml:"Error"`
}

// callSTSGetCallerIdentity calls AWS STS GetCallerIdentity via HTTP and returns the ARN.
func callSTSGetCallerIdentity(ctx context.Context, token *sigV4Token, region string) (string, error) {
	// Build STS endpoint URL
	endpoint := fmt.Sprintf("https://sts.%s.amazonaws.com/", region)

	// Create request body (form-encoded)
	body := strings.NewReader("Action=GetCallerIdentity&Version=2011-06-15")

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, body)
	if err != nil {
		return "", fmt.Errorf("creating STS request: %w", err)
	}

	// Set headers from the pre-signed token
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	req.Header.Set("Authorization", token.Authorization)
	req.Header.Set("X-Amz-Date", token.Date)
	if token.SecurityToken != "" {
		req.Header.Set("X-Amz-Security-Token", token.SecurityToken)
	}

	// Make the request with a timeout
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling STS: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading STS response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		var errResp stsErrorResponse
		if err := xml.Unmarshal(bodyBytes, &errResp); err != nil {
			return "", fmt.Errorf("%w: HTTP %d: %s", ErrInvalidCredentials, resp.StatusCode, string(bodyBytes))
		}
		return "", mapAWSError(errResp.Error.Code, errResp.Error.Message)
	}

	// Parse success response
	var stsResp stsGetCallerIdentityResponse
	if err := xml.Unmarshal(bodyBytes, &stsResp); err != nil {
		return "", fmt.Errorf("parsing STS response: %w", err)
	}

	if stsResp.Result.Arn == "" {
		return "", fmt.Errorf("%w: no ARN in STS response", ErrInvalidCredentials)
	}

	return stsResp.Result.Arn, nil
}

// mapAWSError maps AWS error codes to nauts errors.
func mapAWSError(code, message string) error {
	switch code {
	case "InvalidClientTokenId":
		return fmt.Errorf("%w: invalid AWS credentials", ErrInvalidCredentials)
	case "SignatureDoesNotMatch":
		return fmt.Errorf("%w: AWS signature verification failed", ErrInvalidCredentials)
	case "RequestExpired":
		return fmt.Errorf("%w: AWS request expired", ErrInvalidCredentials)
	case "MissingAuthenticationToken":
		return fmt.Errorf("%w: missing AWS authentication token", ErrInvalidCredentials)
	default:
		return fmt.Errorf("AWS STS error %s: %s", code, message)
	}
}

// parseRoleARN parses an AWS ARN and extracts role information.
func parseRoleARN(arn string) (*parsedARN, error) {
	// ARN format: arn:aws:iam::123456789012:role/path/to/role
	//         or: arn:aws:sts::123456789012:assumed-role/role/session
	parts := strings.Split(arn, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("%w: invalid ARN format", ErrInvalidCredentials)
	}

	if parts[0] != "arn" || parts[1] != "aws" {
		return nil, fmt.Errorf("%w: invalid ARN prefix", ErrInvalidCredentials)
	}

	service := parts[2] // "iam" or "sts"
	awsAccountID := parts[4]
	resourcePath := strings.Join(parts[5:], ":") // Handle ARNs with colons in resource path

	if !awsAccountIDRegex.MatchString(awsAccountID) {
		return nil, fmt.Errorf("%w: invalid AWS account ID in ARN", ErrInvalidCredentials)
	}

	var roleName, sessionName string

	if service == "iam" {
		// IAM role: arn:aws:iam::123456789012:role/path/to/nauts.prod.admin
		if !strings.HasPrefix(resourcePath, "role/") {
			return nil, fmt.Errorf("%w: expected IAM role ARN", ErrInvalidCredentials)
		}
		// Extract role name (last part of path)
		pathParts := strings.Split(resourcePath[5:], "/") // Remove "role/" prefix
		roleName = pathParts[len(pathParts)-1]
	} else if service == "sts" {
		// Assumed role: arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/session-name
		if !strings.HasPrefix(resourcePath, "assumed-role/") {
			return nil, fmt.Errorf("%w: expected assumed-role ARN", ErrInvalidCredentials)
		}
		// Extract role name and session
		pathParts := strings.Split(resourcePath[13:], "/") // Remove "assumed-role/" prefix
		if len(pathParts) < 2 {
			return nil, fmt.Errorf("%w: malformed assumed-role ARN", ErrInvalidCredentials)
		}
		roleName = pathParts[0]
		sessionName = strings.Join(pathParts[1:], "/") // Session name might contain slashes
	} else {
		return nil, fmt.Errorf("%w: unsupported service in ARN: %s", ErrInvalidCredentials, service)
	}

	return &parsedARN{
		AwsAccountID: awsAccountID,
		RoleName:     roleName,
		SessionName:  sessionName,
		FullARN:      arn,
	}, nil
}

// validateAndParseRoleName validates the role name follows nauts.<account>.<role> pattern.
func validateAndParseRoleName(roleName string) (account, role string, err error) {
	parts := strings.Split(roleName, ".")
	if len(parts) != 3 {
		return "", "", fmt.Errorf("%w: expected 3 parts, got %d", ErrInvalidRoleFormat, len(parts))
	}

	if parts[0] != "nauts" {
		return "", "", fmt.Errorf("%w: must start with 'nauts'", ErrInvalidRoleFormat)
	}

	account = parts[1]
	role = parts[2]

	if !natsIdentifierRegex.MatchString(account) {
		return "", "", fmt.Errorf("%w: invalid account name: %s", ErrInvalidRoleFormat, account)
	}

	if !natsIdentifierRegex.MatchString(role) {
		return "", "", fmt.Errorf("%w: invalid role name: %s", ErrInvalidRoleFormat, role)
	}

	return account, role, nil
}

// constructUser builds a User object from the parsed ARN and role information.
func constructUser(parsedARN *parsedARN, account, role string) *User {
	attributes := map[string]string{
		"aws_account": parsedARN.AwsAccountID,
		"aws_role":    parsedARN.RoleName,
		"aws_arn":     parsedARN.FullARN,
	}

	if parsedARN.SessionName != "" {
		attributes["aws_session"] = parsedARN.SessionName
	}

	return &User{
		ID: parsedARN.FullARN,
		Roles: []Role{
			{Account: account, Name: role},
		},
		Attributes: attributes,
	}
}
