# Specification: AWS SigV4 Authentication Provider

**Date:** 2026-02-08  
**Status:** Draft  
**Package:** `identity` (new provider: `AwsSigV4AuthenticationProvider`)  
**Dependencies:** AWS SDK for Go v2 (`github.com/aws/aws-sdk-go-v2/service/sts`)

---

## Goal

Enable AWS IAM role-based authentication for NATS by verifying AWS SigV4-signed requests to AWS STS GetCallerIdentity. This allows AWS workloads (EC2, ECS, Lambda, etc.) to authenticate using their IAM roles without managing separate credentials.

## Summary

The `AwsSigV4AuthenticationProvider` accepts AWS SigV4 signature headers, calls AWS STS GetCallerIdentity to verify the identity and obtain the role ARN, then parses the ARN to extract NATS account and role information. The AWS IAM role name must follow the convention `nauts.<nats-account>.<nats-role>` to map AWS identities to NATS permissions.

---

## Scope

- Accept AWS SigV4 signed request headers in authentication token  
- Call AWS STS GetCallerIdentity to verify identity and obtain ARN  
- Parse role ARN and validate naming convention  
- Extract NATS account and role from AWS role name  
- Support both IAM roles and assumed roles  
- Timestamp validation to prevent replay attacks  

**Out of scope:** 
- AWS Cognito integration
- AWS session token management beyond verification
- Custom role name patterns (only `nauts.<account>.<role>` supported)
- Multi-region failover (uses single configured region)
- Caching of validated identities (every auth calls STS)

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **STS GetCallerIdentity for verification** | This API is the standard AWS identity verification endpoint. It accepts SigV4 signed requests and returns the authenticated caller's ARN. No special permissions required. |
| **Role name convention: `nauts.<account>.<role>`** | Embeds NATS account and role directly in AWS role name. Simple to parse and enforces clear mapping. Alternative (tags) would require IAM:GetRole permission. |
| **Token contains raw SigV4 headers** | Client generates SigV4 signature locally. Nauts validates by reconstructing and forwarding to AWS. This avoids nauts needing AWS credentials. |
| **Timestamp validation (5-minute window)** | SigV4 includes X-Amz-Date header. Validate timestamp freshness to prevent replay attacks. 5-minute window balances security and clock skew tolerance. |
| **AWS account ID as attribute** | Store AWS account ID separately from role name. Useful for audit logs and multi-account scenarios. |
| **Session name in User.ID** | For assumed roles, the full ARN includes session name (e.g., EC2 instance ID). This makes each authentication unique and enables proper audit trails. Different sessions = different User.ID values. |
| **Region extraction from signature** | SigV4 Authorization header contains region in credential scope. Extract region from signature if not configured. If region is configured, validate signature uses that region (security check against region confusion attacks). |
| **Token format: Simplified headers** | Client sends only essential SigV4 headers (Authorization, X-Amz-Date, optional X-Amz-Security-Token) in JSON. Simpler than full HTTP request reconstruction. Nauts builds minimal GetCallerIdentity request from these headers. |
| **AllowedAWSAccounts required** | Single AWS account per provider. Each provider is bound to exactly one AWS account ID. This simplifies configuration and makes security boundaries explicit. For multiple AWS accounts, create multiple provider instances. |
| **Enforce AuthRequest.Account matching** | Validate `AuthRequest.Account` matches the NATS account extracted from role name. Prevents confused deputy attacks where client requests wrong account. |
| **No caching** | Every authentication calls AWS STS. Future enhancement could cache valid ARNs with TTL, but initial implementation prioritizes correctness over performance. |

---

## Public API

### Types

