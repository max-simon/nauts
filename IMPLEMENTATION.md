# Implementation Details

This document covers the architecture and implementation details of nauts.

## Package Structure

```
nauts/
├── cmd/
│   └── nauts/              # CLI entrypoint
│       └── main.go         # CLI with auth and serve subcommands
├── policy/                 # Policy types, compilation, interpolation
│   ├── action.go           # Action types and group expansion
│   ├── compile.go          # Compile() function
│   ├── context.go          # UserContext, RoleContext
│   ├── mapper.go           # Action+Resource to permissions
│   ├── permissions.go      # NatsPermissions with wildcard dedup
│   └── resource.go         # Resource parsing
├── provider/               # Account, role, and policy providers
│   ├── entity.go           # Account type with Signer
│   ├── account_provider.go # AccountProvider interface
│   ├── operator_account_provider.go # OperatorAccountProvider (operator mode)
│   ├── static_account_provider.go # StaticAccountProvider
│   ├── role_provider.go    # RoleProvider interface
│   ├── file_role_provider.go # FileRoleProvider
│   ├── policy_provider.go  # PolicyProvider interface
│   ├── file_policy_provider.go # FilePolicyProvider
│   └── role.go             # Role type
├── identity/               # User identity and authentication
│   ├── user.go             # User and AccountRole types
│   ├── provider.go         # AuthenticationProvider interface, AuthRequest
│   ├── file_user_provider.go # FileAuthenticationProvider
│   └── jwt_user_provider.go # JwtAuthenticationProvider
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys)
│   └── user.go             # IssueUserJWT()
├── auth/                   # Authentication controller and callout service
│   ├── controller.go       # AuthController
│   ├── callout.go          # CalloutService (NATS auth callout)
│   ├── config.go           # Config, LoadConfig, NewAuthControllerWithConfig
│   └── errors.go           # AuthError
└── test/                   # Test fixtures and environments
```

## Components Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              AuthController                                  │
│                                                                             │
│  ┌─────────────┐    ┌──────────────────────┐    ┌─────────────────────┐    │
│  │ ResolveUser │───▶│ ResolveNatsPermissions│───▶│   CreateUserJWT     │    │
│  └──────┬──────┘    └──────────┬───────────┘    └──────────┬──────────┘    │
│         │                      │                           │               │
└─────────┼──────────────────────┼───────────────────────────┼───────────────┘
          │                      │                           │
          ▼                      ▼                           ▼
┌─────────────────┐    ┌─────────────────┐         ┌─────────────────┐
│    identity/    │    │    provider/    │         │      jwt/       │
│                 │    │                 │         │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │         │ ┌─────────────┐ │
│ │    User     │ │    │ │RoleProvider │ │         │ │   Signer    │ │
│ │IdentityProv│ │    │ │PolicyProvider│ │         │ │ IssueUserJWT│ │
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
| `provider/` | NATS account management, policy storage, role storage |
| `identity/` | User authentication and identity resolution |
| `jwt/` | NATS JWT creation and signing |
| `auth/` | Authentication orchestration and NATS auth callout service |

## Authentication Flow

The `AuthController.Authenticate()` method performs:

1. **Parse auth request**: Extract account, token, and optional provider ID from JSON `{"account":"APP","token":"...","ap":"provider-id"}`
2. **Select provider**: Find authentication provider(s) that can manage the account. If multiple match, use provider ID from request.
3. **Verify identity token**: Authentication provider verifies the token and returns user info with all roles across all accounts
4. **Filter roles**: Filter user roles to only include those for the target account
5. **Resolve permissions**: For each role, fetch both global and local role definitions and compile policies
6. **Create JWT**: Sign a NATS user JWT with the compiled permissions
7. **Return result**: `AuthResult` containing user, account, permissions, and signed JWT

```go
// Using configuration file
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)

// Authenticate
result, err := controller.Authenticate(ctx, jwt.ConnectOptions{
    Token: `{"token":"alice:secret"}`,
}, userPublicKey, time.Hour)
// result.User, result.Permissions, result.JWT
```

## Permission Compilation

The `policy.Compile()` function transforms policies to NATS permissions:

1. Expand action groups to atomic actions
2. Interpolate variables in resources (e.g., `{{ user.id }}`)
3. Parse and validate resources
4. Map actions + resources to NATS permissions
5. Add `_INBOX.>` for actions that require it (JetStream, KV)
6. Merge into result permissions
7. Deduplicate using wildcard-aware logic

### Wildcard-Aware Deduplication

`NatsPermissions.Deduplicate()` removes redundant subjects:
- `foo.bar` covered by `foo.*` → remove `foo.bar`
- `foo.bar` covered by `foo.>` → remove `foo.bar`
- `foo.*` covered by `foo.>` → remove `foo.*`

## JWT Permission Encoding

