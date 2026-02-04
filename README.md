<div style="display: flex; align-items: center;">
  <div style="flex-shrink: 0;">
    <img src="./docs/logo.png" alt="Logo" style="height: 100px; display: block; width: 100px">
  </div>
  <div style="margin-left: 20px">
    <h1>NAUTS</h1>
    <b>N</b>ATS <b>Aut</b>hentication <b>S</b>ervice
  </div>
</div>

## TL;DR

nauts simplifies permission and token management for NATS by granting NATS permissions to external users using access human-friendly policies. It contains the following components:
- _permission compiler_: nauts uses policies as a scalable abstraction of low-level NATS permissions and provides a compiler to map them to NATS permissions
- _authentication service_ for [NATS auth callout](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout) making use of nauts policies.
- _control plane_ to manage policies, groups and accounts within NATS.

## Policy Specification

nauts policies are specified in [POLICY.md](./POLICY.md).

Policies are not attached to users directly. Instead they are assigned to user `groups`. The users of a group inherit NATS permissions via the attached policies. A user can be part of multiple groups and inherits permissions from all its groups. Permissions from all groups and policies are merged (union of all allowed permissions).

```typescript
interface Group {
    id: str              // unique identifier
    name: str            // human-readable name
    policies: list[str]  // policy ids of attached policies
}
```

Groups and policies are scoped to a specific NATS account. A user's account is determined by the identity resolver during authentication.

Each NATS account has a default group with id `default`. All users are members of this group, allowing default permissions to be granted to all users of an account.

### Permission Compiler

nauts provides a compiler to map policies to NATS permissions for a given user identity.

The compilation process:
1. Resolve user's groups (including the default group)
2. For each group, fetch policies from the provider
3. For each policy, expand action groups to atomic actions
4. Interpolate variables in resources (e.g., `{{ user.id }}`)
5. Map actions + resources to NATS PUB/SUB permissions
6. Add implicit permissions (`_INBOX.>` for JetStream/KV actions)
7. Deduplicate permissions using wildcard-aware logic
8. Return final `NatsPermissions`

#### Group/Policy Provider

A provider supplies policies and groups for a NATS account. The following provider types are available:

##### File-based

Policies and groups are read from JSON files. The data is only read once during initialization, afterwards it is kept in memory.

```typescript
// Policy file (array of policies)
type Policies = Policy[]

// Group file (array of groups)
type Groups = Group[]
```

##### NATS KV

> Not implemented yet

Policies and groups are read from NATS Key-Value store. The bucket name defaults to `NAUTS` and can be configured. Policies are stored with key `policy.<account id>.<policy id>`. Groups are stored with key `group.<account id>.<group id>`. nauts watches changes on the bucket, so policies and groups can be modified without restart.

Policy and group updates are incorporated on a best-effort basis (eventual consistency).

## Authentication Service

nauts provides an `AuthController` that orchestrates the complete authentication flow. It can be used as a building block for implementing a [NATS auth callout service](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout).

### Authentication Flow

The `AuthController.Authenticate()` method performs the following steps:

1. _Verify identity token_: The identity provider verifies the token and returns user information (user ID, account, groups, attributes).
2. _Compile permissions_: Compiles NATS permissions for the verified user based on their group memberships.
3. _Create JWT_: Signs a NATS user JWT with the compiled permissions using the account's signing key.
4. _Return result_: Returns `AuthResult` containing the user, permissions, and signed JWT.

