# Specification: Identity & Authentication (`identity/`)

**Date:** 2026-02-06  
**Status:** Current  
**Package:** `identity`  
**Dependencies:** None (standalone — uses only stdlib + `golang.org/x/crypto`, `golang-jwt/jwt/v5`)

---

## Goal

Define a pluggable authentication layer that verifies user credentials and returns a normalized user identity. The package is provider-agnostic — any backend that can verify a token and produce a `User` can participate.

## Summary

The `identity` package defines the `AuthenticationProvider` interface, the `User` and `AuthRequest` types, and ships two concrete providers: `FileAuthenticationProvider` (bcrypt passwords from a JSON file) and `JwtAuthenticationProvider` (external JWT verification for IdPs like Keycloak/Auth0). An `AuthenticationProviderManager` routes requests to the correct provider based on account patterns or explicit provider selection.

---

## Scope

- `User` type and `Role` type  
- `AuthRequest` — the parsed authentication token  
- `AuthenticationProvider` interface  
- `FileAuthenticationProvider` — file-based username/password  
- `JwtAuthenticationProvider` — external JWT verification  
- `AuthenticationProviderManager` — provider routing  

**Out of scope:** Authorization (role filtering), JWT issuance, NATS permissions.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Providers return all roles, controller filters** | Authentication (who are you?) is separated from authorization (what can you do?). Providers return the full role set; the `AuthController` filters by requested account. |
| **`AuthRequest` is JSON-based** | NATS auth callout passes the token as a string. A JSON envelope `{"account","token","ap"}` keeps the protocol extensible. |
| **`ap` field for explicit provider selection** | When multiple providers are configured, `ap` lets the client target a specific one. Without `ap`, the manager auto-selects by account pattern matching. |
| **Account pattern matching with `*` and `prefix*`** | Supports multi-tenant deployments where a single IdP manages `tenant-*` accounts. Special accounts `SYS` and `AUTH` require explicit listing (never matched by `*`). |
| **Roles use `<account>.<role>` format** | Binds a role to its account context in a single string. Both file users and JWT claims use this format. |
| **`AuthenticationProviderManager` enforces single-match** | If zero or multiple providers match a request (and no `ap` is given), it returns an error. This prevents silent misrouting. |

---

## Public API

### Types

#### `User`
```go
type User struct {
    ID         string            `json:"id,omitempty"`
  Roles      []Role            `json:"roles"`
    Attributes map[string]string `json:"attributes,omitempty"`
}
```
Normalized user identity. `Roles` are account-scoped. `Attributes` carry provider-specific data (e.g., email, department) used in variable interpolation.

#### `Role`
```go
type Role struct {
    Account string `json:"account"`
  Name    string `json:"role"`
}
```
A role scoped to a NATS account.

#### `AuthRequest`
```go
type AuthRequest struct {
    Account string `json:"account"`        // required
    Token   string `json:"token"`          // provider-specific credential
    AP      string `json:"ap,omitempty"`   // optional provider id
}
```
Parsed from the NATS connect token JSON.

### Interfaces

#### `AuthenticationProvider`
```go
type AuthenticationProvider interface {
    Verify(ctx context.Context, req AuthRequest) (*User, error)
    ManageableAccounts() []string
}
```
Core contract. `Verify` authenticates and returns a `User`. `ManageableAccounts` returns account patterns (e.g., `["*"]`, `["tenant-*", "shared"]`).

### Concrete Providers

#### `FileAuthenticationProvider`
```go
type FileAuthenticationProviderConfig struct {
    UsersPath string
    Accounts  []string
}
func NewFileAuthenticationProvider(cfg FileAuthenticationProviderConfig) (*FileAuthenticationProvider, error)
func (fp *FileAuthenticationProvider) Verify(ctx context.Context, req AuthRequest) (*User, error)
func (fp *FileAuthenticationProvider) ManageableAccounts() []string
```

**Token format:** `"username:password"` (colon-separated inside `AuthRequest.Token`).

