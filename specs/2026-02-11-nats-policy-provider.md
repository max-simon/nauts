# Specification: NATS KV Policy Provider

**Date:** 2026-02-11
**Status:** Draft
**Package:** `provider` (new implementation: `NatsPolicyProvider`)
**Dependencies:** `github.com/nats-io/nats.go` (JetStream / KeyValue API)

---

## Goal

Provide a `PolicyProvider` implementation backed by a NATS KeyValue bucket, enabling dynamic policy and binding management without service restarts. Policies and bindings are fetched on demand, cached with a configurable TTL, and kept fresh via a KV watcher that invalidates stale cache entries.

## Summary

`NatsPolicyProvider` implements the `PolicyProvider` interface using a single NATS KV bucket. Policies are stored under `<account>.policy.<policy-id>`, bindings under `<account>.binding.<role>`. Values are JSON-encoded. The provider fetches data from the bucket on request and caches results in memory. A configurable TTL controls maximum cache staleness. A KV watcher runs in the background to invalidate cache entries when the underlying data changes, providing near-real-time consistency without polling.

---

## Scope

- Implement `PolicyProvider` interface (`GetPolicy`, `GetPoliciesForRole`, `GetPolicies`)
- Single KV bucket for both policies and bindings
- Key schema: `<account>.policy.<policy-id>` and `<account>.binding.<role>`
- In-memory cache with configurable TTL
- KV watcher for proactive cache invalidation
- Integration with existing `Config` / `PolicyConfig` system

**Out of scope:**
- Write/admin operations (create, update, delete policies via the provider)
- Migration tooling from file-based to KV-based storage
- Multi-bucket or cross-cluster replication
- Encryption at rest (delegated to NATS server configuration)

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Single bucket** | Simplifies configuration and management. The key prefix scheme (`policy.` vs `binding.`) provides logical separation within one bucket. |
| **Key schema uses dots** | NATS KV keys support dots and treat them as subject hierarchy separators. This enables targeted watching (e.g., `APP.policy.*`) and natural grouping by account. |
| **Account as key prefix** | Scoping by account first enables `GetPolicies(account)` to use key listing with prefix `<account>.policy.>`. Global policies use literal `*` as the account segment: `*.policy.<id>`. |
| **JSON value encoding** | Consistent with `FilePolicyProvider` data format. Policies are stored as `policy.Policy` JSON, bindings as the existing `binding` struct JSON (same schema as the file bindings array entries). |
| **Fetch-on-demand with cache** | Avoids loading the entire bucket on startup. Only data that is actually requested gets cached. Suitable for deployments with large policy sets where most policies may not apply to a given nauts instance. |
| **TTL + watch dual strategy** | TTL provides a hard upper bound on staleness. Watch provides near-real-time invalidation. Together they balance consistency and resilience: if the watch connection drops, the TTL ensures the cache doesn't serve stale data indefinitely. |
| **Watch invalidates, does not populate** | On a watch event, the provider deletes the cache entry rather than replacing it with the new value. The next `Get*` call fetches the fresh value. This keeps the watch handler simple and avoids partial-update races. |
| **Missing policies don't fail auth** | Consistent with `FilePolicyProvider`: if a binding references a policy ID that does not exist in KV, the policy is silently skipped. |
| **Provider owns NATS connection** | The provider creates its own `nats.Conn` from the configured URL and credentials. This decouples it from the callout service connection and allows independent lifecycle management. |

---

## Public API

### Types

#### `NatsPolicyProviderConfig`

```go
type NatsPolicyProviderConfig struct {
    // Bucket is the name of the NATS KV bucket.
    Bucket string `json:"bucket"`

    // NatsURL is the NATS server URL (e.g., "nats://localhost:4222").
    NatsURL string `json:"natsUrl"`

    // NatsCredentials is the path to NATS credentials file.
    // Mutually exclusive with NatsNkey.
    NatsCredentials string `json:"natsCredentials,omitempty"`

    // NatsNkey is the path to the nkey seed file for NATS authentication.
    // Mutually exclusive with NatsCredentials.
    NatsNkey string `json:"natsNkey,omitempty"`

    // CacheTTL is how long cached entries remain valid.
    // Default: 30s.
    CacheTTL time.Duration `json:"cacheTtl,omitempty"`
}
```

#### `NatsPolicyProvider`

```go
type NatsPolicyProvider struct {
    // unexported fields: nats.Conn, jetstream.KeyValue, cache, watcher, config
}
```

### Constructor

```go
func NewNatsPolicyProvider(cfg NatsPolicyProviderConfig) (*NatsPolicyProvider, error)
```

1. Validate configuration (bucket name required, URL required, credentials exclusive)
2. Connect to NATS
3. Obtain JetStream context and open the KV bucket (bucket must already exist)
4. Initialize empty cache
5. Start background KV watcher
6. Return provider

Returns an error if connection or bucket access fails.

### Interface Methods

All three methods from `PolicyProvider`:

#### `GetPolicy(ctx context.Context, account string, id string) (*policy.Policy, error)`

