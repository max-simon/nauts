# Specification: Auth Controller & Callout Service (`auth/`)

**Date:** 2026-02-06  
**Status:** Current  
**Package:** `auth`  
**Dependencies:** `identity`, `provider`, `policy`, `jwt`

---

## Goal

Orchestrate the full authentication lifecycle — from raw NATS connect token to a signed user JWT with compiled permissions — and expose this as both a programmatic API and a NATS auth callout service.

## Summary

The `auth` package is the top-level coordinator of nauts. `AuthController` wires together identity verification, permission compilation, and JWT issuance into a single `Authenticate` method. `CalloutService` subscribes to the NATS auth callout subject (`$SYS.REQ.USER.AUTH`) and translates protocol messages into `AuthController` calls. A `Config` system loads a JSON configuration file and bootstraps all providers.

---

## Scope

- `AuthController` — authentication orchestration  
- `CalloutService` — NATS auth callout protocol handler  
- `Config` / `LoadConfig` / `NewAuthControllerWithConfig` — configuration and wiring  
- `AuthError` — structured error type  
- `Logger` interface  

**Out of scope:** Individual provider implementations (see `provider/`, `identity/` specs), policy compilation details (see `policy/` spec).

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Controller pattern** | A single orchestrator avoids scattering the auth flow across packages. Each step (resolve user → compile perms → issue JWT) is a separate public method for testability. |
| **`default` role always included** | Every user gets the `default` role's policies (if bound). This provides a safe baseline without explicit assignment. |
| **Role filtering in controller, not provider** | Identity providers return all roles. The controller filters by requested account. This keeps providers simple and moves authorization logic to a single location. |
| **Ephemeral user keys** | When no user public key is provided (auth callout scenario), the controller generates an ephemeral nkeys user key. This allows NATS to establish the connection without pre-provisioned user keys. |
| **Generic error responses** | The callout service never leaks internal error details to clients. All auth failures return `"authentication failed"`. Only JWT-creation errors return `"internal error"`. Full errors are logged server-side. |
| **Encrypted auth callout via XKey** | If both the service and NATS server are configured with curve keys (xkey), requests and responses are encrypted. This is optional — the service works without encryption. |
| **Config-driven bootstrapping** | `NewAuthControllerWithConfig` creates all providers from a single JSON config. This simplifies CLI usage and operational deployment. |

---

## Public API

### Constants

```go
const DefaultRoleName = "default"   // Implicit role for all users
```

### Types

#### `AuthController`
```go
func NewAuthController(
    accountProvider provider.AccountProvider,
    policyProvider provider.PolicyProvider,
    authProviders *identity.AuthenticationProviderManager,
    opts ...ControllerOption,
) *AuthController

func NewAuthControllerWithConfig(config *Config, opts ...ControllerOption) (*AuthController, error)
```

| Method | Signature | Purpose |
|--------|-----------|---------|
| `Authenticate` | `(ctx, connectOptions, userPublicKey, ttl) → (*AuthResult, error)` | Full flow: verify → compile → sign |
| `ResolveUser` | `(ctx, token string) → (*AccountScopedUser, error)` | Parse JSON token, verify identity, filter roles by account, attach account scope |
| `ResolveNatsPermissions` | `(ctx, user) → (*policy.NatsPermissions, error)` | Compile permissions for all user roles (scoped account provided by user) |
| `CreateUserJWT` | `(ctx, user, pubKey, perms, ttl) → (string, error)` | Sign a NATS user JWT (scoped account provided by user) |
| `AccountProvider` | `() → provider.AccountProvider` | Accessor for the account provider |

#### `AccountScopedUser`
```go
type AccountScopedUser struct {
    identity.User
    Account string
}
```
Returned by `ResolveUser`. Roles are filtered to only those matching `Account`.

#### `AuthResult`
```go
type AuthResult struct {
    User          *AccountScopedUser
    UserPublicKey string
    Permissions   *policy.NatsPermissions
    JWT           string
}
```

#### `ControllerOption`
```go
func WithLogger(l Logger) ControllerOption
```

### Authentication Flow (`Authenticate`)

```
Token (JSON string)
  │
  ├─► parseAuthRequest(token) → AuthRequest{account, token, ap}
  │     - Validate: account required, no wildcards in account
  │
  ├─► authProviders.Verify(ctx, authReq) → *User (all roles)
  │     - Routes to correct provider (by ap or account pattern)
  │
    ├─► Filter user.Roles to requested account only
  │     - Validate: no wildcards in role names
    │
    ├─► Return AccountScopedUser{Account: authReq.Account, User: user}
  │
  ├─► ResolveNatsPermissions(ctx, user)
  │     ├─► collectRoleNames(user) → ["default", role1, role2, ...]
  │     ├─► for each role:
  │     │     ├─► policyProvider.GetPoliciesForRole(account, role)
  │     │     └─► policy.Compile(policies, userCtx, roleCtx, perms)
  │     └─► perms.Deduplicate()
  │
  ├─► Generate ephemeral user key (if not provided)
  │
  └─► CreateUserJWT(ctx, user, pubKey, perms, ttl)
        ├─► accountProvider.GetAccount(account)
        ├─► Determine audience (static) vs issuerAccount (operator)
        └─► jwt.IssueUserJWT(...)
```