**Users file format:**
```json
{
  "users": {
    "<username>": {
      "accounts": ["APP"],
      "roles": ["APP.readonly", "APP.admin"],
      "passwordHash": "$2a$10$...",
      "attributes": { "department": "eng" }
    }
  }
}
```

**Verify flow:**
1. Parse `token` as `username:password`
2. Look up user by username → `ErrUserNotFound`
3. bcrypt verify password → `ErrInvalidCredentials`
4. Check requested account is in user's `accounts` list → `ErrInvalidAccount`
5. Parse role strings into `[]Role`, skip invalid formats
6. Return `User` with all roles (not filtered by account)

#### `JwtAuthenticationProvider`
```go
type JwtAuthenticationProviderConfig struct {
    Accounts       []string
    Issuer         string
    PublicKey       string   // base64-encoded PEM
    RolesClaimPath string   // default: "resource_access.nauts.roles"
}
func NewJwtAuthenticationProvider(cfg JwtAuthenticationProviderConfig) (*JwtAuthenticationProvider, error)
func (p *JwtAuthenticationProvider) Verify(ctx context.Context, req AuthRequest) (*User, error)
func (p *JwtAuthenticationProvider) ManageableAccounts() []string
```

**Token format:** Raw JWT string from external IdP inside `AuthRequest.Token`.

**Verify flow:**
1. Parse and verify JWT signature (RSA or ECDSA) → `ErrInvalidCredentials`
2. Validate `iss` claim matches configured issuer → `ErrInvalidCredentials`
3. Extract roles from claim at `rolesClaimPath` (e.g., `resource_access.nauts.roles`)
4. Parse roles as `<account>.<role>` → skip invalid formats
5. Return `ErrNoRolesFound` if no valid roles
6. Extract standard claims as attributes (currently only `sub` → `attributes["sub"]`)
7. Return `User`

### Manager

#### `AuthenticationProviderManager`
```go
func NewAuthenticationProviderManager(providers map[string]AuthenticationProvider) (*AuthenticationProviderManager, error)
func (m *AuthenticationProviderManager) Verify(ctx context.Context, req AuthRequest) (*User, error)
```

**Routing logic:**
1. If `req.AP` is set: look up provider by id → `ErrAuthenticationProviderNotFound` if missing; verify account is manageable → `ErrAuthenticationProviderNotManageable`
2. If `req.AP` is empty: collect all providers whose `ManageableAccounts()` match `req.Account`
   - 0 matches → `ErrAuthenticationProviderNotManageable`
   - 1 match → use it
   - 2+ matches → `ErrAuthenticationProviderAmbiguous`

**Account pattern matching:**
- `"*"` matches any account **except** `SYS` and `AUTH`
- `"prefix*"` matches accounts starting with `prefix`
- Exact match always works
- `SYS` and `AUTH` must be listed explicitly

### Utility

```go
func ParseRoleID(roleID string) (Role, error)
```
Parses `"APP.admin"` → `Role{Account: "APP", Name: "admin"}`. Returns error if format is invalid.

### Sentinel Errors

| Error | Meaning |
|-------|---------|
| `ErrInvalidCredentials` | Password/signature verification failed |
| `ErrUserNotFound` | User does not exist (file provider) |
| `ErrInvalidTokenType` | Token format wrong for provider |
| `ErrInvalidAccount` | Account not valid for this user |
| `ErrNoRolesFound` | No valid roles in JWT claims |
| `ErrAuthenticationProviderNotFound` | Explicit `ap` id not registered |
| `ErrAuthenticationProviderAmbiguous` | Multiple providers match, `ap` not set |
| `ErrAuthenticationProviderNotManageable` | Account not in provider's patterns |

---

## Known Limitations / Future Work

- **No token refresh/revocation:** File provider has no session concept; JWT provider relies on token expiry.
- **No OIDC discovery:** JWT provider requires manual public key configuration; JWKS/OIDC auto-discovery is not implemented.
- **Roles are string-parsed:** No formal role registry; invalid role format strings are silently skipped.
- **Single public key per JWT provider:** Key rotation requires reconfiguration.