1. Build KV key: `<account>.policy.<id>`.
2. Check cache for key. On cache miss or expired entry: fetch directly from KV by key.
3. Decode JSON into `*policy.Policy`, validate, cache, return.
4. Return `ErrPolicyNotFound` if no key exists.

#### `GetPoliciesForRole(ctx context.Context, role identity.Role) ([]*policy.Policy, error)`

1. Build binding key: `<role.Account>.binding.<role.Name>`
2. Check cache for binding. On miss/expired: fetch from KV, decode JSON, cache.
3. Return `ErrRoleNotFound` if key does not exist.
4. For each policy ID in the binding, call `GetPolicy(ctx, role.Account, id)`. If the ID starts with `_global:`, `GetPolicy` strips the prefix and resolves the policy from the global scope (account=`*`).
5. Skip policies that return `ErrPolicyNotFound` (consistent with `FilePolicyProvider`).
6. Return resolved policy list, sorted by ID.

#### `GetPolicies(ctx context.Context, account string) ([]*policy.Policy, error)`

1. List KV keys matching `<account>.policy.>` (account-specific policies).
2. List KV keys matching `*.policy.>` (global policies).
3. For each key, check cache or fetch value, decode, validate.
4. Return combined list, sorted by ID.

### Lifecycle

#### `Stop() error`

1. Stop the KV watcher
2. Drain and close the NATS connection
3. Clear the cache

---

## Key Schema

### Policy Keys

```
<account>.policy.<policy-id>
```

**Examples:**
```
APP.policy.read-access          → policy scoped to APP account
APP.policy.write-access         → policy scoped to APP account
*.policy.base-permissions       → global policy (applies to all accounts)
```

