# Implementation Details

This document covers the architecture and implementation details of nauts.

## Package Structure

```
nauts/
├── cmd/
│   └── nauts/              # CLI entrypoint
│       └── main.go         # CLI for service (optional debug flag)
├── policy/                 # Policy types, compilation, interpolation
│   ├── action.go           # Action types and group expansion
│   ├── compile.go          # Compile() function
│   ├── context.go          # PolicyContext
│   ├── mapper.go           # Action+Resource to permissions
│   ├── permissions.go      # NatsPermissions with wildcard dedup
│   └── resource.go         # Resource parsing
├── provider/               # Account, role, and policy providers
│   ├── entity.go           # Account type with Signer
│   ├── account_provider.go # AccountProvider interface
│   ├── operator_account_provider.go # OperatorAccountProvider (operator mode)
│   ├── static_account_provider.go # StaticAccountProvider
│   ├── policy_provider.go  # PolicyProvider interface
│   ├── file_policy_provider.go # FilePolicyProvider
│   └── errors.go           # Provider errors
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # AuthenticationProvider interface, AuthRequest
│   ├── file_authentication_provider.go # FileAuthenticationProvider
│   └── jwt_authentication_provider.go # JwtAuthenticationProvider
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys)
│   └── user.go             # IssueUserJWT()
├── auth/                   # Authentication controller and callout service
│   ├── controller.go       # AuthController
│   ├── callout.go          # CalloutService (NATS auth callout)
│   ├── debug.go            # DebugService (permission compilation)
│   ├── config.go           # Config, LoadConfig, NewAuthControllerWithConfig
│   └── errors.go           # AuthError
└── e2e/                    # End-to-end tests
    ├── policy-static/      # Policy engine test setup
    ├── common/             # Shared test resources
    ├── env.go              # Test harness
    ├── policy_*_test.go    # Policy tests (NATS, JetStream, KV)
    └── connection_test.go  # Legacy connection tests
```

## Components Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              AuthController                                  │
│                                                                             │
│  ┌──────────────────┐   ┌──────────────────────┐   ┌─────────────────────┐ │
│  │ ScopeUserToAccount│──▶│ CompileNatsPermissions│──▶│   CreateUserJWT     │ │
│  └─────────┬────────┘   └──────────┬───────────┘   └──────────┬──────────┘ │
│         │                      │                           │               │
└─────────┼──────────────────────┼───────────────────────────┼───────────────┘
          │                      │                           │
          ▼                      ▼                           ▼
┌─────────────────┐    ┌─────────────────┐         ┌─────────────────┐
│    identity/    │    │    provider/    │         │      jwt/       │
│                 │    │                 │         │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │         │ ┌─────────────┐ │
│ │    User     │ │    │ │PolicyProvider│ │         │ │   Signer    │ │
│ │IdentityProv│ │    │ │ (roles+pols) │ │         │ │ IssueUserJWT│ │
│ └─────────────┘ │    │ └─────────────┘ │         │ └─────────────┘ │
└─────────────────┘    │ ┌─────────────┐ │         └────────┬────────┘
                       │ │AccountProvider│                  │
                       │ │   (Account)  │◀─────────────────┘
                       │ └─────────────┘ │
                       └─────────────────┘
                                │
                                ▼
                       ┌─────────────────┐
                       │     policy/     │
                       │                 │
                       │ ┌─────────────┐ │
                       │ │   Compile   │ │
                       │ │NatsPermissions│
                       │ └─────────────┘ │
                       └─────────────────┘
```

## Component Responsibilities

| Package | Responsibility |
|---------|---------------|
| `policy/` | Policy specification, compilation, variable interpolation, action mapping |
| `provider/` | NATS account management, policy storage, role→policy mapping |
| `identity/` | User authentication and identity resolution |
| `jwt/` | NATS JWT creation and signing |
| `auth/` | Authentication orchestration and NATS auth callout service |

## Authentication Flow

The `AuthController.Authenticate()` method performs:

1. **Parse auth request**: Extract account and token from JSON `{"account":"APP","token":"..."}`
2. **Select provider**: Choose an auth provider via `AuthenticationProviderManager`
3. **Verify identity token**: Provider verifies the token and returns user info
4. **Scope user**: Filter roles to requested account, validate no wildcards
5. **Compile permissions**: For each role, fetch policies and compile to NATS permissions
6. **Create JWT**: Sign a NATS user JWT with the compiled permissions
7. **Return result**: `AuthResult` containing user, compilation result, and signed JWT

```go
// Using configuration file
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)

