# Specification: Auth Debug Service (`auth/`)

**Date:** 2026-02-12  
**Status:** Draft  
**Package:** `auth`  
**Dependencies:** `identity`, `provider`, `policy`, `nats.go`

---

## Goal

Provide an introspectable NATS debug endpoint (standard NATS service, no encryption) that compiles permissions for a provided user/account and returns the compilation result.

## Summary

The Auth Debug Service is a NATS service that listens on `nauts.debug` and accepts a plain JSON request containing a user object and target account. It scopes the user to the requested account and compiles policies into NATS permissions using the existing `AuthController` methods. The response returns the compilation result (pre/post-dedup permissions, warnings, roles, and policies). The service uses the `nauts.json` server configuration for connectivity. It does not issue JWTs.

---

## Scope

- NATS request/response service on `nauts.debug`
- Input parsing is plain JSON (user + account)
- Response payload contains compilation artifacts (all fields returned)
- Minimal operational logging (no sensitive data in logs by default)
- Configuration sourced from `nauts.json` (same server config block as callout)

**Out of scope:** UI, storage of debug results, rate limiting, or any management plane integration.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Reuse `AuthController` methods** | Ensures parity with production policy compilation behavior and avoids drifting logic. |
| **Dedicated debug subject (`nauts.debug`)** | Avoids interfering with `$SYS.REQ.USER.AUTH` and keeps debug tooling explicit. |
| **Structured JSON response** | Easy for tooling and humans to consume without custom decoding. |

---

## Public API

### Constants

```go
const DebugSubject = "nauts.debug"
```

### Types

#### `DebugConfig`
```go
type DebugConfig struct {
    NatsURL         string
    NatsCredentials string
    NatsNkey        string
    DefaultTTL      time.Duration
}
```

`DebugConfig` is derived from `Config.Server` in `nauts.json`. The debug service ignores `xkeySeedFile`. `DefaultTTL` is currently unused.

#### `DebugService`
```go
type DebugService struct {
    // internal fields only
}

func NewDebugService(controller *AuthController, config DebugConfig, opts ...DebugOption) (*DebugService, error)
func (s *DebugService) Start(ctx context.Context) error
func (s *DebugService) Stop() error
```

#### `DebugOption`
```go
func WithDebugLogger(l Logger) DebugOption
```

### Response Payload

The debug response is a JSON object (plain JSON) with the following shape:

```json
{
  "request": {
    "user": {
      "id": "user-id",
      "roles": [{"account": "APP", "name": "workers"}],
      "attributes": {"key": "value"}
    },
    "account": "APP"
  },
  "compilation_result": {
    "User": {"id": "user-id", "roles": [{"account": "APP", "name": "workers"}], "attributes": {"key": "value"}, "Account": "APP"},
    "Permissions": {"publish": {"allow": [], "deny": []}, "subscribe": {"allow": [], "deny": []}},
    "PermissionsRaw": {"publish": {"allow": [], "deny": []}, "subscribe": {"allow": [], "deny": []}},
    "Warnings": ["..."],
    "Roles": [{"account": "APP", "name": "default"}],
    "Policies": {"APP.default": [{"id": "...", "account": "APP", "name": "...", "statements": []}]}
  },
  "error": {"code": "...", "message": "..."}
}
```

Fields may be null when unavailable (e.g., `request`, `compilation_result`, or `error`).

---

## Protocol Flow

```
NATS Client/Tool           DebugService
    │                           │
    │  nauts.debug              │
    ├──────────────────────────►│
    │                           │
    │ 1. Decode JSON request (user + account)
    │ 2. controller.ScopeUserToAccount(ctx, user, account)
    │ 3. controller.CompileNatsPermissions(ctx, scopedUser)
    │ 4. Build debug response JSON
    │
    │  msg.Respond(response)
    │◄──────────────────────────┤
```

---

## Error Handling

- Decode errors return a JSON response with an `error` object and omit internal details.
- Compilation errors return `error` plus any partial debug fields gathered before failure.
- The service must not log tokens, JWTs, or sensitive user claims unless explicitly configured.

---

## Known Limitations / Future Work

- **No rate limiting or access control**: intended for trusted environments only.
- **Provider details shape not standardized**: implementations may vary by provider type.

---

## Ambiguities / Open Questions

None.