**Value:** JSON-encoded `policy.Policy` (same schema as entries in the file provider's policies JSON array):
```json
{
  "id": "read-access",
  "account": "APP",
  "name": "Read Access",
  "statements": [
    { "effect": "allow", "actions": ["nats.sub"], "resources": ["nats:public.>"] }
  ]
}
```

### Binding Keys

```
<account>.binding.<role>
```

**Examples:**
```
APP.binding.readonly            → readonly role in APP account
APP.binding.admin               → admin role in APP account
APP.binding.default             → default role in APP account
```

**Value:** JSON-encoded binding (same schema as entries in the file provider's bindings JSON array):
```json
{
  "role": "readonly",
  "account": "APP",
  "policies": ["read-access", "_global:base-permissions"]
}
```

Policy IDs prefixed with `_global:` reference global policies. When `GetPolicy` encounters this prefix, it strips `_global:` and looks up the policy with account=`*`, resolving to key `_global.policy.<id>`.

---

## Cache Design

### Structure

```go
type cacheEntry struct {
    value     any       // *policy.Policy or *binding
    expiresAt time.Time
}

type cache struct {
    mu      sync.RWMutex
    entries map[string]*cacheEntry // keyed by KV key
}
```

The cache is keyed by the full KV key (e.g., `APP.policy.read-access`). This ensures 1:1 mapping between KV entries and cache entries.

### Operations

| Operation | Behavior |
|-----------|----------|
| **Get** | Return value if present and `time.Now() < expiresAt`. Otherwise return nil (cache miss). |
| **Put** | Store value with `expiresAt = time.Now() + TTL`. |
| **Invalidate** | Delete entry by key. Next Get triggers a fresh fetch. |
| **InvalidatePrefix** | Delete all entries with a given prefix. Used when a watch event is received for a broad change. |
| **Clear** | Remove all entries. Used on `Stop()`. |

### Concurrency

The cache uses `sync.RWMutex`: reads acquire `RLock`, writes (put/invalidate) acquire full `Lock`. Fetch-on-miss acquires write lock only for the cache update, not for the NATS KV get (to avoid holding the lock during network I/O).

---

## KV Watcher

### Behavior

On startup, the provider creates a KV watcher on the entire bucket (`jetstream.UpdatesOnly()`). The watcher receives events for all key changes.

| Event Type | Action |
|------------|--------|
| `KeyValuePutOp` | Invalidate the cache entry for the changed key. |
| `KeyValueDeleteOp` | Invalidate the cache entry for the deleted key. |

The watcher runs in a dedicated goroutine launched by `NewNatsPolicyProvider`. It exits when `Stop()` is called.

### Resilience

If the watcher channel closes unexpectedly (e.g., NATS reconnect), the provider should attempt to re-establish the watcher with backoff. During the gap, the TTL-based expiration ensures cache entries become stale and are refetched.

---

## Configuration Integration

### `PolicyConfig` Extension

```go
type PolicyConfig struct {
    Type string `json:"type"` // "file" or "nats"
    File *provider.FilePolicyProviderConfig  `json:"file,omitempty"`
    Nats *provider.NatsPolicyProviderConfig  `json:"nats,omitempty"`
}
```

### Example Configuration

```json
{
  "policy": {
    "type": "nats",
    "nats": {
      "bucket": "nauts-policies",
      "natsUrl": "nats://localhost:4222",
      "natsCredentials": "/path/to/creds",
      "cacheTtl": "30s"
    }
  }
}
```

### Validation Rules

- `bucket` is required and must be non-empty
- `natsUrl` is required
- `natsCredentials` and `natsNkey` are mutually exclusive (at most one may be set)
- `cacheTtl` defaults to `30s` if not set; must be positive if set

---

## Error Handling

| Scenario | Behavior |
|----------|----------|
| NATS connection fails on startup | `NewNatsPolicyProvider` returns error |
| Bucket does not exist | `NewNatsPolicyProvider` returns error |
| KV get fails (network) | Return wrapped error from `Get*` method |
| Key not found in KV | `ErrPolicyNotFound` or `ErrRoleNotFound` as appropriate |
| JSON decode fails | Return wrapped error (malformed data in KV) |
| Policy validation fails | Return wrapped error |
| Watcher disconnects | Re-establish with backoff; TTL protects against stale cache |
| Cache miss + NATS unavailable | Return error (no fallback to stale cache) |

---

## `GetPolicy` Lookup Strategy

`GetPolicy(ctx, account, id)` receives both account and policy ID. The KV key is constructed directly as `<account>.policy.<id>`, enabling a single-key lookup with no scanning or secondary index required.

---

## Open Questions

### 1. NATS connection sharing

Should the provider reuse the same NATS connection as the callout service, or create its own?

- **Own connection (proposed):** Simpler lifecycle, independent reconnect behavior, avoids coupling to callout service. Slightly more resource usage (one extra TCP connection).
- **Shared connection:** Fewer connections, but requires passing `*nats.Conn` into the provider and coordinating shutdown.

**Proposed answer:** Own connection. The config already includes `natsUrl` and credentials. Provider manages its own connection.

### 2. Bucket provisioning

Should `NewNatsPolicyProvider` create the bucket if it doesn't exist, or require it to exist?

- **Require exists (proposed):** The provider is read-only. Bucket creation is an admin concern. Failing fast on a missing bucket surfaces misconfiguration early.
- **Auto-create:** Convenient for development, but implies the provider needs write permissions on JetStream.

**Proposed answer:** Require the bucket to exist. Document bucket creation as an operational prerequisite.

### 3. Cache TTL default

What is a reasonable default TTL?

- Short TTL (5-10s): More consistent but more KV traffic.
- Medium TTL (30s): Good balance; watch handles most invalidation, TTL is a safety net.
- Long TTL (5m+): Less load but higher staleness risk when watch is disrupted.

**Proposed answer:** 30 seconds default. Configurable via `cacheTtl`.

### 4. Global policy key prefix

Global policies have `account: "*"`. In the key schema, this becomes `*.policy.<id>`. NATS KV keys support `*` as a literal character. However, `*` is a wildcard in NATS subject space, which could cause issues with KV key listing/watching.

**Alternative:** Use a reserved prefix like `_global.policy.<id>` instead of `*.policy.<id>`.

**Proposed answer:** Use `_global` as the account segment for global policies. The provider maps `policy.Account == "*"` to key prefix `_global` and vice versa.

### 5. Stale cache on NATS failure

If NATS becomes unavailable after the cache is populated, should `Get*` methods return stale cached data or fail?

- **Fail (proposed):** Returning stale policies could grant revoked permissions. Failing forces the callout service to deny authentication, which is safer.
- **Serve stale:** Higher availability but risk of stale permissions.

**Proposed answer:** Fail. Do not serve data beyond its TTL even if the source is unavailable. Security over availability.

### 6. `GetPolicies` listing performance

`GetPolicies(account)` needs to return all policies for an account plus global policies. This requires listing keys with prefix `<account>.policy.` and `_global.policy.`. For buckets with many policies, is KV `ListKeys` efficient enough?

NATS KV `ListKeys` streams key names without values, then values are fetched individually. For large policy sets, this could be chatty. An alternative is to cache the full list of keys per account prefix and invalidate via the watcher.

**Proposed answer:** Accept the listing approach for initial implementation. The watcher can populate a per-account key list in the cache, avoiding KV `ListKeys` entirely on cache hits.

---

## Implementation Checklist

1. Add `NatsPolicyProviderConfig` struct to `provider/` package
2. Implement internal cache with TTL, invalidation, and thread safety
3. Implement `NatsPolicyProvider` with constructor, `GetPolicy`, `GetPoliciesForRole`, `GetPolicies`, `Stop`
4. Implement KV watcher goroutine with reconnect logic
5. Extend `PolicyConfig` in `auth/config.go` with `"nats"` type
6. Extend `NewAuthControllerWithConfig` to construct `NatsPolicyProvider`
7. Add config validation for the `"nats"` policy type
8. Unit tests for cache logic
9. Unit tests for key construction and parsing
10. Integration tests with embedded NATS server
11. E2E tests: add `policy-nats/` test configuration alongside existing `policy-static/`