#### `AwsSigV4AuthenticationProviderConfig`
```go
type AwsSigV4AuthenticationProviderConfig struct {
    // Accounts is the list of NATS account patterns this provider manages
    Accounts []string `json:"accounts"`
    
    // Region is the AWS region for STS endpoint (e.g., "us-east-1").
    // OPTIONAL: If not specified, the region is extracted from the Authorization
    // header's credential scope. If specified, validates that the signature's
    // region matches this value (security check).
    // 
    // Recommendation: Specify explicitly for:
    // - Better performance (regional endpoint closer to workloads)
    // - Security (validate client is using expected region)
    // - Clarity (explicit configuration)
    Region string `json:"region,omitempty"`
    
    // MaxClockSkew is the maximum allowed difference between request timestamp
    // and current time (default: 5 minutes)
    MaxClockSkew time.Duration `json:"maxClockSkew,omitempty"`
    
    // AWSAccount is the AWS account ID that is allowed to authenticate.
    // REQUIRED: Must be a 12-digit AWS account ID.
    // Wildcards are NOT allowed.
    // Example: "123456789012"
    AWSAccount string `json:"awsAccount"`
}
```

#### `AwsSigV4AuthenticationProvider`
```go
func NewAwsSigV4AuthenticationProvider(cfg AwsSigV4AuthenticationProviderConfig) (*AwsSigV4AuthenticationProvider, error)
func (p *AwsSigV4AuthenticationProvider) Verify(ctx context.Context, req AuthRequest) (*User, error)
func (p *AwsSigV4AuthenticationProvider) ManageableAccounts() []string
```

### Token Format

The `AuthRequest.Token` field contains a JSON object with essential AWS SigV4 headers (simplified format):

```json
{
  "authorization": "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20260208/us-east-1/sts/aws4_request, SignedHeaders=host;x-amz-date, Signature=abc123...",
  "date": "20260208T153045Z",
  "securityToken": "IQoJb3JpZ2luX2VjE..."  // Optional, for temporary credentials
}
```

### Configuration Examples

### With Explicit Region (Recommended for Production)
```json
{
  "auth": {
    "aws": [
      {
        "id": "aws-us-east-1",
        "accounts": ["prod-*", "staging-*"],
        "region": "us-east-1",
        "maxClockSkew": "5m",
        "awsAccount": "123456789012"
      }
    ]
  }
}
```

**Benefits:**
- Validates client is using expected region (security)
- Explicit configuration (clarity)
- Potentially better performance (can optimize endpoint)

### Without Region (Flexible)
```json
{
  "auth": {
    "aws": [
      {
        "id": "aws-multi-region",
        "accounts": ["prod-*"],
        "maxClockSkew": "5m",
        "awsAccount": "123456789012"
      }
    ]
  }
}
```

**Benefits:**
- Supports clients in any AWS region
- Region extracted from Authorization header credential scope
- Simpler configuration for multi-region deployments

### Multi-Region with Multiple Providers
```json
{
  "auth": {
    "aws": [
      {
        "id": "aws-us-east-1",
        "accounts": ["prod-us"],
        "region": "us-east-1",
        "awsAccount": "123456789012"
      },
      {
        "id": "aws-eu-west-1",
        "accounts": ["prod-eu"],
        "region": "eu-west-1",
        "awsAccount": "123456789012"
      }
    ]
  }
}
```

**Use case:** Separate NATS accounts per region with region enforcement

### Multi-Account Example
```json
{
  "auth": {
    "aws": [
      {
        "id": "aws-account-1",
        "accounts": ["prod-*"],
        "region": "us-east-1",
        "awsAccount": "123456789012"
      },
      {
        "id": "aws-account-2",
        "accounts": ["dev-*"],
        "region": "us-east-1",
        "awsAccount": "987654321098"
      }
    ]
  }
}
```

**Use case:** Multiple AWS accounts mapping to different NATS accounts

---

## Authentication Flow