```go
// Using configuration file (recommended)
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)

// Or setup providers manually (operator mode)
entityProvider, _ := provider.NewOperatorEntityProvider(provider.OperatorEntityProviderConfig{
    Accounts: map[string]provider.AccountSigningConfig{
        "AUTH": {PublicKey: "AAUTH...", SigningKeyPath: "/path/to/auth-signing.nk"},
        "APP":  {PublicKey: "AAPP...", SigningKeyPath: "/path/to/app-signing.nk"},
    },
})
nautsProvider, _ := provider.NewFileNautsProvider(provider.FileNautsProviderConfig{
    PoliciesPath: "policies.json",
    GroupsPath:   "groups.json",
})
identityProvider, _ := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
    UsersPath: "users.json",
})
controller := auth.NewAuthController(entityProvider, nautsProvider, identityProvider)

// Authenticate
result, err := controller.Authenticate(ctx, identity.UsernamePassword{
    Username: "alice",
    Password: "secret",
}, userPublicKey, time.Hour)
// result.User contains user info
// result.Permissions contains compiled NATS permissions
// result.JWT contains signed NATS user JWT
```

### Auth Callout Service

nauts provides a `CalloutService` that implements the [NATS auth callout protocol](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout). The service subscribes to `$SYS.REQ.USER.AUTH` and handles authentication requests from NATS servers.

```go
// Using configuration file (recommended)
config, _ := auth.LoadConfig("nauts.json")
controller, _ := auth.NewAuthControllerWithConfig(config)
calloutConfig, _ := config.Server.ToCalloutConfig()
service, _ := auth.NewCalloutService(controller, calloutConfig)

// Or configure manually
service, _ := auth.NewCalloutService(controller, auth.CalloutConfig{
    NatsURL:    "nats://localhost:4222",
    NatsNkey:   "auth-service.nk",  // Path to nkey seed file
    XKeySeed:   xkeySeed,           // Optional, for encrypted auth callout (read from file)
    DefaultTTL: time.Hour,
})

// Start service (blocks until shutdown)
ctx, cancel := context.WithCancel(context.Background())
service.Start(ctx)
```

**Protocol Flow**:
1. Receive message on `$SYS.REQ.USER.AUTH`
2. Decrypt request using service's xkey (if configured)
3. Decode `jwt.AuthorizationRequestClaims`
4. Extract username/password from `ConnectOptions`
5. Call `AuthController.Authenticate()` with credentials and user's nkey
6. Build `jwt.AuthorizationResponseClaims` with signed user JWT
7. In operator mode, set `IssuerAccount` to the signing key's public key
8. Encrypt response with server's xkey (from `Nats-Server-Xkey` header)
9. Reply via `msg.Respond()`

**NATS Server Configuration**:

```
accounts {
  AUTH { users: [ { nkey: UAXXXXX... } ] }  # nkey public key from auth-service.nk
  APP {}
}

authorization {
  auth_callout {
    issuer: AXXXXX...  # Account public key that issues user JWTs
    account: AUTH
    xkey: XAXXXXX...   # Public key matching nauts --xkey-seed
  }
}
```

### User Identity Provider

nauts uses identity providers to verify credentials and resolve user information. The following providers are available:

#### File-based (FileUserIdentityProvider)

Static list of users and group assignments stored in a JSON file. Verification is based on username and password check using bcrypt.

```typescript
interface User {
    account: str       // NATS account id
    groups: list[str]  // list of group ids the user belongs to
    passwordHash: str  // bcrypt hash of password
    [attr: str]: str   // additional user attributes, available to policies
}

interface UserList {
    users: {
        [userId: str]: User
    }
}
```

#### JWT-based

> Not implemented yet

### Entity Provider

Entity providers supply NATS account information and signing keys for JWT issuance. nauts supports two entity provider modes:

#### Operator Mode (OperatorEntityProvider)

For NATS deployments with operator/account hierarchy where the auth service runs in the AUTH account but authenticates users for all accounts. Each account has its own signing key.

```json
{
  "entity": {
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
  }
}
```

| Field | Description |
|-------|-------------|
| `accounts` | Map of account names to their signing configuration |
| `publicKey` | The account's public key (starts with 'A') |
| `signingKeyPath` | Path to the account signing key file (.nk file) |

In operator mode:
- Auth callout response includes `IssuerAccount` set to the signing key's public key
- User JWTs don't include audience (account determined by `IssuerAccount`)
- `IsOperatorMode()` returns `true`

#### Static Mode (StaticEntityProvider)

