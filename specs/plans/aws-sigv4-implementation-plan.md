# AWS SigV4 Authentication Provider - Implementation Plan

**Spec:** [2026-02-08-aws-sigv4-authentication.md](../specs/2026-02-08-aws-sigv4-authentication.md)  
**Status:** Ready for implementation  
**Estimated Effort:** 3-5 days development + 2 days testing

---

## Phase 0: Pre-Implementation Decisions ✅ COMPLETE

**All critical decisions have been made and documented in the specification.**

### Task 0.1: Resolve Token Format ✅ COMPLETE
- [x] Review token format options in spec (Section: Ambiguities #1)
- [x] **Decision: Use Option B (simplified headers)**
  ```json
  {
    "authorization": "AWS4-HMAC-SHA256 Credential=...",
    "date": "20260208T153045Z",
    "securityToken": "IQoJb3..."  // Optional
  }
  ```
- [x] Document decision in spec
- [x] Update spec with final token schema

**Owner:** @max-simon  
**Completed:** 2026-02-08

### Task 0.2: Confirm Security Model ✅ COMPLETE
- [x] Review `allowedAwsAccounts` requirement (Spec: Ambiguities #4)
- [x] **Decision: `allowedAwsAccounts` MUST be non-empty and MUST NOT contain wildcards**
- [x] Update provider config validation logic in spec
- [x] Validation requirements:
  - Constructor fails if `allowedAwsAccounts` is empty
  - Constructor fails if any entry is "*" or contains wildcards
  - Each entry must be exactly 12-digit AWS account ID

**Owner:** @max-simon  
**Completed:** 2026-02-08

### Task 0.3: Confirm Validation Behavior ✅ COMPLETE
- [x] Review `AuthRequest.Account` validation (Spec: Ambiguities #3)
- [x] **Decision: Validate match between AuthRequest.Account and extracted account**
- [x] Update spec with validation logic
- [x] Implementation requirement:
  - Fail authentication if `AuthRequest.Account != extracted_account_from_role_name`
  - Return `ErrInvalidAccount` error
  - Prevents confused deputy attacks

**Owner:** @max-simon  
**Completed:** 2026-02-08

---

## Phase 1: Core Implementation

### Task 1.1: Create Provider Package Structure
- [ ] Create `identity/aws_sigv4_authentication_provider.go`
- [ ] Define `AwsSigV4AuthenticationProviderConfig` struct
- [ ] Define `AwsSigV4AuthenticationProvider` struct
- [ ] Add constructor: `NewAwsSigV4AuthenticationProvider(cfg Config) (*Provider, error)`
- [ ] Implement `ManageableAccounts() []string` method

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** None  
**Estimated:** 1 hour

### Task 1.2: Implement Token Parsing
- [ ] Define token JSON structure (based on Task 0.1 decision)
- [ ] Implement `parseAwsSigV4Token(tokenStr string) (*sigV4Token, error)`
- [ ] Extract Authorization header
- [ ] Extract X-Amz-Date header
- [ ] Extract optional X-Amz-Security-Token
- [ ] Validate required fields present
- [ ] Return structured token object

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.1  
**Estimated:** 2 hours

### Task 1.3: Implement Timestamp Validation
- [ ] Parse X-Amz-Date format: `20060102T150405Z`
- [ ] Implement `validateTimestamp(amzDate string, maxSkew time.Duration) error`
- [ ] Check timestamp is within maxSkew window (default 5 minutes)
- [ ] Handle both past and future timestamps
- [ ] Return clear error messages with time difference

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.2  
**Estimated:** 1 hour

### Task 1.4: Implement Region Extraction
- [ ] Parse Authorization header credential scope
- [ ] Extract region from: `Credential=.../DATE/REGION/SERVICE/aws4_request`
- [ ] Implement `extractRegionFromAuthorization(authHeader string) (string, error)`
- [ ] Handle malformed credential scopes
- [ ] Validate region is AWS-valid format

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.2  
**Estimated:** 1.5 hours

### Task 1.5: Integrate AWS STS via HTTP ✅ COMPLETE
- [x] Implement direct HTTP request to STS endpoint
- [x] Build POST request with form-encoded body
- [x] Set SigV4 headers from token (Authorization, X-Amz-Date, X-Amz-Security-Token)
- [x] Parse XML response from AWS STS
- [x] Handle AWS error responses
- [x] Map AWS error codes to nauts errors
- [x] Set 5-second timeout for HTTP client

**Decision:** Use direct HTTP requests instead of AWS SDK to avoid dependency bloat.  
**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.4  
**Completed:** 2026-02-08

### Task 1.6: Implement STS GetCallerIdentity Call ✅ COMPLETE
- [x] Build HTTP POST request to https://sts.{region}.amazonaws.com/
- [x] Set request body: Action=GetCallerIdentity&Version=2011-06-15
- [x] Inject SigV4 headers from token
- [x] Parse XML response structure (GetCallerIdentityResponse)
- [x] Handle AWS error responses (ErrorResponse XML)
- [x] Extract ARN from response
- [x] Map AWS error codes to nauts errors (InvalidClientTokenId, SignatureDoesNotMatch, etc.)

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.5  
**Completed:** 2026-02-08

### Task 1.7: Implement ARN Parsing
- [ ] Implement `parseRoleARN(arn string) (*parsedARN, error)`
- [ ] Split ARN by `:` delimiter
- [ ] Extract AWS account ID
- [ ] Handle IAM role format: `arn:aws:iam::ACCOUNT:role/PATH/NAME`
- [ ] Handle assumed role format: `arn:aws:sts::ACCOUNT:assumed-role/NAME/SESSION`
- [ ] Handle roles with nested paths
- [ ] Extract role name and optional session name

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.6  
**Estimated:** 2 hours

### Task 1.8: Implement Role Name Validation
- [ ] Parse role name format: `nauts.ACCOUNT.ROLE`
- [ ] Split by `.` delimiter
- [ ] Validate exactly 3 parts
- [ ] Validate first part is "nauts"
- [ ] Validate NATS account name (alphanumeric, hyphen, underscore)
- [ ] Validate NATS role name (alphanumeric, hyphen, underscore)
- [ ] Return extracted account and role

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.7  
**Estimated:** 1.5 hours

### Task 1.9: Implement User Construction
- [ ] Build `identity.User` struct
- [ ] Set `ID` to full ARN (including session)
- [ ] Create `AccountRole` with extracted account and role
- [ ] Populate attributes:
  - `aws_account`: AWS account ID
  - `aws_role`: Role name from ARN
  - `aws_session`: Session name (if assumed role)
  - `aws_arn`: Full ARN
- [ ] Return User object

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Task 1.8  
**Estimated:** 1 hour

### Task 1.10: Implement Verify Method
- [ ] Implement `Verify(ctx context.Context, req AuthRequest) (*User, error)`
- [ ] Orchestrate all steps:
  1. Parse token (Task 1.2)
  2. Validate timestamp (Task 1.3)
  3. Extract region (Task 1.4)
  4. Validate region match if configured
  5. Call STS (Task 1.6)
  6. Parse ARN (Task 1.7)
  7. Validate role name (Task 1.8)
  8. Validate AWS account if allowedAwsAccounts configured
  9. Validate AuthRequest.Account matches extracted account (Task 0.3)
  10. Construct User (Task 1.9)
- [ ] Add comprehensive error handling
- [ ] Add logging at key steps

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Tasks 1.2-1.9  
**Estimated:** 2 hours

---

## Phase 2: Configuration Integration

### Task 2.1: Add Config Types
- [ ] Update `auth/config.go`
- [ ] Add `AwsAuthProviderConfig` struct
- [ ] Add `Aws []AwsAuthProviderConfig` field to `AuthConfig`
- [ ] Add JSON tags

**Files:** `auth/config.go`  
**Dependencies:** None  
**Estimated:** 30 minutes

### Task 2.2: Integrate with Config Loading
- [ ] Update `NewAuthControllerWithConfig` in `auth/config.go`
- [ ] Load AWS provider configs
- [ ] Create `AwsSigV4AuthenticationProvider` instances
- [ ] Register with `AuthenticationProviderManager`
- [ ] Validate unique provider IDs

**Files:** `auth/config.go`  
**Dependencies:** Task 2.1, Task 1.10  
**Estimated:** 1 hour

### Task 2.3: Add Config Validation
- [ ] Validate required fields in `AwsAuthProviderConfig`
- [ ] Validate region format (if provided)
- [ ] Validate maxClockSkew is positive
- [ ] Validate allowedAwsAccounts is non-empty (REQUIRED)
- [ ] Validate allowedAwsAccounts contains no wildcards ("*")
- [ ] Validate allowedAwsAccounts format (12-digit numbers)

**Files:** `auth/config.go`  
**Dependencies:** Task 2.1  
**Estimated:** 1 hour

---

## Phase 3: Unit Testing

### Task 3.1: Test ARN Parsing
- [ ] Create `identity/aws_sigv4_authentication_provider_test.go`
- [ ] Test IAM role ARN: `arn:aws:iam::123456789012:role/nauts.prod.admin`
- [ ] Test assumed role ARN: `arn:aws:sts::123456789012:assumed-role/nauts.prod.admin/i-123`
- [ ] Test role with path: `arn:aws:iam::123456789012:role/apps/nauts.prod.admin`
- [ ] Test nested path: `arn:aws:iam::123456789012:role/a/b/c/nauts.prod.admin`
- [ ] Test invalid ARN format
- [ ] Test malformed role name
- [ ] Test service-linked role (should fail)
- [ ] Minimum 15 test cases (per spec)

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.7  
**Estimated:** 2 hours

### Task 3.2: Test Role Name Validation
- [ ] Test valid patterns: `nauts.prod.admin`, `nauts.staging-app.readonly`
- [ ] Test invalid patterns: `my-app-role`, `nauts.prod`, `nauts.prod.admin.extra`
- [ ] Test case sensitivity
- [ ] Test special characters (should fail)
- [ ] Test empty parts
- [ ] Minimum 10 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.8  
**Estimated:** 1 hour

### Task 3.3: Test Timestamp Validation
- [ ] Test valid timestamp (within 5 min)
- [ ] Test timestamp exactly 5 min old (boundary)
- [ ] Test timestamp 5 min 1 sec old (rejected)
- [ ] Test future timestamp (within 5 min)
- [ ] Test far future timestamp (rejected)
- [ ] Test malformed timestamp format
- [ ] Test clock skew scenarios
- [ ] Minimum 8 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.3  
**Estimated:** 1.5 hours

### Task 3.4: Test Region Extraction
- [ ] Test standard Authorization header format
- [ ] Test various AWS regions
- [ ] Test malformed Authorization header
- [ ] Test missing credential scope
- [ ] Test invalid credential scope parts
- [ ] Minimum 6 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.4  
**Estimated:** 1 hour

### Task 3.5: Test Token Parsing
- [ ] Test valid token JSON
- [ ] Test missing required fields
- [ ] Test malformed JSON
- [ ] Test with and without securityToken
- [ ] Test empty fields
- [ ] Minimum 6 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.2  
**Estimated:** 1 hour

### Task 3.6: Test AWS Account Validation
- [ ] Test allowed AWS account (accepted)
- [ ] Test disallowed AWS account (rejected)
- [ ] Test empty allowedAwsAccounts list (behavior per Task 0.2)
- [ ] Minimum 4 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.10  
**Estimated:** 30 minutes

### Task 3.7: Test Region Validation
- [ ] Test matching region (accepted)
- [ ] Test mismatched region (rejected)
- [ ] Test no region configured (extracted region used)
- [ ] Minimum 4 test cases

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 1.10  
**Estimated:** 30 minutes

---

## Phase 4: Integration Testing

### Task 4.1: Create Mock STS Service
- [ ] Create `identity/testdata/mock_sts_server.go` or use httptest
- [ ] Mock GetCallerIdentity endpoint
- [ ] Return configurable ARNs
- [ ] Simulate AWS errors (InvalidClientTokenId, SignatureDoesNotMatch)
- [ ] Simulate throttling
- [ ] Simulate network timeout

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Phase 3 complete  
**Estimated:** 2 hours

### Task 4.2: Integration Test with Mock STS
- [ ] Test full Verify flow with mock STS
- [ ] Test successful authentication
- [ ] Test invalid signature (AWS error)
- [ ] Test throttling (retries)
- [ ] Test timeout handling
- [ ] Verify User object construction
- [ ] Verify attributes populated correctly

**Files:** `identity/aws_sigv4_authentication_provider_test.go`  
**Dependencies:** Task 4.1  
**Estimated:** 2 hours

---

## Phase 5: E2E Testing

### Task 5.1: Create AWS Test Resources
- [ ] Create test AWS account (or use existing)
- [ ] Create IAM role: `nauts.test.admin`
- [ ] Add trust policy for EC2 or test principal
- [ ] Generate test credentials
- [ ] Document setup in `e2e/README.md`

**Files:** `e2e/aws-sigv4/`, `e2e/README.md`  
**Dependencies:** None (can be parallel with implementation)  
**Estimated:** 1 hour

### Task 5.2: Create E2E Test Setup Script
- [ ] Create `e2e/aws-sigv4/setup.sh`
- [ ] Generate nauts config with AWS provider
- [ ] Create policies and bindings for test role
- [ ] Setup NATS server config
- [ ] Generate test users file
- [ ] Similar structure to existing e2e tests

**Files:** `e2e/aws-sigv4/setup.sh`, `e2e/aws-sigv4/nauts.json`  
**Dependencies:** Task 5.1  
**Estimated:** 1.5 hours

### Task 5.3: Implement E2E Authentication Test
- [ ] Create `e2e/aws_sigv4_test.go`
- [ ] Test authentication with real AWS credentials
- [ ] Verify NATS connection established
- [ ] Verify permissions work correctly
- [ ] Test publish/subscribe with compiled permissions
- [ ] Skip test if AWS credentials not available (CI)

**Files:** `e2e/aws_sigv4_test.go`  
**Dependencies:** Task 5.2, Phase 1 complete  
**Estimated:** 2 hours

### Task 5.4: Create Client SDK Example
- [ ] Create example in `examples/aws-sigv4/`
- [ ] Show how to generate SigV4 signature (Go)
- [ ] Show how to format authentication token
- [ ] Show how to connect to NATS
- [ ] Add README with instructions

**Files:** `examples/aws-sigv4/main.go`, `examples/aws-sigv4/README.md`  
**Dependencies:** Phase 1 complete  
**Estimated:** 2 hours

---

## Phase 6: Documentation

### Task 6.1: Update Identity Spec
- [ ] Update `specs/2026-02-06-identity-authentication.md`
- [ ] Add AWS SigV4 provider to provider list
- [ ] Add configuration example
- [ ] Add token format documentation
- [ ] Link to AWS SigV4 spec

**Files:** `specs/2026-02-06-identity-authentication.md`  
**Dependencies:** Phase 1 complete  
**Estimated:** 30 minutes

### Task 6.2: Update System Overview
- [ ] Update `specs/2026-02-06-system-overview.md`
- [ ] Add AWS authentication flow diagram (optional)
- [ ] Update external dependencies list (AWS SDK)

**Files:** `specs/2026-02-06-system-overview.md`  
**Dependencies:** Phase 1 complete  
**Estimated:** 30 minutes

### Task 6.3: Create AWS IAM Setup Guide
- [ ] Create `docs/aws-iam-setup.md`
- [ ] Document how to create IAM roles
- [ ] Document naming convention: `nauts.<account>.<role>`
- [ ] Document trust policies for various AWS services (EC2, ECS, Lambda)
- [ ] Document instance profile setup
- [ ] Add troubleshooting section

**Files:** `docs/aws-iam-setup.md`  
**Dependencies:** Phase 5 complete  
**Estimated:** 2 hours

### Task 6.4: Update CLAUDE.md
- [ ] Add AWS SigV4 provider to provider list
- [ ] Document token format
- [ ] Document configuration options
- [ ] Add to implementation checklist

**Files:** `CLAUDE.md`  
**Dependencies:** Phase 1 complete  
**Estimated:** 30 minutes

### Task 6.5: Update README.md
- [ ] Add AWS SigV4 to authentication providers section
- [ ] Add configuration example
- [ ] Add quick start example
- [ ] Link to setup guide

**Files:** `README.md`  
**Dependencies:** Task 6.3  
**Estimated:** 30 minutes

---

## Phase 7: Security & Performance

### Task 7.1: Security Review
- [ ] Review timestamp validation implementation
- [ ] Verify no credential leakage in logs
- [ ] Verify error messages don't leak details
- [ ] Review AWS account restriction enforcement
- [ ] Test replay attack prevention
- [ ] Code review focused on security

**Owner:** @max-simon + Security reviewer  
**Dependencies:** Phase 1 complete  
**Estimated:** 2 hours

### Task 7.2: Performance Testing
- [ ] Measure STS call latency (p50, p95, p99)
- [ ] Test with 100 concurrent authentications
- [ ] Monitor AWS API call volume
- [ ] Test timeout behavior
- [ ] Document performance characteristics
- [ ] Add recommendations for production

**Files:** Documentation  
**Dependencies:** Phase 5 complete  
**Estimated:** 2 hours

### Task 7.3: Add Metrics/Logging
- [ ] Log successful authentications (with ARN)
- [ ] Log failed authentications (reason, no details)
- [ ] Log STS call duration
- [ ] Log timestamp validation failures (clock skew)
- [ ] Add metrics hook points (if metrics framework available)

**Files:** `identity/aws_sigv4_authentication_provider.go`  
**Dependencies:** Phase 1 complete  
**Estimated:** 1 hour

---

## Phase 8: Final Review & Release

### Task 8.1: Code Review
- [ ] Self-review implementation
- [ ] Check code follows Go conventions
- [ ] Verify all error cases handled
- [ ] Verify all TODOs resolved
- [ ] Run linters (golangci-lint)

**Dependencies:** Phases 1-7 complete  
**Estimated:** 1 hour

### Task 8.2: Documentation Review
- [ ] Review all updated docs for accuracy
- [ ] Check examples work
- [ ] Verify setup guide is complete
- [ ] Test documentation with fresh setup

**Dependencies:** Phase 6 complete  
**Estimated:** 1 hour

### Task 8.3: Update Spec to "Current"
- [ ] Change spec status from "Draft" to "Current"
- [ ] Note implementation date
- [ ] Remove "Draft" from specs README

**Files:** `specs/2026-02-08-aws-sigv4-authentication.md`, `specs/README.md`  
**Dependencies:** Implementation complete, tests passing  
**Estimated:** 15 minutes

### Task 8.4: Create Release Notes
- [ ] Document new feature
- [ ] List configuration changes
- [ ] Note any breaking changes (none expected)
- [ ] Add upgrade instructions
- [ ] List AWS SDK dependencies

**Files:** Release notes / CHANGELOG  
**Dependencies:** All phases complete  
**Estimated:** 30 minutes

---

## Summary

### Total Estimated Effort
- **Phase 0:** 2 hours (decisions)
- **Phase 1:** 18 hours (core implementation)
- **Phase 2:** 2.5 hours (config integration)
- **Phase 3:** 7.5 hours (unit tests)
- **Phase 4:** 4 hours (integration tests)
- **Phase 5:** 6.5 hours (e2e tests)
- **Phase 6:** 5 hours (documentation)
- **Phase 7:** 5 hours (security & performance)
- **Phase 8:** 2.75 hours (final review)

**Total: ~53 hours (~7 days for one developer)**

### Critical Path
1. Phase 0 (decisions) → MUST complete first
2. Phase 1 (core implementation) → Longest phase
3. Phase 3 (unit tests) → Can start as Phase 1 completes
4. Phase 5 (e2e tests) → Validates everything works
5. Phase 7 (security review) → Final gate before release

### Parallel Work Opportunities
- Phase 3 (unit tests) can start as Phase 1 tasks complete
- Phase 5.1 (AWS setup) can be done anytime
- Phase 6 (docs) can be written alongside implementation
- Phase 4 (integration tests) can be done while unit tests are written


**None** - Uses only Go standard library (encoding/json, encoding/xml, net/http, time, regexp, strings)/sts`
- `github.com/aws/aws-sdk-go-v2/config`

### Success Criteria
- [ ] All unit tests pass (>80% coverage)
- [ ] Integration tests pass with mock STS
- [ ] E2E test passes with real AWS credentials
- [ ] Security review approved
- [ ] Documentation complete and reviewed
- [ ] Performance meets SLA (p95 < 500ms)
- [ ] No breaking changes to existing providers

---

## Quick Start Checklist

For fast implementation start:

1. ✅ Review and approve Phase 0 decisions
2. ✅ Create feature branch: `feature/aws-sigv4-auth`
3. ✅ Complete Task 1.1 (package structure)
4. ✅ Complete Task 1.2 (token parsing)
5. ✅ Add first unit test (Task 3.5)
6. ✅ Iterate on implementation with TDD approach

## Notes
- Consider using test-driven development (TDD) for complex parsing logic
- Mock AWS STS for most tests to avoid API costs
- Document any deviations from spec in comments
- Add TODO comments for future enhancements (caching, etc.)