NATS JWT defaults to allowing everything when no permissions are specified. nauts handles this by explicitly denying all when no allow permissions are granted:

- Empty pub permissions → `Pub.Deny: [">"]` (user cannot publish)
- Empty sub permissions → `Sub.Deny: [">"]` (user cannot subscribe)
- Non-empty permissions → only `Allow` list is set

This ensures the principle of least privilege.

## Role System

Roles replace the older "groups" concept. Each role has:

```go
type Role struct {
    Name     string   `json:"name"`     // Part of unique key
    Account  string   `json:"account"`  // "*" for global, specific account for local
    Policies []string `json:"policies"` // Policy IDs
}
```

- **Global roles** (`account: "*"`): Apply to all accounts
- **Local roles** (specific account): Apply only to that account
- **Composite key**: `(Name, Account)` is unique

When resolving permissions, both global and account-specific roles are considered:
1. Look up `roles[name:*]` (global role)
2. Look up `roles[name:account]` (local role)
3. Merge policies from both

### User Roles vs Role Definitions

Important distinction:
- **User.Roles** (`[]AccountRole`): Concrete role assignments with account associations (e.g., `{Account: "APP", Role: "admin"}`)
- **Role** (from RoleProvider): Role definitions that can be global (`Account: "*"`) or local (concrete account)
- User roles always have concrete accounts (never `*`)
- Role definitions from RoleProvider can have `Account: "*"` for global roles
    Name     string   `json:"name"`     // Part of unique key
    Account  string   `json:"account"`  // "*" for global, specific account for local
    Policies []string `json:"policies"` // Policy IDs
}
```

- **Global roles** (`account: "*"`): Apply to all accounts
- **Local roles** (specific account): Apply only to that account
- **Composite key**: `(Name, Account)` is unique

When resolving permissions, both global and account-specific roles are considered:
1. Look up `roles[name:*]` (global role)
2. Look up `roles[name:account]` (local role)
3. Merge policies from both

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

**Configuration**:
```json
{
  "authentication": {
    "file": [
      {
        "id": "local-users",
        "accounts": ["*"],
        "usersPath": "users.json"
      }
    ]
  }
}
```

**User data format**: Roles use `<account>.<role>` format with concrete accounts:
```json
{
  "users": {
    "alice": {
      "roles": ["APP.readonly", "APP.viewer"],
      "passwordHash": "$2a$10$...",
      "attributes": { "email": "alice@example.com" }
    }
  }
}
```

The `account` field in the token is required. The provider returns a User with all roles, which AuthController filters for the target account.

### JwtAuthenticationProvider

Verify JWTs from external identity providers (Keycloak, Auth0, etc.).

**Token format**: `{"account":"APP","token":"<external-jwt>"}`

**Configuration**:
```json
{
  "authentication": {
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
1. Parse JWT and verify signature using provider's public key (RSA or ECDSA)
2. Verify issuer matches provider's configured issuer
3. Extract roles from claims at provider's configured path (default: `resource_access.nauts.roles`)
4. Parse roles: format is `<account>.<role>` (e.g., `tenant-a.admin`) with concrete accounts
5. Return User with all roles (no filtering at this stage)
6. AuthController filters roles for target account

**Provider Selection**:
- Each provider declares which accounts it can manage via `CanManageAccount(account)` method
- Supports wildcards: `*` matches any, `tenant-*` matches prefix
- Special accounts `SYS` and `AUTH` require exact match
- If multiple providers match an account, token must include `"ap": "provider-id"` field

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

### auth Subcommand

Authenticate a user and output a signed NATS JWT.

```bash
./bin/nauts auth [options]

Options:
  -c, --config string    Path to configuration file (required)
  -token string          Token to authenticate (JSON format, required)
  -user-pubkey string    User's public key for JWT subject (optional)
  -ttl duration          JWT time-to-live (default 1h)

Environment variables:
  NAUTS_CONFIG    Path to configuration file
```

### serve Subcommand

Run the NATS auth callout service.

```bash
./bin/nauts serve [options]

Options:
  -c, --config string    Path to configuration file (required)

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
├── roles.json          # Shared roles
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
| alice | `{"token":"alice:secret"}` | readonly | APP | Subscribe to `public.>` |
| bob | `{"token":"bob:secret"}` | full | APP | Pub/Sub to `public.>` |

## Future Enhancements

- **Explicit deny rules**: Support `effect: "deny"` with evaluation order
- **Resource limits**: Connection limits in policies (`maxSubscriptions`, `maxPayload`)
- **Policy simulation API**: Dry-run endpoint to test permissions
- **Per-user inbox scoping**: Replace `_INBOX.>` with user-specific prefixes
- **NATS KV Role/Policy Provider**: Dynamic configuration from NATS KV
- **Control Plane**: Management API for policies, roles, and accounts