A simplified provider that uses a single signing key for all accounts. Useful for development and simple deployments where all accounts share the same credentials.

```json
{
  "entity": {
    "type": "static",
    "static": {
      "publicKey": "AXXXXX...",
      "privateKeyPath": "/path/to/account.nk",
      "accounts": ["AUTH", "APP"]
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `publicKey` | The account public key (used for all accounts) |
| `privateKeyPath` | Path to the nkey seed file (used to sign JWTs for all accounts) |
| `accounts` | List of account names that this provider serves |

In static mode:
- User JWTs include audience set to account name
- `IsOperatorMode()` returns `false`

## Package Structure

```
nauts/
├── cmd/
│   └── nauts/              # CLI entrypoint
│       └── main.go         # CLI with auth and serve subcommands
├── policy/                 # Policy types, compilation, interpolation
│   ├── action.go           # Action types and group expansion
│   ├── compile.go          # Compile() function
│   ├── context.go          # UserContext, GroupContext
│   ├── mapper.go           # Action+Resource to permissions
│   ├── permissions.go      # NatsPermissions with wildcard dedup
│   └── resource.go         # Resource parsing
├── provider/               # Entity and policy/group providers
│   ├── entity.go           # Operator, Account types
│   ├── entity_provider.go  # EntityProvider interface
│   ├── operator_entity_provider.go # OperatorEntityProvider (operator mode)
│   ├── static_entity_provider.go # StaticEntityProvider
│   ├── nauts_provider.go   # NautsProvider interface
│   ├── file_nauts_provider.go # FileNautsProvider
│   └── group.go            # Group type
├── identity/               # User identity management
│   ├── user.go             # User type
│   ├── provider.go         # UserIdentityProvider interface
│   └── file_user_provider.go # FileUserIdentityProvider
├── jwt/                    # JWT issuance
│   ├── signer.go           # Signer interface
│   ├── local_signer.go     # LocalSigner (nkeys)
│   └── user.go             # IssueUserJWT()
├── auth/                   # Authentication controller and callout service
│   ├── controller.go       # AuthController
│   ├── callout.go          # CalloutService (NATS auth callout)
│   ├── config.go           # Config, LoadConfig, NewAuthControllerWithConfig
│   └── errors.go           # AuthError
└── testdata/               # Test fixtures
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
│ │    User     │ │    │ │    Group    │ │         │ │   Signer    │ │
│ │IdentityProv│ │    │ │ NautsProvider│ │         │ │ IssueUserJWT│ │
│ └─────────────┘ │    │ └─────────────┘ │         │ └─────────────┘ │
└─────────────────┘    │ ┌─────────────┐ │         └────────┬────────┘
                       │ │EntityProvider│ │                  │
                       │ │  (Operator,  │ │                  │
                       │ │   Account)   │◀─────────────────┘
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

### Component Responsibilities

| Package | Responsibility |
|---------|---------------|
| `policy/` | Policy specification, compilation, variable interpolation, action mapping |
| `provider/` | NATS entity management (operators, accounts), policy/group storage |
| `identity/` | User authentication and identity resolution |
| `jwt/` | NATS JWT creation and signing |
| `auth/` | Authentication orchestration and NATS auth callout service |

## CLI

nauts provides a command-line interface with two subcommands:
- `auth` - One-shot authentication to generate a NATS JWT
- `serve` - Run the NATS auth callout service

Both commands use a JSON configuration file specified via `-c/--config` flag or `NAUTS_CONFIG` environment variable.

### Installation

```bash
go build -o bin/nauts ./cmd/nauts
```

### Configuration File

The CLI uses a JSON configuration file that specifies all providers and server settings:

**Operator mode** (recommended for production):

