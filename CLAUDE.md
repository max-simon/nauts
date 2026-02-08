# CLAUDE.md

This file provides context to Claude Code for working on the nauts project.

> **Note**: See [AGENTS.md](./AGENTS.md) for specialized agent personas (@go-impl, @go-test, @reviewer, @architect, @docs) 
> that can be referenced in prompts to activate specific development roles and expertise.

## Documentation

- [README.md](./README.md) - Quick start, architecture, and key features
- [POLICY.md](./POLICY.md) - Detailed Policy and Action reference
- [IMPLEMENTATION.md](./IMPLEMENTATION.md) - Internal architecture and package details
- [AGENTS.md](./AGENTS.md) - Specialized agent personas for development
- [specs/](./specs/README.md) - Authoritative component specifications
  - [system-overview](./specs/2026-02-06-system-overview.md) - Dependency graph, deployment modes
  - [policy-engine](./specs/2026-02-06-policy-engine.md) - NRN, actions, interpolation, compilation
  - [jwt-issuance](./specs/2026-02-06-jwt-issuance.md) - Signer interface, IssueUserJWT
  - [identity-authentication](./specs/2026-02-06-identity-authentication.md) - User, AuthProvider, Manager
  - [providers](./specs/2026-02-06-providers.md) - AccountProvider, PolicyProvider
  - [auth-controller-callout](./specs/2026-02-06-auth-controller-callout.md) - AuthController, CalloutService
- [e2e/](./e2e/README.md) - End-to-End tests and test configurations

## Project Overview

nauts (**N**ATS **Aut**hentication **S**ervice) is a framework for scalable, human-friendly permission management for NATS. It provides:

- **Policy specification and compilation engine**: Translates high-level policies to low-level NATS permissions
- **Authentication service**: NATS auth callout implementation
- **Control plane**: Management API for policies, roles, and accounts (future)

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
│       └── main.go         # CLI with auth and serve subcommands
├── policy/                 # Policy types, compilation, interpolation, action mapping
│   ├── action.go           # Action types and action group expansion
│   ├── compile.go          # Policy compilation (Compile function)
│   ├── context.go          # Interpolation context types (UserContext, RoleContext)
│   ├── errors.go           # Policy errors (PolicyError, ValidationError)
│   ├── interpolate.go      # Variable interpolation ({{ user.id }}, etc.)
│   ├── mapper.go           # Action+Resource to NATS permissions mapping
│   ├── permissions.go      # NatsPermissions with Allow/Deny, wildcard dedup and queue handling
│   ├── policy.go           # Policy, Statement, Effect types
│   └── resource.go         # Resource parsing and validation
├── provider/               # Account, role, and policy providers
│   ├── entity.go           # Account type with Signer
│   ├── account_provider.go # AccountProvider interface
│   ├── operator_account_provider.go # OperatorAccountProvider (operator mode with signing keys)
│   ├── static_account_provider.go # StaticAccountProvider (single key for all accounts)
│   ├── policy_provider.go  # PolicyProvider interface
│   ├── file_policy_provider.go # FilePolicyProvider (JSON file backend)
│   └── errors.go           # Provider errors (ErrNotFound, etc.)
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # AuthenticationProvider interface, IdentityToken
│   └── file_authentication_provider.go # FileAuthenticationProvider (bcrypt passwords)
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys-based signing)
│   └── user.go             # IssueUserJWT function
├── auth/                   # Authentication controller and callout service
│   ├── controller.go       # AuthController (orchestrates auth flow)
│   ├── callout.go          # CalloutService (NATS auth callout handler)
│   ├── config.go           # Config types and NewAuthControllerWithConfig
│   └── errors.go           # Auth errors (AuthError)
├── e2e/                    # End-to-End tests
│   ├── connection_test.go  # Legacy connection tests (static/operator mode)
│   ├── policy_nats_test.go # Tests for Core NATS actions
│   ├── policy_jetstream_test.go # Tests for JetStream actions
│   ├── policy_kv_test.go   # Tests for KV actions
│   ├── env.go              # Test environment harness
│   ├── README.md           # Testing documentation
│   ├── common/             # Shared setup (users, policies, keys)
│   ├── policy-static/      # Policy engine test setup
│   ├── connection-operator/# Operator Mode config & setup
│   └── connection-static/  # Static Mode config & setup
└── docs/                   # Additional documentation
```

## Development Commands

### Build

```bash
go build -o bin/nauts ./cmd/nauts
```

### Test

```bash
# Run unit tests
go test -v ./...

