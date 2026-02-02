# CLAUDE.md

This file provides context to Claude Code for working on the nauts project.

## Project Overview

nauts (**N**ATS **Aut**hentication **S**ervice) is a framework for scalable, human-friendly permission management for NATS. It provides:

- **Policy specification and compilation engine**: Translates high-level policies to low-level NATS permissions
- **Authentication service**: NATS auth callout implementation
- **Control plane**: Management API for policies, groups, and accounts (future)

See [README.md](./README.md) for architecture and [POLICY.md](./POLICY.md) for policy specification.

## Tech Stack

- **Language**: Go 1.22+
- **NATS Client**: github.com/nats-io/nats.go
- **JWT Handling**: github.com/nats-io/jwt/v2
- **NKeys**: github.com/nats-io/nkeys
- **Password Hashing**: golang.org/x/crypto/bcrypt
- **Testing**: Standard library
- **Configuration**: Environment variables + JSON files

## Project Structure

```
nauts/
├── cmd/
│   └── nauts/              # CLI entrypoint
│       └── main.go         # Authentication CLI
├── policy/                 # Policy types, compilation, interpolation, action mapping
│   ├── action.go           # Action types and action group expansion
│   ├── compile.go          # Policy compilation (Compile function)
│   ├── context.go          # Interpolation context types (UserContext, GroupContext)
│   ├── errors.go           # Policy errors (PolicyError, ValidationError)
│   ├── interpolate.go      # Variable interpolation ({{ user.id }}, etc.)
│   ├── mapper.go           # Action+Resource to NATS permissions mapping
│   ├── permissions.go      # NatsPermissions with Allow/Deny and wildcard dedup
│   ├── policy.go           # Policy, Statement, Effect types
│   └── resource.go         # Resource parsing and validation
├── provider/               # Entity and policy/group providers
│   ├── entity.go           # Operator, Account types with Signer
│   ├── entity_provider.go  # EntityProvider interface
│   ├── nsc_entity_provider.go # NscEntityProvider (reads nsc directory)
│   ├── nauts_provider.go   # NautsProvider interface (policies/groups)
│   ├── file_nauts_provider.go # FileNautsProvider (JSON file backend)
│   ├── group.go            # Group type with validation
│   └── errors.go           # Provider errors (ErrNotFound, etc.)
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # UserIdentityProvider interface, IdentityToken
│   └── file_user_provider.go # FileUserIdentityProvider (bcrypt passwords)
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys-based signing)
│   └── user.go             # IssueUserJWT function
├── auth/                   # Authentication controller
│   ├── controller.go       # AuthController (orchestrates auth flow)
│   └── errors.go           # Auth errors (AuthError)
├── testdata/               # Test fixtures (policies, groups, users)
└── docs/                   # Additional documentation
```

## Go Conventions

### Code Style

- Follow standard Go conventions (gofmt, go vet, staticcheck)
- Use meaningful variable names; avoid single letters except in loops/short lambdas
- Prefer early returns over deep nesting
- Group imports: stdlib, external, internal

### Error Handling

- Wrap errors with context: `fmt.Errorf("compiling policy %s: %w", id, err)`
- Define sentinel errors for expected failure modes
- Use custom error types when callers need to inspect error details

### Naming

- Resource types: `Resource`, `ResourceType`, `ParseAndValidateResource()`
- Policy types: `Policy`, `Statement`, `Effect`
- Actions: `Action`, `ActionDef`, `ResolveActions()`
- Auth controller: `AuthController`, `NewAuthController()`, `Authenticate()`, `ResolveUser()`, `ResolveNatsPermissions()`, `CreateUserJWT()`
- Permissions: `NatsPermissions`, `Permission`, `PermissionSet`
- Providers: `EntityProvider`, `NscEntityProvider`, `NautsProvider`, `FileNautsProvider`, `UserIdentityProvider`, `FileUserIdentityProvider`
- JWT: `Signer`, `LocalSigner`, `IssueUserJWT()`
- Entities: `Operator`, `Account`, `Group`, `User`

### Testing

- Table-driven tests for parsing and compilation
- Test files alongside implementation: `policy.go` + `policy_test.go`
- Use testdata/ for fixture files
- Aim for >80% coverage on core logic (policy, auth packages)

## Common Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linter
golangci-lint run

# Build binary
go build -o bin/nauts ./cmd/nauts

# Authenticate and get JWT
./bin/nauts -nsc-dir ~/.nsc -operator myop \
  -policies policies.json -groups groups.json -users users.json \
  -username alice -password secret -user-pubkey UABC123...