```json
{
  "entity": {
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
  "nauts": {
    "type": "file",
    "file": {
      "policiesPath": "policies.json",
      "groupsPath": "groups.json"
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

**Static mode** (simpler setup for development):

```json
{
  "entity": {
    "type": "static",
    "static": {
      "publicKey": "AXXXXX...",
      "privateKeyPath": "/path/to/account.nk",
      "accounts": ["AUTH", "APP"]
    }
  },
  "nauts": { ... },
  "identity": { ... },
  "server": { ... }
}
```

#### Configuration Sections

| Section | Description |
|---------|-------------|
| `entity` | Entity provider configuration (operator/account info) |
| `nauts` | Nauts provider configuration (policies/groups) |
| `identity` | Identity provider configuration (user credentials) |
| `server` | Server settings for auth callout service (only needed for `serve`) |

#### Server Configuration Options

| Field | Description |
|-------|-------------|
| `natsUrl` | NATS server URL |
| `natsCredentials` | Path to NATS credentials file (mutually exclusive with natsNkey) |
| `natsNkey` | Path to nkey seed file for NATS authentication (mutually exclusive with natsCredentials) |
| `xkeySeedFile` | Path to file containing XKey seed for encrypted auth callout |
| `ttl` | JWT time-to-live (e.g., "1h", "30m") |

### auth Subcommand

Authenticate a user and output a signed NATS JWT.

```bash
./bin/nauts auth [options]

Options:
  -c, --config string    Path to configuration file (required)
  -username string       Username to authenticate (required)
  -password string       Password for authentication (required)
  -user-pubkey string    User's public key for JWT subject (optional)
  -ttl duration          JWT time-to-live (default 1h, overrides config)

Environment variables:
  NAUTS_CONFIG    Path to configuration file
```

**Example**:
```bash
./bin/nauts auth -c nauts.json -username alice -password secret123

# Output: eyJ0eXAiOiJKV1QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ...
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

**Example**:
```bash
./bin/nauts serve -c nauts.json

# Service logs:
# INFO: auth callout service started, listening on $SYS.REQ.USER.AUTH
```

The service handles SIGINT/SIGTERM for graceful shutdown.

### Test Environment Setup

A setup script is provided to quickly create a test environment with all required components:

```bash
# Run the setup script
./scripts/setup-test-env.sh ./testenv

# This creates:
# ./testenv/
# ├── nsc/              # nsc operator and accounts
# ├── user-keys/        # Test user nkeys
# ├── users.json        # User credentials (alice, bob, charlie)
# ├── policies.json     # Sample policies
# ├── groups.json       # Sample groups (admin, workers, readonly)
# ├── nauts.json        # nauts configuration file
# ├── nats-server.conf  # NATS server configuration
# └── xkey.seed         # XKey for encrypted auth (if nk is installed)
```

The script creates three test users:
| User | Password | Groups |
|------|----------|--------|
| alice | alice123 | admin, workers |
| bob | bob456 | workers |
| charlie | charlie789 | readonly |

After running the setup script:

```bash
# Build the CLI
go build -o bin/nauts ./cmd/nauts

# Get JWT for alice (admin + workers permissions)
./bin/nauts auth -c ./testenv/nauts.json -username alice -password alice123

# Or run the auth callout service
# 1. Start NATS server
nats-server -c ./testenv/nats-server.conf

# 2. Start nauts serve
./bin/nauts serve -c ./testenv/nauts.json

# 3. Test with nats CLI
nats --user alice --password alice123 pub test.subject 'Hello World'
```

**Requirements**: The setup script requires `nsc` and `go` to be installed. For xkey generation, `nk` is optional.

## Future Enhancements

The following features are planned for future versions:

- **Explicit deny rules**: Support `effect: "deny"` in policy statements with evaluation order: explicit deny > explicit allow > implicit deny.
- **Resource limits**: Allow policies to specify connection limits (`maxSubscriptions`, `maxPayload`, `maxData`).
- **Policy simulation API**: Dry-run endpoint to test compiled permissions without authenticating.
- **Per-user inbox scoping**: Replace global `_INBOX.>` with user-specific prefixes to prevent reply interception.
- **JWT based User Identity Provider**
- **nkey based User Identity Provider**
- **NATS KV Group/Policy Provider**
- **Control Plane**