```
Client (AWS workload)                nauts                        AWS STS
      │                                 │                            │
      │ 1. Generate SigV4 signature     │                            │
      │    for GetCallerIdentity        │                            │
      │                                 │                            │
      │ 2. Send AuthRequest             │                            │
      │    {account, token with headers}│                            │
      ├────────────────────────────────►│                            │
      │                                 │                            │
      │                                 │ 3. Parse token, extract    │
      │                                 │    SigV4 headers           │
      │                                 │                            │
      │                                 │ 4. Validate timestamp      │
      │                                 │    (check X-Amz-Date)      │
      │                                 │                            │
      │                                 │ 5. Reconstruct HTTP request│
      │                                 │    to GetCallerIdentity    │
      │                                 │                            │
      │                                 │ 6. Forward to STS          │
      │                                 ├───────────────────────────►│
      │                                 │                            │
      │                                 │ 7. Response with ARN       │
      │                                 │◄───────────────────────────┤
      │                                 │                            │
      │                                 │ 8. Parse ARN               │
      │                                 │    Extract role name       │
      │                                 │                            │
      │                                 │ 9. Validate role name      │
      │                                 │    pattern: nauts.X.Y      │
      │                                 │                            │
      │                                 │ 10. Build User object      │
      │                                 │     - ID from ARN          │
      │                                 │     - Role from name       │
      │                                 │     - AWS account attr     │
      │                                 │                            │
      │ 11. Return User                 │                            │
      │◄────────────────────────────────┤                            │
```

---

## ARN Parsing

### Expected ARN Format

```
arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/session-name
arn:aws:iam::123456789012:role/nauts.prod.readonly
```

### Parsing Algorithm

1. Split ARN by `:` → `["arn", "aws", "sts|iam", "", "AWS_ACCOUNT_ID", "role-path"]`
2. Extract AWS account ID (position 4)
3. Extract role path (position 5+)
4. Handle two cases:
   - **IAM role:** `role/nauts.<account>.<role>` → extract role name
   - **Assumed role:** `assumed-role/nauts.<account>.<role>/session` → extract role name, ignore session
5. Split role name by `.` → `["nauts", "nats-account", "nats-role"]`
6. Validate:
   - Exactly 3 parts
   - First part is `"nauts"`
   - Second part is valid NATS account (matches requested account or pattern)
   - Third part is valid NATS role name (alphanumeric, hyphen, underscore)

### Examples

| AWS IAM Role Name | NATS Account | NATS Role | Valid? |
|-------------------|--------------|-----------|:------:|
| `nauts.prod.admin` | `prod` | `admin` | ✅ |
| `nauts.staging-app.readonly` | `staging-app` | `readonly` | ✅ |
| `nauts.prod.data-engineer` | `prod` | `data-engineer` | ✅ |
| `my-app-role` | — | — | ❌ (wrong format) |
| `nauts.prod` | — | — | ❌ (missing role) |
| `nauts.prod.admin.extra` | — | — | ❌ (too many parts) |
| `nauts.prod.admin/session-123` | `prod` | `admin` | ✅ (session ignored) |

---

## User Construction

### Full ARN as User.ID (Confirmed)

The User.ID is set to the **full ARN** returned by AWS STS GetCallerIdentity, including the session name for assumed roles. This provides:
- ✅ Unique identity per authentication session
- ✅ Full audit trail (which EC2 instance, Lambda invocation, etc.)
- ✅ Consistent with AWS identity model

```go
// Example for assumed role: arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234
User{
    ID: "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
    Roles: []AccountRole{
        {Account: "prod", Role: "admin"},
    },
    Attributes: map[string]string{
        "aws_account": "123456789012",
        "aws_role": "nauts.prod.admin",
        "aws_session": "i-0abcd1234",  // For assumed roles only
        "aws_arn": "arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-0abcd1234",
    },
}

// Example for IAM role: arn:aws:iam::123456789012:role/nauts.staging.readonly
User{
    ID: "arn:aws:iam::123456789012:role/nauts.staging.readonly",
    Roles: []AccountRole{
        {Account: "staging", Role: "readonly"},
    },
    Attributes: map[string]string{
        "aws_account": "123456789012",
        "aws_role": "nauts.staging.readonly",
        "aws_arn": "arn:aws:iam::123456789012:role/nauts.staging.readonly",
    },
}
```

**Note:** Different sessions of the same role (e.g., two EC2 instances with the same role) will have different User.ID values due to unique session names. This is correct behavior - each authentication is independent.