// Authenticate
result, err := controller.Authenticate(ctx, natsjwt.ConnectOptions{
  Token: `{"account":"APP","token":"alice:secret"}`,
}, userPublicKey, time.Hour)
// result.User, result.CompilationResult, result.JWT
```

## Permission Compilation

The `policy.Compile()` function transforms policies to NATS permissions:

1. Expand action groups to atomic actions
2. Interpolate variables in resources (e.g., `{{ user.id }}`)
3. Parse and validate resources
4. Map actions + resources to NATS permissions
5. Add `_INBOX_{{ user.id }}.>` subscription for all users
6. Merge into result permissions
7. Deduplicate using wildcard-aware logic

### Wildcard-Aware Deduplication

`NatsPermissions.Deduplicate()` removes redundant subjects:
- `foo.bar` covered by `foo.*` → remove `foo.bar`
- `foo.bar` covered by `foo.>` → remove `foo.bar`
- `foo.*` covered by `foo.>` → remove `foo.*`
- Queue subscriptions are handled separately unless covered by a broader subscription without queue.

### JSON Encoding of Permissions

`PermissionSet` marshals to JSON as an object with an `allow` array. `NatsPermissions` uses
`pub` and `sub` fields containing these arrays, so the type is JSON-serializable for debug output.

## JWT Permission Encoding

NATS JWT defaults to empowering everything when no permissions are specified. nauts handles this by explicitly denying all when no allow permissions are granted:

- Empty pub permissions → `Pub.Deny: [">"]` (user cannot publish)
- Empty sub permissions → `Sub.Deny: [">"]` (user cannot subscribe)
- Non-empty permissions → only `Allow` list is set
- Queue subscriptions are merged into the main `Allow` list as NATS JWTs do not support specific queue restrictions.

This ensures the principle of least privilege.

## Debug Service

The debug service listens on `nauts.debug` and accepts a plain JSON payload:

```json
{
  "user": {"id": "alice", "roles": [{"account": "APP", "name": "workers"}]},
  "account": "APP"
}
```

It scopes the user to the account, compiles permissions via `CompileNatsPermissions`,
and returns a JSON response containing `NautsCompilationResult`.
The service uses `ServerConfig` for NATS connectivity (credentials or nkey) and ignores
`xkeySeedFile`.

## Role Bindings

Role bindings replace the older "groups" concept. Each binding maps a role name to a set of policy ids for a specific account:

```go
type Binding struct {
  Role     string   `json:"role"`     // Part of unique key
  Account  string   `json:"account"`  // Account id (e.g. "APP")
  Policies []string `json:"policies"` // Policy ids
}
```

- **No global bindings**: bindings are resolved by exact `(account, role)` match
- **Composite key**: `(Account, Role)` is unique (often represented as `account.role`)

## Account Providers

### Operator Mode (OperatorAccountProvider)

For NATS deployments with operator/account hierarchy:
- Auth service runs in AUTH account
- Authenticates users for all configured accounts
- Each account has its own signing key
- `IsOperatorMode()` returns `true`

```json
{
  "account": {
    "type": "operator",
    "operator": {
      "accounts": {
        "AUTH": { "publicKey": "AAUTH...", "signingKeyPath": "auth-signing.nk" },
        "APP": { "publicKey": "AAPP...", "signingKeyPath": "app-signing.nk" }
      }
    }
  }
}
```

In operator mode:
- Auth callout response includes `IssuerAccount` set to the signing key's public key
- User JWTs don't include audience (account determined by `IssuerAccount`)

### Static Mode (StaticAccountProvider)

Simpler setup with single signing key for all accounts:
- `IsOperatorMode()` returns `false`
- User JWTs include audience set to account name

```json
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "AXXXXX...",
      "privateKeyPath": "account.nk",
      "accounts": ["AUTH", "APP"]
    }
  }
}
```

## Authentication Providers

### FileAuthenticationProvider

Static user list with bcrypt password hashes.

**Token format**: `{"account":"APP","token":"username:password"}`

**Optional provider selection**: `{"account":"APP","token":"username:password","ap":"local"}`

```json
{
  "users": {
    "alice": {
      "accounts": ["APP"],
      "roles": ["APP.readonly"],
      "passwordHash": "$2a$10$...",
      "attributes": { "department": "engineering" }
    }
  }
}
```

`account` is required in the request.

### JwtAuthenticationProvider

Verify JWTs from external identity providers (Keycloak, Auth0, etc.).

**Token format**: `{"account":"APP","token":"<external-jwt>"}`

**Optional provider selection**: `{"account":"APP","token":"<external-jwt>","ap":"keycloak"}`

**Configuration**:
```json
{
  "auth": {
    "jwt": [
      {
        "id": "keycloak",
        "accounts": ["tenant-*", "shared"],
        "issuer": "https://keycloak.example.com/realms/myrealm",
        "publicKey": "<base64 encoded PEM public key>",
        "rolesClaimPath": "resource_access.nauts.roles"
      }
    ]
  }
}
```

**Verification process**:
1. Verify signature using the provider's configured public key (RSA or ECDSA)
2. Validate issuer (iss claim) matches the provider's configured issuer
3. Extract roles from claims at the provider's configured path (default: `resource_access.nauts.roles`)
4. Parse roles: format is `<account>.<role>` (e.g., `tenant-a.admin`)
5. Return all roles; authorization later filters roles by the requested account

**Account wildcards**:
- `*` matches any account
- `tenant-*` matches accounts starting with `tenant-`
- `shared` matches only `shared`

## Auth Callout Service

The `CalloutService` implements the NATS auth callout protocol:

```go
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)
calloutConfig, _ := config.Server.ToCalloutConfig()
service, _ := auth.NewCalloutService(controller, calloutConfig)
service.Start(ctx)
```

**Protocol flow**:
1. Subscribe to `$SYS.REQ.USER.AUTH`
2. Decrypt request using service's xkey (if configured)
3. Decode `jwt.AuthorizationRequestClaims`
4. Extract token from `ConnectOptions`
5. Call `AuthController.Authenticate()`
6. Build `jwt.AuthorizationResponseClaims` with user JWT
7. In operator mode, set `IssuerAccount`
8. Encrypt response with server's xkey (if provided)
9. Reply via `msg.Respond()`

**NATS Server Configuration**:
```
accounts {
  AUTH { users: [ { nkey: UAXXXXX... } ] }
  APP {}
}

