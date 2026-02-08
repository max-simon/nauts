package identity

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAwsSigV4AuthenticationProvider(t *testing.T) {
	tests := []struct {
		name    string
		config  AwsSigV4AuthenticationProviderConfig
		wantErr string
	}{
		{
			name: "valid config",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod", "staging"},
				AWSAccount: "123456789012",
			},
			wantErr: "",
		},
		{
			name: "empty awsAccount",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod"},
				AWSAccount: "",
			},
			wantErr: "awsAccount is required",
		},
		{
			name: "wildcard awsAccount",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod"},
				AWSAccount: "*",
			},
			wantErr: "awsAccount must not contain wildcards",
		},
		{
			name: "wildcard pattern in awsAccount",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod"},
				AWSAccount: "123456*",
			},
			wantErr: "awsAccount must not contain wildcards",
		},
		{
			name: "invalid AWS account ID format",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod"},
				AWSAccount: "123",
			},
			wantErr: "invalid aws account ID format",
		},
		{
			name: "non-numeric AWS account ID",
			config: AwsSigV4AuthenticationProviderConfig{
				Accounts:   []string{"prod"},
				AWSAccount: "abcd12345678",
			},
			wantErr: "invalid aws account ID format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewAwsSigV4AuthenticationProvider(tt.config)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, provider)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.config.Accounts, provider.ManageableAccounts())
			}
		})
	}
}

func TestParseAwsSigV4Token(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		want    *sigV4Token
		wantErr error
	}{
		{
			name: "valid token",
			token: `{
				"authorization": "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request",
				"date": "20260208T153045Z"
			}`,
			want: &sigV4Token{
				Authorization: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request",
				Date:          "20260208T153045Z",
			},
			wantErr: nil,
		},
		{
			name: "valid token with security token",
			token: `{
				"authorization": "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request",
				"date": "20260208T153045Z",
				"securityToken": "IQoJb3JpZ2luX2VjE..."
			}`,
			want: &sigV4Token{
				Authorization: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request",
				Date:          "20260208T153045Z",
				SecurityToken: "IQoJb3JpZ2luX2VjE...",
			},
			wantErr: nil,
		},
		{
			name:    "malformed JSON",
			token:   `{"authorization": "test"`,
			want:    nil,
			wantErr: ErrInvalidTokenType,
		},
		{
			name:    "missing authorization",
			token:   `{"date": "20260208T153045Z"}`,
			want:    nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name:    "missing date",
			token:   `{"authorization": "AWS4-HMAC-SHA256 Credential=..."}`,
			want:    nil,
			wantErr: ErrInvalidCredentials,
		},
		{
			name:    "empty authorization",
			token:   `{"authorization": "", "date": "20260208T153045Z"}`,
			want:    nil,
			wantErr: ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAwsSigV4Token(tt.token)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidateTimestamp(t *testing.T) {
	now := time.Now()
	maxSkew := 5 * time.Minute

	tests := []struct {
		name    string
		amzDate string
		wantErr bool
	}{
		{
			name:    "current time",
			amzDate: now.UTC().Format("20060102T150405Z"),
			wantErr: false,
		},
		{
			name:    "4 minutes ago",
			amzDate: now.Add(-4 * time.Minute).UTC().Format("20060102T150405Z"),
			wantErr: false,
		},
		{
			name:    "exactly 5 minutes ago (boundary)",
			amzDate: now.Add(-5*time.Minute + time.Second).UTC().Format("20060102T150405Z"),
			wantErr: false,
		},
		{
			name:    "5 minutes 1 second ago (rejected)",
			amzDate: now.Add(-5*time.Minute - time.Second).UTC().Format("20060102T150405Z"),
			wantErr: true,
		},
		{
			name:    "4 minutes in future",
			amzDate: now.Add(4 * time.Minute).UTC().Format("20060102T150405Z"),
			wantErr: false,
		},
		{
			name:    "6 minutes in future (rejected)",
			amzDate: now.Add(6 * time.Minute).UTC().Format("20060102T150405Z"),
			wantErr: true,
		},
		{
			name:    "malformed format",
			amzDate: "2026-02-08T15:30:45Z",
			wantErr: true,
		},
		{
			name:    "invalid date",
			amzDate: "20261399T999999Z",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimestamp(tt.amzDate, maxSkew)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractRegionFromAuthorization(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		want       string
		wantErr    bool
	}{
		{
			name:       "us-east-1",
			authHeader: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123",
			want:       "us-east-1",
			wantErr:    false,
		},
		{
			name:       "eu-west-1",
			authHeader: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/eu-west-1/sts/aws4_request",
			want:       "eu-west-1",
			wantErr:    false,
		},
		{
			name:       "ap-southeast-2",
			authHeader: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/ap-southeast-2/sts/aws4_request",
			want:       "ap-southeast-2",
			wantErr:    false,
		},
		{
			name:       "missing Credential",
			authHeader: "AWS4-HMAC-SHA256 SignedHeaders=host",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "malformed credential scope (too few parts)",
			authHeader: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1",
			want:       "",
			wantErr:    true,
		},
		{
			name:       "empty region",
			authHeader: "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208//sts/aws4_request",
			want:       "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractRegionFromAuthorization(tt.authHeader)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseRoleARN(t *testing.T) {
	tests := []struct {
		name    string
		arn     string
		want    *parsedARN
		wantErr bool
	}{
		{
			name: "IAM role without path",
			arn:  "arn:aws:iam::123456789012:role/nauts.prod.admin",
			want: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "",
				FullARN:      "arn:aws:iam::123456789012:role/nauts.prod.admin",
			},
			wantErr: false,
		},
		{
			name: "IAM role with path",
			arn:  "arn:aws:iam::123456789012:role/apps/nauts.prod.admin",
			want: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "",
				FullARN:      "arn:aws:iam::123456789012:role/apps/nauts.prod.admin",
			},
			wantErr: false,
		},
		{
			name: "IAM role with nested path",
			arn:  "arn:aws:iam::123456789012:role/a/b/c/nauts.prod.admin",
			want: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "",
				FullARN:      "arn:aws:iam::123456789012:role/a/b/c/nauts.prod.admin",
			},
			wantErr: false,
		},
		{
			name: "assumed role",
			arn:  "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
			want: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "i-0abcd1234",
				FullARN:      "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
			},
			wantErr: false,
		},
		{
			name: "assumed role with session containing slash",
			arn:  "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/session/with/slashes",
			want: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "session/with/slashes",
				FullARN:      "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/session/with/slashes",
			},
			wantErr: false,
		},
		{
			name:    "invalid ARN format (too few parts)",
			arn:     "arn:aws:iam::123456789012",
			wantErr: true,
		},
		{
			name:    "invalid ARN prefix",
			arn:     "not:aws:iam::123456789012:role/test",
			wantErr: true,
		},
		{
			name:    "invalid AWS account ID",
			arn:     "arn:aws:iam::abc:role/nauts.prod.admin",
			wantErr: true,
		},
		{
			name:    "unsupported service",
			arn:     "arn:aws:s3::123456789012:bucket/test",
			wantErr: true,
		},
		{
			name:    "malformed assumed-role (missing session)",
			arn:     "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRoleARN(tt.arn)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestValidateAndParseRoleName(t *testing.T) {
	tests := []struct {
		name        string
		roleName    string
		wantAccount string
		wantRole    string
		wantErr     bool
	}{
		{
			name:        "valid simple role",
			roleName:    "nauts.prod.admin",
			wantAccount: "prod",
			wantRole:    "admin",
			wantErr:     false,
		},
		{
			name:        "valid with hyphen",
			roleName:    "nauts.staging-app.readonly",
			wantAccount: "staging-app",
			wantRole:    "readonly",
			wantErr:     false,
		},
		{
			name:        "valid with underscore",
			roleName:    "nauts.prod_data.data_engineer",
			wantAccount: "prod_data",
			wantRole:    "data_engineer",
			wantErr:     false,
		},
		{
			name:     "wrong prefix",
			roleName: "myapp.prod.admin",
			wantErr:  true,
		},
		{
			name:     "too few parts",
			roleName: "nauts.prod",
			wantErr:  true,
		},
		{
			name:     "too many parts",
			roleName: "nauts.prod.admin.extra",
			wantErr:  true,
		},
		{
			name:     "empty account",
			roleName: "nauts..admin",
			wantErr:  true,
		},
		{
			name:     "empty role",
			roleName: "nauts.prod.",
			wantErr:  true,
		},
		{
			name:     "invalid chars in account",
			roleName: "nauts.prod@test.admin",
			wantErr:  true,
		},
		{
			name:     "invalid chars in role",
			roleName: "nauts.prod.admin!",
			wantErr:  true,
		},
		{
			name:     "dots in account name",
			roleName: "nauts.prod.staging.admin",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, role, err := validateAndParseRoleName(tt.roleName)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantAccount, account)
				assert.Equal(t, tt.wantRole, role)
			}
		})
	}
}