---

## Error Handling

### Sentinel Errors

| Error | When | HTTP Equivalent |
|-------|------|-----------------|
| `ErrInvalidCredentials` | AWS rejects signature, timestamp too old/new, malformed headers | 401 |
| `ErrInvalidTokenType` | Token not in expected JSON format | 400 |
| `ErrInvalidAccount` | Role name doesn't match requested NATS account | 403 |
| `ErrInvalidRoleFormat` | AWS role name doesn't follow `nauts.<account>.<role>` pattern | 403 |
| `ErrAWSAccountNotAllowed` | AWS account ID does not match configured account | 403 |

### AWS API Errors

| AWS Error | nauts Mapping |
|-----------|---------------|
| `InvalidClientTokenId` | `ErrInvalidCredentials` |
| `SignatureDoesNotMatch` | `ErrInvalidCredentials` |
| `RequestExpired` | `ErrInvalidCredentials` + log timestamp difference |
| `MissingAuthenticationToken` | `ErrInvalidCredentials` |
| `Throttling` | Return AWS error wrapped, caller should retry |
| Network timeout | Return AWS error wrapped |

---

## Security Considerations

### Timestamp Validation

**Problem:** Replay attacks - attacker captures valid signed request, replays later.

**Solution:** Validate X-Amz-Date header:
```go
requestTime := parseAmzDate(headers["X-Amz-Date"])
now := time.Now()
if now.Sub(requestTime).Abs() > maxClockSkew {
    return ErrInvalidCredentials // "request timestamp too old or too new"
}
```

**Default:** `maxClockSkew = 5 minutes`

### AWS Account Restrictions

**Problem:** Any AWS account can create role `nauts.prod.admin` and authenticate.

**Solution:** Each provider is bound to a single AWS account (REQUIRED):
```json
{
  "awsAccount": "123456789012"
}
```

