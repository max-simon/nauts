# Specification: Auth Debug Service (`auth/`)

**Date:** 2026-02-12  
**Status:** Draft  
**Package:** `auth`  
**Dependencies:** `identity`, `provider`, `policy`, `jwt`, `nats.go`, `jwt/v2`

---

## Goal

Provide an introspectable NATS debug endpoint (standard NATS service, no encryption) that replays the auth callout flow and returns a detailed, developer-friendly breakdown of the authentication pipeline.

## Summary

The Auth Debug Service is a NATS service that listens on `nauts.debug` and accepts the same request payload as the auth callout service (`$SYS.REQ.USER.AUTH`). It decodes and validates the request, runs the existing `AuthController` flow, and returns a structured debug response including decoded auth request data, the selected `AuthenticationProvider`, the resolved user, compiled permissions with warnings from `policy.Compile`, deduplicated permissions, and the resulting signed user JWT. The service uses the `nauts.json` server configuration and always applies the default TTL. The service reuses existing auth package methods to keep behavior aligned with the callout service.

---

## Scope

- NATS request/response service on `nauts.debug`
- Input parsing identical to callout service (`AuthorizationRequestClaims`)
- Response payload containing diagnostic artifacts from the auth flow (all fields returned)
- Minimal operational logging (no sensitive data in logs by default)
- Configuration sourced from `nauts.json` (same server config block as callout)

**Out of scope:** UI, storage of debug results, rate limiting, or any management plane integration.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Reuse `AuthController` methods** | Ensures parity with production auth behavior and avoids drifting logic. |
| **Same request format as callout** | Allows replaying real auth requests without translation. |
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

`DebugConfig` is derived from `Config.Server` in `nauts.json`. The debug service ignores `xkeySeedFile` and always uses `DefaultTTL` for JWT issuance (no per-request override).

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
    "user_nkey": "U...",
    "server": {
      "id": "...",
      "name": "...",
      "host": "...",
      "version": "..."
    },
    "connect_options": {
      "token": "...",
      "user": "...",
      "name": "...",
      "account": "..."
    }
  },
  "auth_provider": {
    "id": "provider-id",
    "type": "jwt|file|aws-sigv4|...",
    "manageable_accounts": ["APP", "tenant-*", "*"]
  },
  "resolved_user": {
    "id": "user-id",
    "name": "...",
    "account": "A...",
    "roles": ["role1", "role2"],
    "claims": {"key": "value"}
  },
  "permissions": {
    "compiled": {"publish": {"allow": [], "deny": []}, "subscribe": {"allow": [], "deny": []}},
    "warnings": ["..."],
    "deduplicated": {"publish": {"allow": [], "deny": []}, "subscribe": {"allow": [], "deny": []}}
  },
  "user_jwt": "...",
  "error": {
    "code": "...",
    "message": "..."
  }
}
```

All fields are returned; when unavailable, values are empty objects/arrays or empty strings. The `error` object is always present (empty `code`/`message` on success).

---

## Protocol Flow

```
NATS Client/Tool           DebugService
    │                           │
    │  nauts.debug              │
    │  nauts.debug              │
    ├──────────────────────────►│
    │                           │
    │ 1. Decode AuthorizationRequestClaims
    │ 2. Extract ConnectOptions.Token + user public key
    │ 3. controller.ResolveUser(ctx, token)
    │ 4. Compile permissions with policy.Compile (capture CompileResult.Warnings)
    │ 5. Capture compiled (pre-dedup) permissions snapshot
    │ 6. permissions.Deduplicate()
    │ 7. Capture deduplicated permissions snapshot
    │ 8. controller.CreateUserJWT(ctx, user, pubKey, perms, DefaultTTL)
    │ 9. Build debug response JSON
    │
    │  msg.Respond(response)
    │◄──────────────────────────┤
```

---

## Error Handling

- Decode errors return a JSON response with an `error` object and omit internal details.
- Auth pipeline errors return `error` plus any partial debug fields gathered before failure.
- The service must not log tokens, JWTs, or sensitive user claims unless explicitly configured.

---

## Known Limitations / Future Work

- **No rate limiting or access control**: intended for trusted environments only.
- **Provider details shape not standardized**: implementations may vary by provider type.

---

## Ambiguities / Open Questions

None.
