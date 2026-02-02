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
- **NATS Client**: github.com/nats-io/nats.go (future)
- **JWT Handling**: github.com/nats-io/jwt/v2 (future)
- **NKeys**: github.com/nats-io/nkeys (future)
- **Password Hashing**: golang.org/x/crypto/bcrypt
- **Testing**: Standard library
- **Configuration**: Environment variables + JSON files

## Project Structure

```
nauts/
├── cmd/
│   └── nauts/              # Main entrypoint (future)
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
├── auth/                   # Authentication service
│   ├── service.go          # AuthService (permission compilation orchestrator)
│   ├── errors.go           # Auth errors (AuthError)
│   ├── model/              # Core domain types
│   │   ├── user.go         # User type
│   │   └── group.go        # Group type, ValidationError
│   ├── provider/           # Group and policy providers
│   │   ├── provider.go     # GroupPolicyProvider interface
│   │   ├── errors.go       # Provider errors (ErrPolicyNotFound, ErrGroupNotFound)
│   │   └── grouppolicyprovider/
│   │       └── file.go     # FileGroupPolicyProvider (JSON file backend)
│   ├── identity/           # User identity providers
│   │   ├── identity.go     # UserIdentityProvider interface, IdentityToken
│   │   ├── errors.go       # Identity errors (ErrInvalidCredentials, ErrUserNotFound)
│   │   └── static/
│   │       └── static.go   # StaticProvider (bcrypt password verification)
│   └── callout/            # Auth callout service
│       ├── callout.go      # CalloutService (orchestrates auth flow)
│       └── errors.go       # CalloutError
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
- Auth service: `AuthService`, `NewAuthService()`, `GetNatsPermission()`
- Permissions: `NatsPermissions`, `Permission`, `PermissionSet`
- Providers: `GroupPolicyProvider`, `FileGroupPolicyProvider`, `UserIdentityProvider`, `StaticProvider`
- Callout: `CalloutService`, `AuthResult`, `CalloutError`

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

# Run with file-based config
./bin/nauts --config config.json
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

The `auth.AuthService` orchestrates compilation:
1. Resolve user's groups (including default group)
2. For each group, fetch policies from provider
3. Call `policy.Compile()` with user/group context
4. Deduplicate permissions with wildcard awareness
5. Return final `NatsPermissions`

### Wildcard-Aware Deduplication

`NatsPermissions.Deduplicate()` removes redundant subjects:
- `foo.bar` covered by `foo.*` → remove `foo.bar`
- `foo.bar` covered by `foo.>` → remove `foo.bar`
- `foo.*` covered by `foo.>` → remove `foo.*`

### Auth Callout Service

The `callout.CalloutService` orchestrates the full authentication flow:
1. Verify identity token via `UserIdentityProvider` → returns `*model.User`
2. Compile permissions via `AuthService.GetNatsPermission()` → returns `*policy.NatsPermissions`
3. Return `AuthResult` containing user and permissions

Usage:
```go
// Setup providers
policyProvider, _ := grouppolicyprovider.NewFile(grouppolicyprovider.FileConfig{
    PoliciesPath: "policies.json",
    GroupsPath:   "groups.json",
})
identityProvider, _ := static.New(static.Config{UsersPath: "users.json"})

// Create services
authService := auth.NewAuthService(policyProvider)
calloutService := callout.NewCalloutService(identityProvider, authService)

// Authenticate
result, err := calloutService.Authenticate(ctx, static.UsernamePassword{
    Username: "alice",
    Password: "secret",
})
// result.User, result.Permissions available
```

### Identity Providers

Identity providers verify credentials and return user information:

**StaticProvider** (`auth/identity/static`):
- Loads users from JSON file
- Verifies passwords using bcrypt
- Token type: `static.UsernamePassword{Username, Password}`

## Dependencies

```go
require (
    golang.org/x/crypto v0.x         // bcrypt for password hashing
    github.com/nats-io/nats.go v1.x  // future
    github.com/nats-io/jwt/v2 v2.x   // future
    github.com/nats-io/nkeys v0.x    // future
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
- [x] Auth service: `auth/service.go` - `AuthService` with `GetNatsPermission()`
- [x] Group/Policy provider: `auth/provider/` - `GroupPolicyProvider` interface
- [x] File-based provider: `auth/provider/grouppolicyprovider/` - JSON file backend
- [x] Identity provider: `auth/identity/` - `UserIdentityProvider` interface
- [x] Static identity provider: `auth/identity/static/` - Username/password with bcrypt
- [x] Auth callout service: `auth/callout/` - `CalloutService` orchestrating auth flow
- [ ] NATS auth callout integration: Wire up to NATS auth callout protocol
- [ ] NATS KV provider: Future
- [ ] JWT identity provider: Future
- [ ] Control plane: Future
