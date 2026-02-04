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
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # UserIdentityProvider interface, AuthRequest
│   ├── file_user_provider.go # FileUserIdentityProvider
│   └── jwt_user_provider.go # JwtUserIdentityProvider
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

1. **Parse auth request**: Extract account and token from JSON `{"account":"APP","token":"..."}`
2. **Verify identity token**: Identity provider verifies the token and returns user info
3. **Resolve roles**: Collect user's roles (including default role)
4. **Compile permissions**: For each role, fetch policies and compile to NATS permissions
5. **Create JWT**: Sign a NATS user JWT with the compiled permissions
6. **Return result**: `AuthResult` containing user, permissions, and signed JWT

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

## Identity Providers

### FileUserIdentityProvider

Static user list with bcrypt password hashes.

**Token format**: `{"account":"APP","token":"username:password"}`

```json
{
  "users": {
    "alice": {
      "accounts": ["APP"],
      "roles": ["readonly"],
      "passwordHash": "$2a$10$...",
      "attributes": { "department": "engineering" }
    }
  }
}
```

- If user has single account, `account` in request is optional
- If user has multiple accounts, `account` must be specified

### JwtUserIdentityProvider

Verify JWTs from external identity providers (Keycloak, Auth0, etc.).

**Token format**: `{"account":"APP","token":"<external-jwt>"}`

**Configuration**:
```json
{
  "identity": {
    "type": "jwt",
    "jwt": {
      "issuers": {
        "https://keycloak.example.com/realms/myrealm": {
          "publicKey": "-----BEGIN PUBLIC KEY-----\n...",
          "accounts": ["tenant-*", "shared"]
        }
      },
      "rolesClaimPath": "resource_access.nauts.roles"
    }
  }
}
```

**Verification process**:
1. Parse JWT to extract issuer (iss claim)
2. Look up issuer configuration
3. Verify signature using issuer's public key (RSA or ECDSA)
4. Extract roles from claims at configured path (default: `resource_access.nauts.roles`)
5. Parse roles: format is `<account>.<role>` (e.g., `tenant-a.admin`)
6. Determine target account from request or derive from roles
7. Validate issuer can manage target account (supports wildcards)
8. Filter roles for target account and strip account prefix

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
