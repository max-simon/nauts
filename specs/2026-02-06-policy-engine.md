# Specification: Policy Engine (`policy/`)

**Date:** 2026-02-06  
**Status:** Current  
**Package:** `policy`  
**Dependencies:** None (standalone)

---

## Goal

Provide a self-contained engine that translates human-friendly, high-level access policies into low-level NATS permissions. The policy engine is the heart of nauts — it decouples permission authoring from NATS protocol details.

## Summary

The `policy` package defines the full lifecycle of a permission policy: types for policies and statements, a resource naming scheme (NRN), an action registry with group expansion, a template interpolation engine, an action→permission mapper, and a compiler that ties everything together. The output is a `NatsPermissions` struct containing concrete NATS publish/subscribe subjects ready to be embedded into a NATS user JWT.

---

## Scope

- Policy, Statement, and Effect types  
- Resource (NRN) parsing and validation  
- Action registry and group expansion  
- Variable interpolation (`{{ user.id }}`, `{{ role.name }}`, …)  
- Action + Resource → NATS permission mapping  
- Permission aggregation with wildcard-aware deduplication  
- Compilation orchestration (`Compile`)

**Out of scope:** Policy storage, role bindings, authentication, JWT issuance.

**Note**: currently, queue wildcards are not handled in deduplication. For example `foo bar` and `foo *` are both returned from `Deduplicate`.

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Allow-only statements** | Deny rules add evaluation-order complexity. The system starts with allow-only; deny is reserved for a future phase. |
| **NRN scheme** (`type:id[:subid]`) | A concise, human-readable identifier for NATS objects. Colon-separated to avoid ambiguity with NATS dot-delimited subjects. |
| **Action groups expand recursively** | Groups like `js.worker` include `js.viewer` which itself is a group. Recursive expansion keeps definitions DRY. |
| **Interpolation excludes on failure** | If a variable cannot be resolved or fails validation, the entire resource is silently excluded (with a warning). This is a safe default — missing context never widens permissions. |
| **Wildcard-aware dedup** | NATS subjects `foo.bar` and `foo.>` overlap. The deduplicator removes subjects already covered by broader wildcards to keep JWTs compact. |
| **`_INBOX.>` is implicit for JS/KV** | All JetStream and KV operations use request/reply. Rather than forcing users to declare inbox subscriptions, they are added automatically when any JS/KV action is granted. |

---

## Public API

### Types

#### `Policy`
```go
type Policy struct {
    ID         string      `json:"id"`
    Name       string      `json:"name"`
    Statements []Statement `json:"statements"`
}
func (p *Policy) Validate() error
```
A named collection of permission statements. `ID` is the unique key used by providers.

#### `Statement`
```go
type Statement struct {
    Effect    Effect   `json:"effect"`
    Actions   []Action `json:"actions"`
    Resources []string `json:"resources"`
}
func (s *Statement) Validate() error
```
A single rule: grant (`allow`) a set of actions on a set of resources.

#### `Effect`
```go
type Effect string
const EffectAllow Effect = "allow"
func (e Effect) IsValid() bool
```
Currently only `"allow"` is valid.

#### `Action`
```go
type Action string
func (a Action) IsValid() bool
func (a Action) RequiresInbox() bool
```
A string-typed action identifier. Atomic actions: `nats.pub`, `nats.sub`, `nats.req`, `js.readStream`, `js.writeStream`, `js.deleteStream`, `js.readConsumer`, `js.writeConsumer`, `js.deleteConsumer`, `js.consume`, `kv.read`, `kv.write`, `kv.watchBucket`, `kv.readBucket`, `kv.writeBucket`, `kv.deleteBucket`. Groups: `nats.*`, `js.viewer`, `js.worker`, `js.*`, `kv.reader`, `kv.writer`, `kv.*`.

#### `ActionDef`
```go
type ActionDef struct {
    Name          string
    IsAtomic      bool
    RequiresInbox bool
    ExpandsTo     []Action  // non-empty for groups
}
```
Registry entry describing an action's properties.

#### `Resource`
```go
type Resource struct {
    Type          ResourceType  // "nats", "js", "kv"
    Identifier    string
    SubIdentifier string
    Raw           string
}
func (n *Resource) HasSubIdentifier() bool
func (n *Resource) String() string
func (n *Resource) FullType() ResourceType
```
A parsed NRN. Full types: `nats:subject`, `nats:subject:queue`, `js:stream`, `js:stream:consumer`, `kv:bucket`, `kv:bucket:entry`.

