# Specification: Providers (`provider/`)

**Date:** 2026-02-06  
**Status:** Current  
**Package:** `provider`  
**Dependencies:** `policy` (types), `jwt` (`Signer`, `LocalSigner`)

---

## Goal

Abstract the storage and retrieval of NATS accounts, policies, and role bindings behind provider interfaces. This lets the system swap backends (file, NATS KV, database) without changing the authentication or compilation logic.

## Summary

The `provider` package defines two core interfaces — `AccountProvider` (NATS account management) and `PolicyProvider` (policy and role-binding lookup) — and ships file/static/operator implementations for each. It also defines the `Account` entity that bundles a name, public key, and signer. Role bindings map `(account, role)` pairs to sets of policy IDs.

---

## Scope

- `AccountProvider` interface and two implementations (Operator, Static)  
- `PolicyProvider` interface and one implementation (File)  
- `Account` entity  
- Role bindings model  

**Out of scope:** User identity, authentication, JWT issuance, policy compilation.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Two account provider modes** | NATS supports simple (static) and production (operator) key hierarchies. Each mode has fundamentally different signing behavior, so separate implementations are cleaner than mode flags. |
| **`IsOperatorMode()` on `AccountProvider`** | Callers (auth controller, callout service) need to adjust JWT fields (`Audience` vs `IssuerAccount`) based on mode. This method avoids type assertions. |
| **`Account` holds a `Signer`** | Each account needs to sign user JWTs. Encapsulating the signer in the entity keeps signing concerns within the provider boundary. |
| **Bindings key = `(account, role)`** | A role's meaning is always scoped to an account. The composite key `account.role` ensures uniqueness without a separate ID. |
| **Missing policies don't fail auth** | If a binding references a policy ID that doesn't exist, the policy is silently skipped. This prevents a single misconfigured binding from blocking all authentication. |
| **`default` role** | Every user implicitly belongs to the `default` role. This enables baseline permissions without explicit role assignment. |
| **Data loaded once at startup** | File-based providers load all data into memory during initialization. This is simple and fast but requires restart for changes. Future KV-based providers will support dynamic updates. |

---

## Public API

### Entity

#### `Account`
```go
type Account struct { /* name, publicKey, signer */ }
func (a *Account) Name() string
func (a *Account) PublicKey() string
func (a *Account) Signer() jwt.Signer
```
Represents a NATS account with its signing capability.

### Interfaces

#### `AccountProvider`
```go
type AccountProvider interface {
    GetAccount(ctx context.Context, name string) (*Account, error)
    ListAccounts(ctx context.Context) ([]*Account, error)
    IsOperatorMode() bool
}
```

| Method | Purpose |
|--------|---------|
| `GetAccount` | Retrieve account by name. Returns `ErrAccountNotFound` if missing. |
| `ListAccounts` | Return all known accounts. |
| `IsOperatorMode` | `true` for operator mode (per-account signing keys), `false` for static mode (shared key). |

#### `PolicyProvider`
```go
type PolicyProvider interface {
    GetPolicy(ctx context.Context, id string) (*policy.Policy, error)
  GetPoliciesForRole(ctx context.Context, role identity.Role) ([]*policy.Policy, error)
  GetPolicies(ctx context.Context, account string) ([]*policy.Policy, error)
}
```

| Method | Purpose |
|--------|---------|
| `GetPolicy` | Retrieve a policy by its ID. Returns `ErrPolicyNotFound` if missing. |
| `GetPoliciesForRole` | Resolve all policies for a role via bindings (`role.Account`, `role.Name`). Returns `ErrRoleNotFound` if no binding exists. Missing policies within a valid binding are silently skipped. |
| `GetPolicies` | Return all policies applicable to an account (including global policies). |

### Implementations

#### `OperatorAccountProvider`
```go
type OperatorAccountProviderConfig struct {
    Accounts map[string]AccountSigningConfig
}
type AccountSigningConfig struct {
    PublicKey      string
    SigningKeyPath string  // .nk file
}
func NewOperatorAccountProvider(cfg OperatorAccountProviderConfig) (*OperatorAccountProvider, error)
```
- Each account has its own signing key loaded from an `.nk` file.
- `IsOperatorMode()` → `true`.
- JWT `IssuerAccount` = signing key's public key.

#### `StaticAccountProvider`
```go
type StaticAccountProviderConfig struct {
    PublicKey      string
    PrivateKeyPath string
    Accounts       []string
}
func NewStaticAccountProvider(cfg StaticAccountProviderConfig) (*StaticAccountProvider, error)
```
- One shared signing key for all accounts.
- `IsOperatorMode()` → `false`.
- JWT `Audience` = account name.

#### `FilePolicyProvider`
```go
type FilePolicyProviderConfig struct {
    PoliciesPath string
    BindingsPath string
}
func NewFilePolicyProvider(cfg FilePolicyProviderConfig) (*FilePolicyProvider, error)
```

**Policies file format:**
```json
[
  {
    "id": "read-access",
    "name": "Read Access",
    "statements": [
      { "effect": "allow", "actions": ["nats.sub"], "resources": ["nats:public.>"] }
    ]
  }
]
```

**Bindings file format:**
```json
[
  { "role": "readonly", "account": "APP", "policies": ["read-access"] },
  { "role": "admin", "account": "APP", "policies": ["read-access", "write-access"] }
]
```

**`GetPoliciesForRole` algorithm:**
1. Build key `account.role` from parameters
2. Look up binding by key → `ErrRoleNotFound` if missing
3. Collect unique, sorted policy IDs from binding
4. Resolve each policy ID via `GetPolicy`; skip `ErrPolicyNotFound`
5. Return resolved policy list

### Sentinel Errors

| Error | Meaning |
|-------|---------|
| `ErrAccountNotFound` | Account name not registered |
| `ErrPolicyNotFound` | Policy ID not loaded |
| `ErrRoleNotFound` | No binding for `(account, role)` pair |

---

## Role Binding Model

```
Binding {
    Role     string    // e.g., "readonly"
    Account  string    // e.g., "APP"
    Policies []string  // e.g., ["read-access"]
}

Key: "{account}.{role}" → unique
```

- No global bindings (`account="*"`) in current implementation
- The `default` role is implicit — if a binding exists for `(APP, default)`, those policies apply to all APP users

---

## Known Limitations / Future Work

- **File-based only:** No dynamic backend (NATS KV, database). Changes require restart.
- **No policy versioning:** Policies are identified by ID only. There is no version or change history.
- **No cascading bindings:** A role in account `APP` only resolves bindings with `account=APP`. There is no inheritance from a parent or global scope.
- **Missing policies are silent:** A binding referencing a non-existent policy ID produces no error. This could mask misconfiguration.