### Callout Service

#### `CalloutService`
```go
type CalloutConfig struct {
    NatsURL         string
    NatsCredentials string        // mutually exclusive with NatsNkey
    NatsNkey        string        // mutually exclusive with NatsCredentials
    XKeySeed        string        // optional, for encrypted callout
    DefaultTTL      time.Duration
}

func NewCalloutService(controller *AuthController, config CalloutConfig, opts ...CalloutOption) (*CalloutService, error)
func (s *CalloutService) Start(ctx context.Context) error   // blocks until stopped
func (s *CalloutService) Stop() error                        // signal graceful shutdown
```

#### Protocol Flow

```
NATS Server                    CalloutService
    │                               │
    │  $SYS.REQ.USER.AUTH           │
    │  [Header: Nats-Server-Xkey]   │
    ├──────────────────────────────►│
    │                               │
    │                 1. Decrypt (if xkey configured)
    │                 2. Decode AuthorizationRequestClaims
    │                 3. Extract ConnectOptions.Token
    │                 4. controller.Authenticate(ctx, opts, nkey, ttl)
    │                 5. Build AuthorizationResponseClaims
    │                 6. Set IssuerAccount (operator mode only)
    │                 7. Sign response with AUTH account signer
    │                 8. Encrypt (if xkey configured)
    │                               │
    │  msg.Respond(response)        │
    │◄──────────────────────────────┤
```

**Error handling:**
- Decryption failure → `"authentication failed"`
- Decode failure → `"authentication failed"`
- Missing token → `"authentication failed"`
- Auth failure → `"authentication failed"` (detailed error logged)
- JWT creation failure → `"internal error"` (detailed error logged)

**Graceful shutdown:**
1. `Stop()` closes the done channel
2. `Start()` drains the subscription (no new requests)
3. `sync.WaitGroup` waits for in-flight requests
4. NATS connection closed

#### `CalloutOption`
```go
func WithCalloutLogger(l Logger) CalloutOption
```

### Configuration

#### `Config`
```go
type Config struct {
    Account AccountConfig   `json:"account"`
    Policy  PolicyConfig    `json:"policy"`
    Auth    AuthConfig      `json:"auth"`
    Server  ServerConfig    `json:"server"`
}
func LoadConfig(path string) (*Config, error)
func (c *Config) Validate() error
```

#### Sub-configs

| Config | Key fields |
|--------|------------|
| `AccountConfig` | `type` (`"operator"` / `"static"`), `operator` / `static` sub-config |
| `PolicyConfig` | `type` (`"file"`), `file` sub-config with `policiesPath`, `bindingsPath` |
| `AuthConfig` | `file` (list of file auth providers), `jwt` (list of JWT auth providers) |
| `ServerConfig` | `natsUrl`, `natsCredentials` / `natsNkey`, `xkeySeedFile`, `ttl` |

#### Validation Rules

- Account type must be `"operator"` or `"static"` (defaults to `"static"`)
- Policy type must be `"file"` (defaults to `"file"`)
- At least one auth provider required
- All provider IDs must be unique
- Required fields per provider type enforced

### Error Types

#### `AuthError`
```go
type AuthError struct {
    UserID  string
    Phase   string   // "resolve_user", "resolve_permissions", "create_jwt", "authenticate"
    Message string
    Err     error
}
func NewAuthError(userID, phase, message string, err error) *AuthError
```
Wraps errors with user context and lifecycle phase.

### Logger Interface
```go
type Logger interface {
    Info(msg string, args ...any)
    Warn(msg string, args ...any)
    Debug(msg string, args ...any)
}
```
A `defaultLogger` using `log.Printf` is provided. Override with `WithLogger` / `WithCalloutLogger`.

---

## Operator vs Static Mode Summary

| Aspect | Operator Mode | Static Mode |
|--------|--------------|-------------|
| Signing keys | Per-account | Shared |
| `IsOperatorMode()` | `true` | `false` |
| JWT `Audience` | Not set | Account name |
| JWT `IssuerAccount` | Signing key pubkey | Not set |
| Auth callout `IssuerAccount` | Signing key pubkey | Not set |
| Client auth | Sentinel creds + token | Token only |

---

## Known Limitations / Future Work

- **No rate limiting:** Auth callout has no per-client or global rate limits.
- **No caching:** Every request re-compiles permissions from scratch. A permission cache keyed by (user, roles) would improve throughput.
- **No hot-reload:** Configuration changes require service restart.
- **No health checks:** No liveness/readiness endpoints or NATS-based health reporting.
- **Singleton AUTH account:** The callout response is always signed by the `AUTH` account's signer. Multi-auth-account deployments are not supported.
