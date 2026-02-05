# Identity Package Refactoring

**Date**: 2026-02-05  
**Status**: Completed  
**Related**: `sdd/backlog/26-01 identity package refactoring.md`, `sdd/specs/26-02-05-refactoring.md`

---

## Goal

Refactor the identity package to properly separate authentication from authorization concerns, support multiple authentication providers with account-based routing, and establish clearer type semantics for user roles.

---

## Changes Implemented

### 1. Type System Refactoring

**Renamed Types** (Authentication Provider Naming):
- `UserIdentityProvider` → `AuthenticationProvider`
- `FileUserIdentityProvider` → `FileAuthenticationProvider`  
- `JwtUserIdentityProvider` → `JwtAuthenticationProvider`

**New Type**: `AccountRole`
- Combines account and role name: `{Account: "APP", Role: "admin"}`
- String representation: `"APP.admin"`
- Helper functions: `ParseAccountRole()`, `ParseAccountRoles()`

**Updated Type**: `User`
- Changed `Roles` field from `[]string` → `[]AccountRole`
- Removed `Account` field (account passed separately through flow)
- Preserves role-account associations throughout auth flow

### 2. Authentication Provider Interface

**Added Methods**:
- `ID() string` - Unique provider identifier for explicit selection
- `CanManageAccount(account string) bool` - Wildcard pattern matching for provider routing

**Added Utility**:
- `MatchAccountPattern(pattern, account string) bool` - Wildcard matching (`*` and `?` support)

**Verify Method Behavior**:
- Returns `User` with **all** roles as `[]AccountRole` (no filtering)
- Account information embedded in each `AccountRole`
- No account resolution logic in providers

### 3. File Authentication Provider

**Configuration Changes**:
- Added `id` field (required for multiple providers)
- Added `accounts` field (array of account patterns: `["APP", "PROD*"]`)
- Renamed config key: `identity.file` → `authentication.file` (array)

**User Data Format**:
- Roles use `"account.role"` format: `["APP.admin", "APP.viewer"]`
- Removed separate `accounts` and `roles` fields
- Parser handles format validation during authentication

### 4. JWT Authentication Provider

**Configuration Changes**:
- Changed from multi-issuer map to single-issuer per provider
- Each provider instance handles one issuer
- Added `id` and `accounts` fields like file provider
- Renamed config key: `identity.jwt` → `authentication.jwt` (array)

**Removed Functions**:
- `determineTargetAccount()` - Account now from AuthRequest
- `filterAndStripRoles()` - Filtering moved to AuthController
- `issuerCanManageAccount()` - Replaced by interface method

**Behavior**:
- Returns all user roles from JWT claims
- No filtering based on account patterns
- Roles parsed as `[]AccountRole` from JWT

### 5. Authentication Controller

**Multiple Provider Support**:
- Changed from single provider → array of providers
- Added `selectAuthenticationProvider()` for provider routing
- Selection based on account patterns and optional explicit provider ID

**Role Filtering**:
- Added `filterRolesForAccount()` method
- Filters `[]AccountRole` → `[]AccountRole` for target account
- Applied in `ResolveUser()` after authentication

**Updated Methods**:
- `ResolveUser()`: Select provider → authenticate → filter roles → return User
- `ResolveNatsPermissions()`: Accept account parameter, iterate through filtered `[]AccountRole`

**Auth Request Format**:
- Required: `{"account": "APP", "token": "..."}`
- Optional: `{"account": "APP", "token": "...", "ap": "provider-id"}`
- Account field used for provider selection and role filtering

### 6. Configuration Structure

**Before** (Single Provider):
```json
{
  "identity": {
    "type": "file",
    "file": {...}
  }
}
```

**After** (Multiple Providers):
```json
{
  "authentication": {
    "file": [
      {"id": "local", "accounts": ["APP"], ...}
    ],
    "jwt": [
      {"id": "okta", "accounts": ["PROD*"], ...}
    ]
  }
}
```

### 7. Documentation Updates

**Updated Files**:
- `README.md`: Configuration examples, authentication provider descriptions
- `IMPLEMENTATION.md`: Package structure, authentication flow, provider selection
- `CLAUDE.md`: Type references, configuration format, recent changes section

---

## Outcomes

### Achieved
✅ Clear separation: authentication (credential verification) vs authorization (permission resolution)  
✅ Multiple authentication providers with account-based routing  
✅ Explicit role-account associations via `AccountRole` type  
✅ Provider selection using wildcard patterns and explicit IDs  
✅ Role filtering centralized in authorization layer (AuthController)  
✅ Code compiles successfully (`go build ./...`)  
✅ Documentation synchronized with implementation

### Pending
⏳ Test file updates (compilation errors due to old type names)  
⏳ Test data updates (user files need `account.role` format)  
⏳ Configuration file updates in test directories  
⏳ Full test suite validation

---

## Key Decisions

1. **No separate AuthenticatedUser type**: Single `User` type serves both pre-filter (from provider) and post-filter (from controller) contexts

2. **AccountRole never contains wildcards**: Only concrete account values (`"APP"`, `"PROD"`); wildcards only in Role definitions from RoleProvider

3. **Role filtering in AuthController**: Authentication providers return all roles; controller filters based on target account

4. **Array-based provider configuration**: Supports multiple providers of same type (multiple file providers, multiple JWT issuers)

5. **Provider selection via account patterns**: Automatic provider routing using `CanManageAccount()` with wildcard support

6. **Explicit provider selection**: Optional `ap` field in auth request for disambiguation when multiple providers match

7. **Backward compatibility not maintained**: Configuration format changed (breaking change requiring migration)

---

## Migration Guide

**Configuration Files**:
- Rename `identity` → `authentication`
- Convert to array format: `{"file": [{...}], "jwt": [{...}]}`
- Add `id` and `accounts` fields to each provider

**User Data Files**:
- Convert roles from `["admin"]` → `["APP.admin"]`
- Remove separate `accounts` array

**Authentication Requests**:
- Add required `account` field: `{"account": "APP", "token": "..."}`
- Optionally specify provider: `{"account": "APP", "token": "...", "ap": "provider-id"}`

**Test Code**:
- Update type names: `FileUserIdentityProvider` → `FileAuthenticationProvider`
- Handle `[]AccountRole` instead of `[]string` for roles
- Adjust test data to new formats
