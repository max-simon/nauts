# CLAUDE.md

This file provides context to Claude Code for working on the nauts project.

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
│   ├── permissions.go      # NatsPermissions with Allow/Deny and wildcard dedup
│   ├── policy.go           # Policy, Statement, Effect types
│   └── resource.go         # Resource parsing and validation
├── provider/               # Account, role, and policy providers
│   ├── entity.go           # Account type with Signer
│   ├── account_provider.go # AccountProvider interface
│   ├── operator_account_provider.go # OperatorAccountProvider (operator mode with signing keys)
│   ├── static_account_provider.go # StaticAccountProvider (single key for all accounts)
│   ├── role_provider.go    # RoleProvider interface
│   ├── file_role_provider.go # FileRoleProvider (JSON file backend)
│   ├── policy_provider.go  # PolicyProvider interface
│   ├── file_policy_provider.go # FilePolicyProvider (JSON file backend)
│   ├── role.go             # Role type with validation
│   └── errors.go           # Provider errors (ErrNotFound, etc.)
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # UserIdentityProvider interface, IdentityToken
│   └── file_user_provider.go # FileUserIdentityProvider (bcrypt passwords)
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys-based signing)
│   └── user.go             # IssueUserJWT function
├── auth/                   # Authentication controller and callout service
│   ├── controller.go       # AuthController (orchestrates auth flow)
│   ├── callout.go          # CalloutService (NATS auth callout handler)
│   ├── config.go           # Config types and NewAuthControllerWithConfig
│   └── errors.go           # Auth errors (AuthError)
├── test/                   # Test fixtures and e2e tests
│   ├── e2e_test.go         # Go e2e test suite for both modes
│   ├── operator/           # Operator mode test environment
│   │   ├── nauts.json      # nauts configuration
│   │   ├── nats-server.conf# NATS server config with operator JWTs
│   │   ├── auth.creds      # Auth service credentials
│   │   ├── sentinel.creds  # Sentinel user credentials for client auth
│   │   └── *.nk            # Signing keys and xkey
│   ├── static/             # Static mode test environment
│   │   ├── nauts.json      # nauts configuration
│   │   ├── nats-server.conf# NATS server config with accounts
│   │   └── *.nk            # Account key and xkey
│   ├── users.json          # Test users (alice, bob)
│   ├── roles.json          # Test roles (readonly, full)
│   └── policies.json       # Test policies (read-access, write-access)
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
- Auth controller: `AuthController`, `NewAuthController()`, `NewAuthControllerWithConfig()`, `Authenticate()`, `ResolveUser()`, `ResolveNatsPermissions()`, `CreateUserJWT()`
- Callout service: `CalloutService`, `NewCalloutService()`, `CalloutConfig`, `Start()`, `Stop()`
- Configuration: `Config`, `LoadConfig()`, `AccountConfig`, `RoleConfig`, `PolicyConfig`, `IdentityConfig`, `ServerConfig`
- Permissions: `NatsPermissions`, `Permission`, `PermissionSet`
- Providers: `AccountProvider`, `OperatorAccountProvider`, `StaticAccountProvider`, `RoleProvider`, `FileRoleProvider`, `PolicyProvider`, `FilePolicyProvider`, `UserIdentityProvider`, `FileUserIdentityProvider`
- JWT: `Signer`, `LocalSigner`, `IssueUserJWT()`
- Entities: `Account`, `Role`, `User`

### Testing

- Table-driven tests for parsing and compilation
- Test files alongside implementation: `policy.go` + `policy_test.go`
- Use testdata/ for fixture files
- Aim for >80% coverage on core logic (policy, auth packages)

**E2E Tests** (`test/e2e_test.go`):
- Tests both static and operator modes
- Starts NATS server and nauts service automatically
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
# Token is JSON format: { "account"?: string, "token": string }
./bin/nauts auth -c nauts.json -token '{"token":"alice:secret"}'
# For multi-account users, specify the account:
./bin/nauts auth -c nauts.json -token '{"account":"APP","token":"alice:secret"}'

# Run auth callout service
./bin/nauts serve -c nauts.json