authorization {
  auth_callout {
    issuer: AXXXXX...
    account: AUTH
    xkey: XAXXXXX...
  }
}
```

## CLI Reference

Run the NATS auth callout service (optionally with debug service).

```bash
./bin/nauts [options]

Options:
  -c, --config string       Path to configuration file (required)
  --enable-debug-svc        Start the NATS auth debug service

Environment variables:
  NAUTS_CONFIG    Path to configuration file
```

## Configuration Reference

### Complete Example (Operator Mode)

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

### Server Configuration Options

| Field | Description |
|-------|-------------|
| `natsUrl` | NATS server URL |
| `natsCredentials` | Path to NATS credentials file (mutually exclusive with natsNkey) |
| `natsNkey` | Path to nkey seed file (mutually exclusive with natsCredentials) |
| `xkeySeedFile` | Path to file containing XKey seed for encrypted auth callout |
| `ttl` | JWT time-to-live (e.g., "1h", "30m") |

## Test Environments

Pre-configured environments in `test/`:

```
test/
├── e2e_test.go         # Go e2e test suite
├── policies.json       # Shared policies
├── bindings.json       # Shared role bindings
├── users.json          # Shared users
├── operator/           # Operator mode setup
│   ├── nauts.json
│   ├── nats-server.conf
│   └── *.nk
└── static/             # Static mode setup
    ├── nauts.json
    ├── nats-server.conf
    └── *.nk
```

**Test users**:
| User | Token | Roles | Account | Permissions |
|------|-------|-------|---------|-------------|
| alice | `{"account":"APP","token":"alice:secret"}` | readonly | APP | Subscribe to `public.>` |
| bob | `{"account":"APP","token":"bob:secret"}` | full | APP | Pub/Sub to `public.>` |

## Future Enhancements

- **Explicit deny rules**: Support `effect: "deny"` with evaluation order
- **Resource limits**: Connection limits in policies (`maxSubscriptions`, `maxPayload`)
- **Policy simulation API**: Dry-run endpoint to test permissions
- **Per-user inbox scoping**: Replace `_INBOX.>` with user-specific prefixes
- **NATS KV Role/Policy Provider**: Dynamic configuration from NATS KV
- **Control Plane**: Management API for policies, roles, and accounts