# Run E2E tests (requires setup)
go test -v ./e2e/ -static -operator
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
- Auth controller: `AuthController`, `NewAuthController()`, `NewAuthControllerWithConfig()`, `Authenticate()`, `ResolveUser()`, `ResolveNatsPermissions()`, `CreateUserJWT()`
- Callout service: `CalloutService`, `NewCalloutService()`, `CalloutConfig`, `Start()`, `Stop()`
- Configuration: `Config`, `LoadConfig()`, `AccountConfig`, `PolicyConfig`, `IdentityConfig`, `ServerConfig`
- Permissions: `NatsPermissions`, `Permission`, `PermissionSet`
- Providers: `AccountProvider`, `OperatorAccountProvider`, `StaticAccountProvider`, `PolicyProvider`, `FilePolicyProvider`, `AuthenticationProvider`, `FileAuthenticationProvider`
- JWT: `Signer`, `LocalSigner`, `IssueUserJWT()`
- Entities: `Account`, `Role`, `User`

### Testing

- Table-driven tests for parsing and compilation
- Test files alongside implementation: `policy.go` + `policy_test.go`
- Use testdat:

Policy engine tests (`e2e/policy_*_test.go`):
- Verify policy compilation and permission enforcement (NATS, JetStream, KV)
- Tests use `policy-static/` configuration with rich test users
- Cover action mappings, variable interpolation, and permission isolation
- Test users: admin, writer, reader, service, alice, bob (all password: "secret")

Legacy connection tests (`e2e/connection_test.go`):
- Tests both static and operator modes
- Starts NATS server and nauts service automatically
- Verifies authentication flows
- Run with `-static` or `-operator` flag to select mode
- Verifies authentication, authorization, and permission enforcement
- Run with `-static` or `-operator` flag to select mode
- Test users: alice (readonly), bob (full access), password: "secret"

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

# One-shot authentication (get JWT)
# Token is JSON format: { "account": string, "token": string, "ap"?: string }
./bin/nauts auth -c nauts.json -token '{"account":"APP","token":"alice:secret"}'

# Run auth callout service
./bin/nauts serve -c nauts.json