**Enforcement:**
- Provider constructor MUST validate `awsAccount` is not empty
- Provider constructor MUST reject wildcard values (\"*\")
- AWS account ID must be exactly 12 digits
- Authentication MUST verify caller's AWS account matches configured account
- For multiple AWS accounts, create multiple provider instances
Region Configuration - Required or optional?** ✅ **RESOLVED**
   - **Decision:** Region is OPTIONAL in configuration
   - If not specified: Extract region from Authorization header credential scope
   - If specified: Validate that signature's region matches (security check)
   - **Rationale:** Provides flexibility while allowing security-conscious deployments to enforce specific regions
   - **Implementation:** Parse Authorization header: `Credential=.../20260208/us-east-1/sts/aws4_request` → extract "us-east-1"

**Solution:** Validate role name components:
- NATS account: `[a-zA-Z0-9_\-]+` (no dots, slashes, wildcards)
- NATS role: `[a-zA-Z0-9_\-]+` (no dots, slashes, wildcards)

Reject if validation fails.

---

## Ambiguities & Open Questions

### 🔴 Critical Ambiguities

1. **Token Format - Which headers are required?** ✅ RESOLVED
   - ~~Option A: Full HTTP request (method, URL, headers, body)~~
   - ✅ **Option B: Just Authorization + X-Amz-Date + optional Security Token**
   - **Decision:** Option B (simplified). Clients send only essential SigV4 headers in JSON format.

2. **STS Endpoint - Which URL to use?**
   - Global: `https://sts.amazonaws.com` (simple but slower for non-US-East-1)
   - Regional: `https://sts.{region}.amazonaws.com` (faster but requires region config)
   - **Recommendation:** Regional with explicit config (`region` field required)

3. **Requested Account Redundancy** ✅ RESOLVED
   - The `AuthRequest.Account` is redundant with the role name (which contains NATS account)
   - ✅ **Decision:** Validate match. Fail authentication if `AuthRequest.Account != extracted_account`
   - **Rationale:** Prevents confused deputy attacks

4. **Multiple AWS Accounts → Same NATS Account** ✅ RESOLVED
   - Can two different AWS accounts both have `nauts.prod.admin` roles?
   - Would they get same NATS permissions? (Yes)
   - Is this a security issue? (Yes, without restrictions)
   - ✅ **Decision:** Each provider is bound to a single AWS account (one-to-one mapping)
   - **Multi-account support:** Create multiple provider instances, one per AWS account

5. **Session Name Handling**
   - Assumed roles have format: `assumed-role/nauts.X.Y/session-name`
   - Session name is unique per assumption (EC2 instance ID, Lambda request ID, etc.)
   - Should session be part of User.ID? (Yes for uniqueness)
   - Should session be an attribute? (Yes for logging)

### 🟡 Design Questions

6. **Caching Strategy**
   - Every auth calls AWS (100-200ms latency + cost)
   - Should we cache validated ARNs?
   - Cache key: Hash of (Authorization header, X-Amz-Date)?
   - Cache TTL: Related to timestamp window (5 min)?
   - **Recommendation:** No caching in v1. Add as future enhancement.

7. **Concurrent Requests Spike**
   - If 1000 clients authenticate simultaneously, 1000 STS calls
   - AWS STS has rate limits (burst: 200/sec, sustained: 20/sec default)
   - Need request coalescing? (dedupe identical in-flight requests)
   - **Recommendation:** Document limitation. Consider rate limiter in future.

8. **Federated Users vs Roles**
   - GetCallerIdentity also works for federated users: `arn:aws:sts::123456789012:federated-user/alice`
   - Should we support this? Pattern: `nauts.prod.admin` as federated user name?
   - **Recommendation:** Roles only in v1. Federated users are complex.

9. **Service-Linked Roles**
   - AWS service-linked roles have path: `role/aws-service-role/...`
   - These can't follow `nauts.X.Y` pattern
   - Should we reject? (Yes, these are for AWS services)
   - **Recommendation:** Explicitly reject service-linked roles

10. **Cross-Account Assumed Roles**
    - Role in Account A assumed by principal from Account B
    - ARN shows Account A (where role is defined)
    - Is this correct for nauts? (Yes, role defines permissions)
    - **Recommendation:** Works as-is, but document behavior

### 🟢 Implementation Details

11. **HTTP Client Configuration**
    - Timeout for STS calls? (Recommend: 5 seconds)
    - Retry policy? (AWS SDK default: 3 retries with exponential backoff)
    - Connection pooling? (Yes, reuse HTTP client)

12. **Logging**
    - Log AWS account ID? (Yes, for audit)
    - Log full ARN? (Yes, but sanitize in public logs)
    - Log STS call duration? (Yes, for monitoring)

13. **Metrics**
    - Track STS call latency (p50, p95, p99)
    - Track STS error rate by type
    - Track authenticated AWS accounts

---

## Possible Bugs & Edge Cases

### 🐛 Potential Implementation Bugs

1. **Timezone Parsing Bug**
   - X-Amz-Date format: `20260208T153045Z` (ISO 8601 basic format)
   - Go time.Parse needs exact layout: `"20060102T150405Z"`
   - **Bug risk:** Using wrong layout → all requests rejected
   - **Test:** Various date formats, edge of month/year

2. **ARN Split Bug**
   - ARN with role path: `arn:aws:iam::123456789012:role/path/to/nauts.prod.admin`
   - Naive split by `/` gives wrong result
   - **Bug risk:** Extracting wrong part as role name
   - **Test:** Roles with paths, nested paths

3. **Unicode in Role Names**
   - AWS allows Unicode in role names (though not recommended)
   - Regex `[a-zA-Z0-9_\-]+` rejects Unicode
   - **Bug risk:** Valid AWS role rejected
   - **Recommendation:** Explicitly document ASCII-only restriction

4. **Whitespace in Headers**
   - HTTP headers can have leading/trailing whitespace
   - AWS signature includes exact header values
   - **Bug risk:** Trim whitespace → signature mismatch
   - **Test:** Headers with various whitespace

5. **Case Sensitivity**
   - HTTP headers are case-insensitive, but SigV4 signs exact case
   - Must preserve exact case from client
   - **Bug risk:** Normalizing case → signature mismatch
   - **Test:** Various header case combinations

6. **Signature Replay Across Regions**
   - SigV4 signature includes region in credential scope
   - Request signed for us-east-1 won't work with us-west-2 endpoint
   - **Bug risk:** Provider sends to wrong region → InvalidSignature
   - **Test:** Token signed for different region than provider config

### 🎯 Edge Cases to Test

| Edge Case | Expected Behavior |
|-----------|-------------------|
| Empty token | `ErrInvalidTokenType` |
| Missing Authorization header | `ErrInvalidCredentials` |
| Missing X-Amz-Date header | `ErrInvalidCredentials` |
| Timestamp exactly 5 minutes old | Accept (boundary) |
| Timestamp 5 min 1 sec old | Reject `ErrInvalidCredentials` |
| Clock skew (server ahead) | Accept if within window |
| Clock skew (server behind) | Accept if within window |
| Role name `nauts.PROD.admin` | Match case-sensitive or insensitive? |
| AWS account ID with leading zeros | Parse as string, not int |
| Session name with slashes | Extract correctly from ARN |
| IAM role vs assumed role | Both work, different ARN patterns |
| Role in nested path `/apps/nauts.prod.admin` | Extract role name correctly |
| Concurrent identical requests | Both call STS (no dedup in v1) |
| STS throttling error | Return error, client retries |
| Network timeout | Return error, client retries |
| Malformed ARN from AWS | Parse error, `ErrInvalidCredentials` |

---

## Testing Strategy

### Unit Tests
- ARN parsing with various formats
- Role name validation (valid/invalid patterns)
- Timestamp validation at boundaries
- Error mapping from AWS SDK

### Integration Tests  
- Mock STS service returning various ARNs
- Test with real AWS SDK client against mock
- Verify HTTP request construction

### E2E Tests
- Requires AWS credentials for actual IAM role
- Use test AWS account with test roles
- Verify full authentication flow

---

## Future Enhancements

| Enhancement | Priority | Description |
|------------|:--------:|-------------|
| **Caching** | High | Cache validated ARNs with TTL to reduce STS calls |
| **Request coalescing** | Medium | Deduplicate concurrent identical requests |
| **Multi-region failover** | Low | Try alternate regions on timeout |
| **Federated user support** | Low | Support `federated-user` ARNs |
| **Custom role patterns** | Low | Allow configurable regex for role name format |
| **AWS Organizations support** | Low | Validate account is in specific AWS Organization |
| **Metrics/observability** | Medium | Expose STS call metrics and latency |
| **Rate limiting** | Medium | Limit STS calls per second to avoid throttling |

---

## Example Client Implementation (AWS SDK Go v2)

```go
// Client-side: Generate SigV4 signature for GetCallerIdentity
func generateAuthToken() (string, error) {
    // Load AWS credentials from environment/instance profile
    cfg, _ := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))
    
    // Create STS client
    stsClient := sts.NewFromConfig(cfg)
    
    // Build GetCallerIdentity request
    req, _ := stsClient.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
    
    // Sign request (generates Authorization header)
    _ = req.Sign()
    
    // Extract signed headers
    token := map[string]interface{}{
        "authorization": req.HTTPRequest.Header.Get("Authorization"),
        "date": req.HTTPRequest.Header.Get("X-Amz-Date"),
    }
    
    if sessionToken := req.HTTPRequest.Header.Get("X-Amz-Security-Token"); sessionToken != "" {
        token["securityToken"] = sessionToken
    }
    
    tokenJSON, _ := json.Marshal(token)
    return string(tokenJSON), nil
}
```

---

## References

- [AWS SigV4 Signing](https://docs.aws.amazon.com/general/latest/gr/signature-version-4.html)
- [STS GetCallerIdentity API](https://docs.aws.amazon.com/STS/latest/APIReference/API_GetCallerIdentity.html)
- [IAM ARN Format](https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html)
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/)