# Run e2e tests (from test/ directory)
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
  "role": {
    "type": "file",
    "file": {
      "path": "roles.json"
    }
  },
  "policy": {
    "type": "file",
    "file": {
      "path": "policies.json"
    }
  },
  "identity": {
    "type": "file",
    "file": {
      "usersPath": "users.json"
    }
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
{ "account": "APP", "token": "username:password" }
```

The `account` field is optional if the user has only one account configured. If the user has multiple accounts, the `account` field is required to specify which account to authenticate to.

**Static mode**: Clients connect with just a token:
```bash
nats --token '{"token":"alice:secret"}' pub test "hello"
# Or with account specified:
nats --token '{"account":"APP","token":"alice:secret"}' pub test "hello"
```

**Operator mode**: Clients must use sentinel credentials + token. The sentinel user authenticates to the AUTH account, which triggers auth callout with the provided token:
```bash
nats --creds sentinel.creds --token '{"token":"alice:secret"}' pub test "hello"
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

- Empty pub permissions → `Pub.Deny: [">"]`
- Empty sub permissions → `Sub.Deny: [">"]`
- Non-empty permissions → only `Allow` list is set (no `Deny`)

The `auth.AuthController` orchestrates the full authentication flow:
1. Verify identity token via `UserIdentityProvider` → returns `*identity.User`
2. Resolve user's roles (including default role) from `RoleProvider`
3. For each role, fetch both global and local roles, then compile policies with user/role context
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
// Token is JSON: { "account"?: string, "token": string }
result, err := controller.Authenticate(ctx, jwt.ConnectOptions{
    Token: `{"token":"alice:secret"}`,  // JSON token format
}, userPublicKey, time.Hour)
// result.User, result.Permissions, result.JWT available

// For multi-account users, specify the account:
result, err := controller.Authenticate(ctx, jwt.ConnectOptions{
    Token: `{"account":"CORP","token":"bob:secret"}`,
}, userPublicKey, time.Hour)
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

roleProvider, _ := provider.NewFileRoleProvider(provider.FileRoleProviderConfig{
    RolesPath: "roles.json",
})

policyProvider, _ := provider.NewFilePolicyProvider(provider.FilePolicyProviderConfig{
    PoliciesPath: "policies.json",
})

identityProvider, _ := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
    UsersPath: "users.json",
})

// Create controller
controller := auth.NewAuthController(accountProvider, roleProvider, policyProvider, identityProvider)

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
7. In operator mode, set `IssuerAccount` to the signing key's public key
8. Encrypt response with server's xkey (if provided)
9. Reply via `msg.Respond()`

**Error Handling**: Internal errors are logged but never leaked to clients. All authentication failures return generic "authentication failed" message.

### Identity Providers

Identity providers verify credentials and return user information.

**AuthRequest Type** (`identity/provider.go`):
The authentication token is expected to be a JSON object:
```go
type AuthRequest struct {
    Account string `json:"account,omitempty"` // Optional if user has single account
    Token   string `json:"token"`             // Provider-specific token (e.g., "user:pass")
}
```