# Run e2e tests (from e2e/ directory)
cd test
go test -v -static .   # Run static mode e2e tests
go test -v -operator . # Run operator mode e2e tests
```

### Configuration File Format

The CLI uses a JSON configuration file (`-c/--config`):

**Operator mode** (recommended for production with NATS operator/accounts):

```json
{
  "account": {
    "type": "operator",
    "operator": {
      "accounts": {
        "AUTH": {
          "publicKey": "AAUTH...",
          "signingKeyPath": "/path/to/auth-signing.nk"
        },
        "APP": {
          "publicKey": "AAPP...",
          "signingKeyPath": "/path/to/app-signing.nk"
        }
      }
    }
  },
  "policy": {
    "type": "file",
    "file": {
      "policiesPath": "policies.json",
      "bindingsPath": "bindings.json"
    }
  },
  "auth": {
    "file": [
      {
        "id": "local",
        "accounts": ["*"],
        "userPath": "users.json"
      }
    ]
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsNkey": "auth-service.nk",
    "xkeySeedFile": "xkey.seed",
    "ttl": "1h"
  }
}
```

**Static mode** (single key for all accounts, simpler setup):

```json
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "AXXXXX...",
      "privateKeyPath": "/path/to/account.nk",
      "accounts": ["AUTH", "APP"]
    }
  }
}
```

## Key Implementation Notes

### Account Provider Modes

nauts supports two account provider modes:

**Operator Mode** (`OperatorAccountProvider`):
- For NATS deployments with operator/account hierarchy
- Auth service runs in AUTH account but authenticates users for all accounts
- Each account has its own signing key
- Auth callout response includes `IssuerAccount` to identify the signing key
- User JWTs don't include audience (account determined by `IssuerAccount`)
- `IsOperatorMode()` returns `true`

**Static Mode** (`StaticAccountProvider`):
- Single signing key shared by all accounts
- Simpler setup for development or single-account deployments
- User JWTs include audience set to account name
- `IsOperatorMode()` returns `false`

### Client Authentication

Clients authenticate using a JSON token with the following structure:
```json
{ "account": "APP", "token": "username:password", "ap": "local" }
```

The `account` field is required. The `ap` field is optional and can be used to select a specific authentication provider by id.

**Static mode**: Clients connect with just a token:
```bash
nats --token '{"account":"APP","token":"alice:secret"}' pub test "hello"
```

**Operator mode**: Clients must use sentinel credentials + token. The sentinel user authenticates to the AUTH account, which triggers auth callout with the provided token:
```bash
nats --creds sentinel.creds --token '{"account":"APP","token":"alice:secret"}' pub test "hello"
```

The sentinel user is a special user in the AUTH account that:
- Has minimal permissions (pub/sub denied on `>`)
- Is listed in the account's `auth_users` to trigger auth callout
- Allows the NATS server to accept the connection and invoke auth callout

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
- Supported variables: `user.id`, `user.account`, `user.attr.<key>`, `role.name`, `role.account`
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

### JWT Permission Encoding

**Important:** NATS JWT defaults to allowing everything when no permissions are specified. The `jwt.IssueUserJWT()` function handles this by explicitly setting `Deny: [">"]` when no allow permissions are granted for a permission type (pub/sub). This ensures users without explicit permissions are denied access rather than granted full access.
2. `ResolveUser()` wraps identity in `*auth.AccountScopedUser` (identity + requested `Account`)
   - Filters roles to only those matching the requested account
3. For each role, fetch policies via `PolicyProvider.GetPoliciesForRole()`, then compile with user/role context
4. Deduplicate permissions with wildcard awareness
5. Create signed JWT via `jwt.IssueUserJWT()` using account's signer from `AccountProvider`
6. Return `AuthResult` containing user, permissions, and JWT

**AccountScopedUser**: Value-embedded `identity.User` with an `Account` field. Used to scope roles and
avoid re-deriving account context during permission compilation and JWT issuance.on flow:
1. Verify identity token via `AuthenticationProviderManager` → returns `*identity.User` (internal)
  - `ResolveUser()` returns `*auth.AccountScopedUser` (identity + requested `Account`)
2. Resolve user's roles (including default role) from identity provider claims
3. For each role, fetch policies via `PolicyProvider.GetPoliciesForRole()` (global+local), then compile with user/role context
4. Deduplicate permissions with wildcard awareness
5. Create signed JWT via `jwt.IssueUserJWT()` using account's signer from `AccountProvider`
6. Return `AuthResult` containing user, permissions, and JWT

### Wildcard-Aware Deduplication

`NatsPermissions.Deduplicate()` removes redundant subjects:
- `foo.bar` covered by `foo.*` → remove `foo.bar`
- `foo.bar` covered by `foo.>` → remove `foo.bar`
- `foo.*` covered by `foo.>` → remove `foo.*`

### Auth Controller

The `auth.AuthController` orchestrates the full authentication flow, combining user resolution, permission compilation, and JWT creation.

**Using configuration file (recommended)**:
```go
// Load config and create controller
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)

// Authenticate using ConnectOptions (same as auth callout)
// Token is JSON: { "account": string, "token": string, "ap"?: string }
result, err := controller.Authenticate(ctx, jwt.ConnectOptions{
  Token: `{"account":"APP","token":"alice:secret"}`,
}, userPublicKey, time.Hour)
// result.User, result.Permissions, result.JWT available
```

**Manual provider setup (operator mode)**:
```go
// Setup providers individually
accountProvider, _ := provider.NewOperatorAccountProvider(provider.OperatorAccountProviderConfig{
    Accounts: map[string]provider.AccountSigningConfig{
        "AUTH": {
            PublicKey:      "AAUTH...",
            SigningKeyPath: "/path/to/auth-signing.nk",
        },
        "APP": {
            PublicKey:      "AAPP...",
            SigningKeyPath: "/path/to/app-signing.nk",
        },
    },
})

policyProvider, _ := provider.NewFilePolicyProvider(provider.FilePolicyProviderConfig{
  PoliciesPath: "policies.json",
  BindingsPath: "bindings.json",
})

identityProvider, _ := identity.NewFileAuthenticationProvider(identity.FileAuthenticationProviderConfig{
    UsersPath: "users.json",
})

authProviders := identity.NewAuthenticationProviderManager(map[string]identity.AuthenticationProvider{
  "local": identityProvider,
})

// Create controller
controller := auth.NewAuthController(accountProvider, policyProvider, authProviders)

// Or use individual methods
user, err := controller.ResolveUser(ctx, token)
perms, err := controller.ResolveNatsPermissions(ctx, user)
jwt, err := controller.CreateUserJWT(ctx, user, userPubKey, perms, ttl)
```

### Callout Service

The `auth.CalloutService` implements the NATS auth callout protocol:

```go
// Create callout service (with credentials file)
service, _ := auth.NewCalloutService(controller, auth.CalloutConfig{
    NatsURL:         "nats://localhost:4222",
    NatsCredentials: "/path/to/creds",
    XKeySeed:        xkeySeed,  // Optional, for encrypted auth callout (read from file)
    DefaultTTL:      time.Hour,
})

