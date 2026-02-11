# Specification: System Overview & Architecture

**Date:** 2026-02-06  
**Status:** Current  
**Scope:** Full system  
**Packages:** `policy`, `jwt`, `identity`, `provider`, `auth`, `cmd/nauts`

---

## Goal

Provide a single reference document that describes how all nauts components fit together — the dependency graph, the end-to-end authentication flow, deployment modes, and the data model relationships. This is the entry point for understanding the system before diving into individual component specs.

## Summary

**nauts** (NATS Authentication Service) is a policy-based access-control framework for [NATS](https://nats.io). It bridges external identity providers with the NATS permission model by compiling human-friendly policies into low-level NATS publish/subscribe permissions, embedding them in signed JWTs, and serving them via the NATS auth callout protocol.

### What it does (one paragraph)

A NATS client connects with a JSON token containing an account name and credentials. The NATS server forwards this to nauts via auth callout. Nauts verifies the identity (file-based passwords or external JWT), looks up the user's roles, resolves the policies bound to those roles, compiles the policies into NATS subjects, signs a user JWT, and returns it. The NATS server uses this JWT to authorize the connection with the exact permissions specified.

---

## Component Dependency Graph

```
┌──────────────────────────────────────────────────────────────┐
│                       cmd/nauts/                             │
│  CLI: auth (one-shot) · serve (callout service)             │
│  ───────────────────────────────────────────────────────────  │
│  Depends on: auth                                            │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────┐
│                         auth/                                │
│  AuthController · CalloutService · Config                    │
│  ─────────────────────────────────────────────────────────── │
│  Depends on: identity, provider, policy, jwt                 │
└────────┬──────────┬───────────────┬──────────────┬───────────┘
         │          │               │              │
         ▼          ▼               ▼              ▼
    ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌────────────┐
    │identity/│ │provider/ │ │ policy/  │ │   jwt/     │
    │         │ │          │ │          │ │            │
    │ User    │ │ Account  │ │ Compile  │ │ Signer     │
    │ AuthProv│ │ Policy   │ │ NRN      │ │ IssueJWT   │
    │ Manager │ │ Provider │ │ Actions  │ │ LocalSigner│
    └─────────┘ └─────┬────┘ └──────────┘ └────────────┘
                      │           ▲              ▲
                      │           │              │
                      └───────────┴──────────────┘
                      provider depends on policy (types)
                      provider depends on jwt (Signer)
```

### Dependency matrix

| Package | `policy` | `jwt` | `identity` | `provider` | `auth` | `cmd/nauts` |
|---------|:--------:|:-----:|:----------:|:----------:|:------:|:-----------:|
| `policy` | — | — | — | — | — | — |
| `jwt` | ✓ | — | — | — | — | — |
| `identity` | — | — | — | — | — | — |
| `provider` | ✓ | ✓ | — | — | — | — |
| `auth` | ✓ | ✓ | ✓ | ✓ | — | — |
| `cmd/nauts` | — | — | — | — | ✓ | — |

`policy` and `identity` are standalone. `jwt` depends only on `policy` for the `NatsPermissions` type. `provider` depends on `policy` (types) and `jwt` (Signer). `auth` is the orchestrator. `cmd/nauts` is the CLI wrapper around `auth`.

---

## Specification Index

These specs are ordered from standalone → dependent. Read them in order for a bottom-up understanding.

| # | Spec File | Package | Dependencies |
|---|-----------|---------|-------------|
| 1 | [policy-engine](2026-02-06-policy-engine.md) | `policy/` | None |
| 2 | [jwt-issuance](2026-02-06-jwt-issuance.md) | `jwt/` | `policy` |
| 3 | [identity-authentication](2026-02-06-identity-authentication.md) | `identity/` | None |
| 4 | [providers](2026-02-06-providers.md) | `provider/` | `policy`, `jwt` |
| 5 | [auth-controller-callout](2026-02-06-auth-controller-callout.md) | `auth/` | All above |
| 6 | [cli](2026-02-08-cli.md) | `cmd/nauts/` | `auth` |
| 7 | This document | System | All |

---

## End-to-End Authentication Flow

```
Client                 NATS Server              nauts (CalloutService)
  │                         │                           │
  │  CONNECT {token: JSON}  │                           │
  ├────────────────────────►│                           │
  │                         │  $SYS.REQ.USER.AUTH       │
  │                         ├──────────────────────────►│
  │                         │                           │
  │                         │            ┌──────────────┤
  │                         │            │ 1. Decrypt   │
  │                         │            │ 2. Parse     │
  │                         │            │    token     │
  │                         │            │              │
  │                         │            │ 3. Verify    │
  │                         │            │    identity  │
  │                         │            │   (identity/)│
  │                         │            │              │
  │                         │            │ 4. Resolve   │
  │                         │            │    roles     │
  │                         │            │              │
  │                         │            │ 5. Get       │
  │                         │            │    policies  │
  │                         │            │  (provider/) │
  │                         │            │              │
  │                         │            │ 6. Compile   │
  │                         │            │    perms     │
  │                         │            │   (policy/)  │
  │                         │            │              │
  │                         │            │ 7. Sign JWT  │
  │                         │            │    (jwt/)    │
  │                         │            └──────────────┤
  │                         │                           │
  │                         │  Response (signed JWT)    │
  │                         │◄──────────────────────────┤
  │                         │                           │
  │  Connection established │                           │
  │  (with NATS permissions)│                           │
  │◄────────────────────────┤                           │
```

---

## Data Model Relationships

```
User ──has──► []Role ──references──► Binding
                                              │
                                              ├─ role: string
                                              ├─ account: string
                                              └─ policies: []string ──references──► Policy
                                                                                      │
                                                                                      ├─ id: string
                                                                                      └─ statements: []Statement
                                                                                            │
                                                                                            ├─ effect: "allow"
                                                                                            ├─ actions: []Action
                                                                                            └─ resources: []NRN string
```

**Key relationships:**
- A `User` has multiple `Role`s (scoped to accounts)
- A `Role(account, name)` maps to a `Binding`
- A `Binding` references multiple `Policy` IDs
- A `Policy` contains `Statements` with `Actions` and `Resources`
- `Resources` are NRN strings compiled into NATS permissions

---

## Deployment Modes

### Static Mode
- **Use case:** Development, single-account, simple setups
- **Key setup:** One nkey signing key shared by all accounts
- **Client connects with:** `--token '{"account":"APP","token":"alice:secret"}'`
- **JWT identifies account via:** `Audience` field

### Operator Mode
- **Use case:** Production, multi-account, NATS operator hierarchy
- **Key setup:** Per-account signing keys derived from operator
- **Client connects with:** `--creds sentinel.creds --token '{"account":"APP","token":"alice:secret"}'`
- **JWT identifies account via:** `IssuerAccount` field (signing key's public key)
- **Requires:** Sentinel user in AUTH account, operator JWTs

---

## Token Format

All client authentication uses a JSON token:

```json
{
  "account": "APP",              // required: target NATS account
  "token": "alice:secret",       // required: provider-specific credential
  "ap": "local"                  // optional: authentication provider id
}
```

- **File provider:** `token` = `"username:password"`
- **JWT provider:** `token` = raw external JWT string

---

## Configuration Structure

```json
{
  "account": { "type": "static|operator", ... },
  "policy":  { "type": "file", "file": { "policiesPath", "bindingsPath" } },
  "auth":    { "file": [...], "jwt": [...] },
  "server":  { "natsUrl", "natsCredentials|natsNkey", "xkeySeedFile", "ttl" }
}
```

See [auth-controller-callout spec](2026-02-06-auth-controller-callout.md) for full config reference.

---

## CLI

```bash
# Build
go build -o bin/nauts ./cmd/nauts

# One-shot authentication (outputs JWT)
./bin/nauts auth -c nauts.json -token '{"account":"APP","token":"alice:secret"}'

# Run auth callout service
./bin/nauts serve -c nauts.json
```

---

## External Dependencies

| Dependency | Purpose |
|-----------|---------|
| `github.com/nats-io/jwt/v2` | NATS JWT encoding/decoding |
| `github.com/nats-io/nats.go` | NATS client for auth callout |
| `github.com/nats-io/nkeys` | Cryptographic key operations |
| `golang.org/x/crypto/bcrypt` | Password hashing (file auth) |
| `github.com/golang-jwt/jwt/v5` | External JWT verification (JWT auth) |

---

## Known Limitations / Future Work

| Area | Limitation | Planned Enhancement |
|------|-----------|-------------------|
| Deny rules | Only `allow` effect | `deny` with evaluation order |
| Inbox scoping | Global `_INBOX.>` | Per-user inbox prefix |
| Resource limits | Not in policy model | `maxSubscriptions`, `maxPayload` |
| Configuration | File-only, restart required | NATS KV provider, hot-reload |
| Caching | None | Permission cache by (user, roles) |
| Observability | Basic logging only | Metrics, tracing, health checks |
| Policy simulation | Not available | Dry-run API for testing permissions |
| Multi-auth accounts | Hardcoded `AUTH` | Configurable auth account |

---

## Updating These Specs

When implementing new features:

1. **Start here** — update the system overview if the architecture changes
2. **Update the affected component spec** — add new types, methods, or design decisions
3. **Create a new spec file** for entirely new components: `specs/YYYY-MM-DD-<name>.md`
4. **Keep the spec index table** in this file current
5. **Mark deprecated decisions** with a note rather than deleting them
