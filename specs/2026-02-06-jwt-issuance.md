# Specification: JWT Issuance (`jwt/`)

**Date:** 2026-02-06  
**Status:** Current  
**Package:** `jwt`  
**Dependencies:** `policy` (for `NatsPermissions`)

---

## Goal

Provide a thin abstraction for signing and issuing NATS user JWTs. The package isolates cryptographic operations behind an interface so the rest of the system never touches raw keys directly.

## Summary

The `jwt` package defines a `Signer` interface for cryptographic signing, a `LocalSigner` implementation backed by NATS nkeys, and an `IssueUserJWT` function that creates a NATS user JWT embedding compiled permissions. A `SignerAdapter` bridges the `Signer` interface to the `nats-io/jwt/v2` library's `nkeys.KeyPair` expectation.

---

## Scope

- `Signer` interface for key abstraction  
- `LocalSigner` — nkey-based signing  
- `IssueUserJWT` — NATS user JWT creation with permissions  
- `SignerAdapter` — bridge to `nats-io/jwt/v2`  

**Out of scope:** Key generation, key storage/rotation, JWT verification, non-user JWT types.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **`Signer` interface** | Decouples signing from key storage. Future implementations may wrap HSMs, Vault, or remote signing services without changing callers. |
| **Explicit deny on empty permissions** | NATS JWTs default to *allow everything* when permissions are unset. `IssueUserJWT` sets `Deny: [">"]` when no allow permissions are granted, enforcing least privilege. |
| **`audienceAccount` vs `issuerAccount`** | In static mode the JWT's `Audience` identifies the target account. In operator mode `IssuerAccount` identifies the signing key's account. Both are passed explicitly to keep the function mode-agnostic. |
| **`SignerAdapter` wraps `Signer`** | The `nats-io/jwt/v2` encode API expects a `nkeys.KeyPair`. The adapter satisfies this by delegating `Sign` and `PublicKey` while returning errors for unsupported operations (`Seed`, `Verify`, `Open`, `Seal`). |

---

## Public API

### Interfaces

#### `Signer`
```go
type Signer interface {
    PublicKey() string
    Sign(data []byte) ([]byte, error)
}
```
Minimal interface for cryptographic signing. Implementations must return an nkey public key string.

### Types

#### `LocalSigner`
```go
type LocalSigner struct { /* nkeys.KeyPair */ }
func NewLocalSigner(seed string) (*LocalSigner, error)
func (s *LocalSigner) PublicKey() string
func (s *LocalSigner) Sign(data []byte) ([]byte, error)
```
Signs using a local nkey seed (`SOABC...`). Implements `Signer`.

#### `SignerAdapter`
```go
type SignerAdapter struct { /* wraps Signer */ }
func NewSignerAdapter(s Signer) SignerAdapter
```
Adapts `Signer` to `nkeys.KeyPair` for use with `natsjwt.Claims.Encode()`.

### Functions

#### `IssueUserJWT`
```go
func IssueUserJWT(
    userName string,
    userPublicKey string,
    ttl time.Duration,
    permissions *policy.NatsPermissions,
    issuerSigner Signer,
    audienceAccount string,   // non-empty in static mode
    issuerAccount string,     // non-empty in operator mode
) (string, error)
```

Creates and signs a NATS user JWT.

**Permission encoding rules:**
| Condition | Pub | Sub |
|-----------|-----|-----|
| Allow list non-empty | `Allow: [...]` | `Allow: [...]` |
| Allow list empty | `Deny: [">"]` | `Deny: [">"]` |

**Audience / IssuerAccount rules:**
| Mode | `audienceAccount` | `issuerAccount` |
|------|-------------------|-----------------|
| Static | account name | `""` |
| Operator | `""` | signing key's public key |

---

## Known Limitations / Future Work

- **No sub-with-queue in JWT:** Queue permissions from `NatsPermissions.SubWithQueueList()` are not yet encoded into the JWT claims (NATS JWT v2 supports this).
- **No resource limits:** `NatsLimits` (max subscriptions, payload, etc.) are not set.
- **No JWT refresh/revocation:** JWTs are fire-and-forget; revocation requires NATS account-level revocation lists.