#### `NatsPermissions`
```go
type NatsPermissions struct { /* pub, sub */ }
func NewNatsPermissions() *NatsPermissions
func (p *NatsPermissions) Allow(perm Permission)
func (p *NatsPermissions) Merge(other *NatsPermissions)
func (p *NatsPermissions) Deduplicate()
func (p *NatsPermissions) PubList() []Permission
func (p *NatsPermissions) SubList() []Permission
func (p *NatsPermissions) IsEmpty() bool
func (p *NatsPermissions) ToNatsJWT() natsjwt.Permissions
```
Accumulator for compiled NATS permissions. Supports pub and sub. Queue subscriptions are stored as Permissions in the unified sub list. `Deduplicate()` removes subjects covered by wildcards, respecting queue group logic. `ToNatsJWT()` converts to NATS JWT format, merging queue subscriptions into the general allow list as separate queue restrictions are not supported in standard NATS JWTs.

#### `Permission` / `PermissionType`
```go
type PermissionType string  // "pub" or "sub"
type Permission struct {
    Type    PermissionType
    Subject string
    Queue   string  // only for sub
}
```

#### Context types
```go
type PolicyContext struct { /* key/value claims for interpolation */ }
```
`PolicyContext` stores a flat map of keys (e.g. `"user.id"`, `"account.id"`, `"role.name"`) to values.

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `ParseResource` | `(s string) (*Resource, error)` | Parse NRN format, no wildcard validation |
| `ParseAndValidateResource` | `(s string) (*Resource, error)` | Parse + validate wildcards |
| `ValidateResource` | `(n *Resource) error` | Validate wildcard rules per resource type |
| `ResolveActions` | `(actions []Action) []Action` | Expand groups to flat list of atomic actions |
| `InterpolateWithContext` | `(template string, ctx *PolicyContext) InterpolationResult` | Replace `{{ var }}` placeholders |
| `ContainsVariables` | `(s string) bool` | Quick check for template variables |
| `MapActionToPermissions` | `(action Action, n *Resource) []Permission` | Convert (action, resource) → NATS permissions |
| `Compile` | `(policies []*Policy, ctx *PolicyContext, perms *NatsPermissions) CompileResult` | Full compilation: expand → interpolate → parse → map → merge |

### Error Types

| Error | Sentinel | Meaning |
|-------|----------|---------|
| `PolicyError` | — | Structured error with code, message, attrs |
| `ValidationError` | — | Field-level validation failure |
| `ErrInvalidResource` | ✓ | Malformed NRN |
| `ErrUnknownResourceType` | ✓ | NRN type not `nats`, `js`, or `kv` |
| `ErrInvalidWildcard` | ✓ | Wildcard in disallowed position |
| `ErrUnknownAction` | ✓ | Action not in registry |

---

## NRN Wildcard Rules

| Resource Part | `*` allowed | `>` allowed |
|---------------|:-----------:|:-----------:|
| NATS subject | ✓ | ✓ |
| NATS queue | ✓ | ✗ |
| JS stream | ✓ | ✗ |
| JS consumer | ✓ | ✗ |
| KV bucket | ✓ | ✗ |
| KV entry (key) | ✓ | ✓ |

## Interpolation Sanitization Rules

Interpolated values must match `^[a-zA-Z0-9_\-\.]+$`. Empty strings, `*`, and `>` are rejected. On failure the resource is excluded (not the entire policy).

---

## Compilation Flow

```
Policies
  └─► for each Statement (effect=allow)
        ├─► ResolveActions(stmt.Actions)       → []Action (flat)
        └─► for each resource string
              ├─► InterpolateWithContext(resource, ctx) → resolved string
              ├─► ParseAndValidateResource(resolved)   → *Resource
              └─► for each action
                    ├─► MapActionToPermissions(action, resource) → []Permission
                    └─► if action.RequiresInbox → allow SUB _INBOX.>
  └─► caller calls perms.Deduplicate()
```

---

## Known Limitations / Future Work

- **No deny rules:** Only `effect: "allow"` is supported. Deny with evaluation order is planned.
- **`_INBOX.>` is global:** All JS/KV users share the same inbox namespace. Per-user inbox scoping is planned.
- **No resource limits:** `maxSubscriptions`, `maxPayload` etc. are not part of the policy model yet.