// Or with nkey file (alternative to credentials file)
service, _ := auth.NewCalloutService(controller, auth.CalloutConfig{
    NatsURL:    "nats://localhost:4222",
    NatsNkey:   "auth-service.nk",  // Path to nkey seed file
    DefaultTTL: time.Hour,
})

// Start service (blocks until shutdown)
ctx, cancel := context.WithCancel(context.Background())
go func() {
    <-sigCh
    cancel()
    service.Stop()
}()
service.Start(ctx)
```

**Protocol Flow**:
1. Subscribe to `$SYS.REQ.USER.AUTH`
2. Decrypt request using service's xkey (if configured)
3. Decode `jwt.AuthorizationRequestClaims`
4. Extract username/password from `ConnectOptions`
5. Call `AuthController.Authenticate()`
6. Build `jwt.AuthorizationResponseClaims` with user JWT
7. In enticationProviderManager** (`identity/authentication_provider_manager.go`):
Routes authentication requests to the correct provider.
- If `req.AP` is set, selects provider by id
- If `req.AP` is empty, selects all providers that can manage `req.Account`
  - If exactly one matches, it is used; if none or many match, returns error
- Account patterns support "*" (all) and "prefix*" (prefix match)
- Wildcards do NOT match SYS or AUTH; those must be explicitly listed

**Errors**:
- `ErrAuthenticationProviderNotFound`: Explicit provider id cannot be resolved
- `ErrAuthenticationProviderAmbiguous`: Multiple providers match and no id specified
- `ErrAuthenticationProviderNotManageable`: Account not manageable by selected provider

**Authoperator mode, set `IssuerAccount` to the signing key's public key
8. Encrypt response with server's xkey (if provided)
9. Reply via `msg.Respond()`

**Error Handling**: Internal errors are logged but never leaked to clients. All authentication failures return generic "authentication failed" message.

### Identity Providers

Identity providers verify credentials and return user information.

**AuthRequest Type** (`identity/provider.go`):
The authentication token is expected to be a JSON object:
```go
type AuthRequest struct {
  Account string `json:"account"`           // Required
  Token   string `json:"token"`             // Provider-specific token (e.g., "user:pass")
  AP      string `json:"ap,omitempty"`      // Optional provider id
}
```

**FileAuthenticationProvider** (`identity/`):
- Loads users from JSON file
- Verifies passwords using bcrypt
- Token format within AuthRequest: `"username:password"` (colon-separated)
- The controller filters roles for the requested account (authorization)

**Users JSON file format**:
```json
{
  "users": {
    "alice": {
      "accounts": ["APP"],
      "roles": ["APP.readonly"],
      "passwordHash": "$2a$10$...",
      "attributes": { "department": "engineering" }
    },
    "bob": {
      "accounts": ["APP", "CORP"],
      "roles": ["APP.full"],
      "passwordHash": "$2a$10$..."
    }
  }
}
```

**Errors** (FileAuthenticationProvider):
- `ErrInvalidCredentials`: Password verification failed
- `ErrUserNotFound`: User does not exist
- `ErrInvalidTokenType`: Token format is wrong
- `ErrInvalidAccount`: Requested account is not valid for user
- `ErrAccountRequired`: User has multiple accounts but no account specified

**JwtAuthenticationProvider** (`identity/jwt_authentication_provider.go`):
Authenticates users using external JWTs (e.g., from Keycloak, Auth0, or other OIDC providers).

**How it works**:
1. **Verify signature**: Verify JWT signature using the provider's configured public key
2. **Validate issuer**: Ensure the `iss` claim matches the provider's configured issuer
3. **Extract roles**: Get roles from configurable claim path (default: `resource_access.nauts.roles`)
   - Roles must follow format `<account>.<role>` (e.g., `tenant-a.admin`, `app.viewer`)
   - Invalid formats are silently skipped
4. **Validate manageability**: Ensure the provider is allowed to manage the requested account
   - Configuration supports wildcards: `*` (any account), `tenant-*` (prefix match)
   - Wildcards in target account are rejected
5. **Return roles**: Return all roles; the controller filters roles by requested account

**Configuration**:
```json
{
  "auth": {
    "jwt": [
      {
        "id": "idp-1",
        "accounts": ["tenant-a-*", "dev"],
        "issuer": "https://auth.example.com/realms/myrealm",
        "publicKey": "<base64 encoded PEM public key>",
        "rolesClaimPath": "resource_access.nauts.roles"
      },
      {
        "id": "idp-2",
        "accounts": ["*"],
        "issuer": "https://another-idp.com",
        "publicKey": "<base64 encoded PEM public key>",
        "rolesClaimPath": "custom.claims.roles"
      }
    ]
  }
}
```

**Account pattern matching examples**:
| Target Account | Allowed Accounts | Result |
|----------------|------------------|--------|
| `tenant-a-acc-1` | `["tenant-a-*"]` | ✓ Valid |
| `tenant-b-acc-1` | `["tenant-a-*", "dev"]` | ✗ Denied |
| `dev` | `["tenant-a-*", "dev"]` | ✓ Valid |
| `any-account` | `["*"]` | ✓ Valid |

**Token format**:
The `AuthRequest.Token` is the raw JWT string from the external identity provider.
```json
{ "account": "tenant-a-acc-1", "token": "eyJhbGciOiJSUzI1NiIs..." }