func TestConstructUser(t *testing.T) {
	tests := []struct {
		name      string
		parsedARN *parsedARN
		account   string
		role      string
		want      *User
	}{
		{
			name: "IAM role",
			parsedARN: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "",
				FullARN:      "arn:aws:iam::123456789012:role/nauts.prod.admin",
			},
			account: "prod",
			role:    "admin",
			want: &User{
				ID: "arn:aws:iam::123456789012:role/nauts.prod.admin",
				Roles: []AccountRole{
					{Account: "prod", Role: "admin"},
				},
				Attributes: map[string]string{
					"aws_account": "123456789012",
					"aws_role":    "nauts.prod.admin",
					"aws_arn":     "arn:aws:iam::123456789012:role/nauts.prod.admin",
				},
			},
		},
		{
			name: "assumed role with session",
			parsedARN: &parsedARN{
				AwsAccountID: "123456789012",
				RoleName:     "nauts.prod.admin",
				SessionName:  "i-0abcd1234",
				FullARN:      "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
			},
			account: "prod",
			role:    "admin",
			want: &User{
				ID: "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
				Roles: []AccountRole{
					{Account: "prod", Role: "admin"},
				},
				Attributes: map[string]string{
					"aws_account": "123456789012",
					"aws_role":    "nauts.prod.admin",
					"aws_session": "i-0abcd1234",
					"aws_arn":     "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := constructUser(tt.parsedARN, tt.account, tt.role)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVerify_AccountValidation(t *testing.T) {
	// This test verifies that Verify enforces AuthRequest.Account matching
	// We'll need to mock the STS call for this

	// For now, we can test the logic path without actual AWS calls
	// Full integration tests will be added later

	t.Skip("Integration test - requires mock STS service")
}