```

## Key Implementation Notes

### Resource Parsing

Resources follow pattern `<type>:<identifier>[:<sub-identifier>]`. Parser must:
- Validate type is one of: `nats`, `js`, `kv`
- Validate wildcards (`*`, `>`) are only in allowed positions per type
- Handle variable interpolation `{{ var.path }}`

Resource types include subidentifier variants:
- `nats:subject`, `nats:subject:queue`
- `js:stream`, `js:stream:consumer`
- `kv:bucket`, `kv:bucket:entry`

### Variable Interpolation

- Custom template engine with `{{ var.path }}` syntax
- Supported variables: `user.id`, `user.account`, `user.attr.<key>`, `group.id`, `group.name`
- Sanitize interpolated values: reject `*`, `>`, empty strings
- Allow only: `[a-zA-Z0-9_\-\.]+`
- Return excluded result (skip resource) on validation failure

### Action System

Actions use an `ActionDef` registry with attributes:
- `Name`: action identifier (e.g., "nats.pub")
- `IsAtomic`: true for atomic actions, false for groups
- `RequiresInbox`: true if action needs `_INBOX.>` subscription
- `ExpandsTo`: list of actions for groups (recursive expansion)

### Permission Compilation

The `policy.Compile()` function transforms policies to NATS permissions:
1. For each policy statement with effect "allow"
2. Expand action groups to atomic actions
3. Interpolate variables in resources
4. Parse and validate resources
5. Map actions + resources to NATS permissions
6. Add `_INBOX.>` for actions that require it
7. Merge into result permissions

The `auth.AuthController` orchestrates the full authentication flow:
1. Verify identity token via `UserIdentityProvider` → returns `*identity.User`
2. Resolve user's groups (including default group) from `NautsProvider`
3. For each group, fetch policies and call `policy.Compile()` with user/group context
4. Deduplicate permissions with wildcard awareness
5. Create signed JWT via `jwt.IssueUserJWT()` using account's signer from `EntityProvider`
6. Return `AuthResult` containing user, permissions, and JWT

### Wildcard-Aware Deduplication

`NatsPermissions.Deduplicate()` removes redundant subjects:
- `foo.bar` covered by `foo.*` → remove `foo.bar`
- `foo.bar` covered by `foo.>` → remove `foo.bar`
- `foo.*` covered by `foo.>` → remove `foo.*`

### Auth Controller

The `auth.AuthController` orchestrates the full authentication flow, combining user resolution, permission compilation, and JWT creation:

Usage:
```go
// Setup providers
entityProvider, _ := provider.NewNscEntityProvider(provider.NscConfig{
    Dir:          "~/.nsc",
    OperatorName: "myoperator",
})

nautsProvider, _ := provider.NewFileNautsProvider(provider.FileNautsProviderConfig{
    PoliciesPath: "policies.json",
    GroupsPath:   "groups.json",
})

identityProvider, _ := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
    UsersPath: "users.json",
})

// Create controller
controller := auth.NewAuthController(entityProvider, nautsProvider, identityProvider)

// Authenticate (combined flow)
result, err := controller.Authenticate(ctx, identity.UsernamePassword{
    Username: "alice",
    Password: "secret",
}, userPublicKey, time.Hour)
// result.User, result.Permissions, result.JWT available

// Or use individual methods
user, err := controller.ResolveUser(ctx, token)
perms, err := controller.ResolveNatsPermissions(ctx, user)
jwt, err := controller.CreateUserJWT(ctx, user, userPubKey, perms, ttl)
```

### Identity Providers

Identity providers verify credentials and return user information:

**FileUserIdentityProvider** (`identity/`):
- Loads users from JSON file
- Verifies passwords using bcrypt
- Token type: `identity.UsernamePassword{Username, Password}`

## Dependencies

```go
require (
    golang.org/x/crypto v0.x         // bcrypt for password hashing
    github.com/nats-io/jwt/v2 v2.x   // JWT encoding/decoding
    github.com/nats-io/nkeys v0.x    // cryptographic signing
)
```

## Current Status

- [x] Policy specification: Defined in POLICY.md
- [x] Resource parser: `policy/resource.go` - Resource parsing and validation
- [x] Policy types: `policy/policy.go` - Policy, Statement, Effect types
- [x] Action types: `policy/action.go` - ActionDef registry and action group expansion
- [x] Variable interpolation: `policy/interpolate.go` - Template variable substitution
- [x] Action mapping: `policy/mapper.go` - Action+Resource to NATS permissions
- [x] Permissions: `policy/permissions.go` - Allow/Deny with wildcard deduplication
- [x] Compilation: `policy/compile.go` - `Compile()` function
- [x] Entity provider: `provider/entity_provider.go` - `EntityProvider` interface
- [x] Nsc entity provider: `provider/nsc_entity_provider.go` - Reads nsc directory structure
- [x] Nauts provider: `provider/nauts_provider.go` - `NautsProvider` interface (policies/groups)
- [x] File nauts provider: `provider/file_nauts_provider.go` - JSON file backend
- [x] Identity provider: `identity/provider.go` - `UserIdentityProvider` interface
- [x] File identity provider: `identity/file_user_provider.go` - Username/password with bcrypt
- [x] JWT issuance: `jwt/user.go` - `IssueUserJWT()` function
- [x] Signer: `jwt/signer.go` - `Signer` interface and `LocalSigner`
- [x] Auth controller: `auth/controller.go` - `AuthController` orchestrating full auth flow
- [x] CLI: `cmd/nauts/main.go` - Authentication CLI
- [ ] NATS auth callout integration: Wire up to NATS auth callout protocol
- [ ] NATS KV provider: Future
- [ ] JWT identity provider: Future
- [ ] Control plane: Future