Optionally select a specific auth provider by id:
```json
{ "account": "tenant-a-acc-1", "token": "eyJhbGciOiJSUzI1NiIs...", "ap": "idp-1" }
```
```

**Extracted user attributes**:
For now, nauts only extracts the standard subject claim:
- `sub` → `attributes["sub"]`

**Errors** (JwtAuthenticationProvider):
- `ErrInvalidCredentials`: JWT signature verification failed, token expired, or issuer mismatch
- `ErrInvalidTokenType`: token claims type is not supported
### Implemented
- [x] Policy specification: Defined in [POLICY.md](./POLICY.md)
- [x] Resource parser: `policy/resource.go` - Resource parsing and validation
- [x] Policy types: `policy/policy.go` - Policy, Statement, Effect types
- [x] Action types: `policy/action.go` - ActionDef registry and action group expansion
- [x] Variable interpolation: `policy/interpolate.go` - Template variable substitution
- [x] Action mapping: `policy/mapper.go` - Action+Resource to NATS permissions
- [x] Permissions: `policy/permissions.go` - Allow/Deny with wildcard deduplication
- [x] Compilation: `policy/compile.go` - `Compile()` function
- [x] Account provider: `provider/account_provider.go` - `AccountProvider` interface with `IsOperatorMode()`
- [x] Operator account provider: `provider/operator_account_provider.go` - Per-account signing keys for operator mode
- [x] Static account provider: `provider/static_account_provider.go` - Single key for all accounts
- [x] Policy provider: `provider/policy_provider.go` - `PolicyProvider` interface
- [x] File policy provider: `provider/file_policy_provider.go` - JSON file backend for policies
- [x] Identity provider: `identity/provider.go` - `AuthenticationProvider` interface
- [x] Provider manager: `identity/authentication_provider_manager.go` - Multi-provider routing with account patterns
- [x] File identity provider: `identity/file_authentication_provider.go` - Username/password with bcrypt
- [x] JWT identity provider: `identity/jwt_authentication_provider.go` - External JWT verification with role mapping
- [x] JWT issuance: `jwt/user.go` - `IssueUserJWT()` function
- [x] Signer: `jwt/signer.go` - `Signer` interface and `LocalSigner`
- [x] Auth controller: `auth/controller.go` - `AuthController` orchestrating full auth flow
- [x] Account scoped user: `auth/account_scoped_user.go` - Wrapper for filtering roles by account
- [x] Configuration: `auth/config.go` - `Config` types and `NewAuthControllerWithConfig()`
- [x] CLI: `cmd/nauts/main.go` - CLI with `auth` and `serve` subcommands (config file based)
- [x] NATS auth callout: `auth/callout.go` - `CalloutService` implementing auth callout protocol
- [x] E2E tests: Connection tests (`e2e/connection_test.go`) for static and operator modes
- [x] E2E tests: Policy tests (`e2e/policy_*_test.go`) for NATS, JetStream, and KV actions

### Future Work
- [ ] Explicit deny rules: Support `effect: "deny"` in policy statements
- [ ] Resource limits: Connection limits in policies (`maxSubscriptions`, `maxPayload`)
- [ ] System variables: `client.*` and `nats.*` interpolation for dynamic auth decisions
- [ ] JetStream domains: Support domain-specific API subjects (currently assumes default prefix)
- [ ] NATS KV provider: Dynamic configuration from NATS KV
- [ ] Control plane: Management API for policies, roles, and accounts
- [ ] Policy simulation API: Dry-run endpoint to test permissions

See also: [specs/](./specs/README.md) for detailed component specifications