**FileUserIdentityProvider** (`identity/`):
- Loads users from JSON file
- Verifies passwords using bcrypt
- Token format within AuthRequest: `"username:password"` (colon-separated)
- **Multi-account support**: Users can have multiple accounts
  - Single account: `account` field in AuthRequest is optional (user's account is used)
  - Multiple accounts: `account` field is required and validated against user's accounts list

**Users JSON file format**:
```json
{
  "users": {
    "alice": {
      "accounts": ["APP"],
      "roles": ["readonly"],
      "passwordHash": "$2a$10$...",
      "attributes": { "department": "engineering" }
    },
    "bob": {
      "accounts": ["APP", "CORP"],
      "roles": ["full"],
      "passwordHash": "$2a$10$..."
    }
  }
}
```

**Errors** (FileUserIdentityProvider):
- `ErrInvalidCredentials`: Password verification failed
- `ErrUserNotFound`: User does not exist
- `ErrInvalidTokenType`: Token format is wrong
- `ErrInvalidAccount`: Requested account is not valid for user
- `ErrAccountRequired`: User has multiple accounts but no account specified

**JwtUserIdentityProvider** (`identity/jwt_user_provider.go`):
Authenticates users using external JWTs (e.g., from Keycloak, Auth0, or other OIDC providers).

**How it works**:
1. **Decode JWT**: Parse the token to extract the issuer (`iss` claim)
2. **Verify signature**: Look up issuer's public key from configuration and verify JWT signature
3. **Extract roles**: Get roles from configurable claim path (default: `resource_access.nauts.roles`)
   - Roles must follow format `<account>.<role>` (e.g., `tenant-a.admin`, `app.viewer`)
   - Invalid formats are silently skipped
4. **Determine target account**:
   - If `AuthRequest.Account` is provided, use it directly
   - Otherwise, derive from roles: if all roles belong to the same account, use that account
   - If roles span multiple accounts without explicit account selection, return error
5. **Validate issuer permissions**: Check that the issuer is allowed to manage the target account
   - Configuration supports wildcards: `*` (any account), `tenant-*` (prefix match)
   - Wildcards in target account are rejected
6. **Filter and transform roles**:
   - Keep only roles belonging to the target account
   - Strip account prefix from role names (`tenant-a.admin` → `admin`)

**Configuration**:
```json
{
  "identity": {
    "type": "jwt",
    "jwt": {
      "issuers": {
        "https://auth.example.com/realms/myrealm": {
          "publicKey": "-----BEGIN PUBLIC KEY-----\nMIIBIjAN...\n-----END PUBLIC KEY-----",
          "accounts": ["tenant-a-*", "dev"]
        },
        "https://another-idp.com": {
          "publicKey": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----",
          "accounts": ["*"]
        }
      },
      "rolesClaimPath": "resource_access.nauts.roles"
    }
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
```

**Extracted user attributes**:
Standard JWT claims are extracted as user attributes:
- `email` → `attributes["email"]`
- `name` → `attributes["name"]`
- `preferred_username` → `attributes["preferred_username"]`

**Errors** (JwtUserIdentityProvider):
- `ErrInvalidCredentials`: JWT signature verification failed or token expired
- `ErrIssuerNotConfigured`: JWT issuer not found in configuration
- `ErrIssuerNotAllowed`: Issuer not allowed to manage target account
- `ErrNoRolesFound`: No valid roles found in JWT claims
- `ErrAmbiguousAccount`: Roles span multiple accounts and no account specified
- `ErrWildcardInAccount`: Target account contains wildcard characters

## Dependencies

```go
require (
    golang.org/x/crypto v0.x           // bcrypt for password hashing
    github.com/nats-io/jwt/v2 v2.x     // NATS JWT encoding/decoding
    github.com/nats-io/nkeys v0.x      // cryptographic signing
    github.com/nats-io/nats.go v1.x    // NATS client for auth callout
    github.com/golang-jwt/jwt/v5 v5.x  // External JWT verification
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
- [x] Account provider: `provider/account_provider.go` - `AccountProvider` interface with `IsOperatorMode()`
- [x] Operator account provider: `provider/operator_account_provider.go` - Per-account signing keys for operator mode
- [x] Static account provider: `provider/static_account_provider.go` - Single key for all accounts
- [x] Role provider: `provider/role_provider.go` - `RoleProvider` interface
- [x] File role provider: `provider/file_role_provider.go` - JSON file backend for roles
- [x] Policy provider: `provider/policy_provider.go` - `PolicyProvider` interface
- [x] File policy provider: `provider/file_policy_provider.go` - JSON file backend for policies
- [x] Identity provider: `identity/provider.go` - `UserIdentityProvider` interface
- [x] File identity provider: `identity/file_user_provider.go` - Username/password with bcrypt
- [x] JWT identity provider: `identity/jwt_user_provider.go` - External JWT verification with role mapping
- [x] JWT issuance: `jwt/user.go` - `IssueUserJWT()` function
- [x] Signer: `jwt/signer.go` - `Signer` interface and `LocalSigner`
- [x] Auth controller: `auth/controller.go` - `AuthController` orchestrating full auth flow
- [x] Configuration: `auth/config.go` - `Config` types and `NewAuthControllerWithConfig()`
- [x] CLI: `cmd/nauts/main.go` - CLI with `auth` and `serve` subcommands (config file based)
- [x] NATS auth callout: `auth/callout.go` - `CalloutService` implementing auth callout protocol
- [x] E2E tests: `test/e2e_test.go` - End-to-end tests for static and operator modes
- [ ] NATS KV provider: Future
- [ ] Control plane: Future
