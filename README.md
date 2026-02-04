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
accountProvider, _ := provider.NewOperatorAccountProvider(provider.OperatorAccountProviderConfig{
    Accounts: map[string]provider.AccountSigningConfig{
        "AUTH": {PublicKey: "AAUTH...", SigningKeyPath: "/path/to/auth-signing.nk"},
        "APP":  {PublicKey: "AAPP...", SigningKeyPath: "/path/to/app-signing.nk"},
    },
})
groupProvider, _ := provider.NewFileGroupProvider(provider.FileGroupProviderConfig{
    GroupsPath: "groups.json",
})
policyProvider, _ := provider.NewFilePolicyProvider(provider.FilePolicyProviderConfig{
    PoliciesPath: "policies.json",
})
identityProvider, _ := identity.NewFileUserIdentityProvider(identity.FileUserIdentityProviderConfig{
    UsersPath: "users.json",
})
controller := auth.NewAuthController(accountProvider, groupProvider, policyProvider, identityProvider)

// Authenticate using ConnectOptions (used by auth callout)
result, err := controller.Authenticate(ctx, jwt.ConnectOptions{
    Token: "alice:secret",  // Token format depends on identity provider
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

### Account Provider

Account providers supply NATS account information and signing keys for JWT issuance. nauts supports two account provider modes:

#### Operator Mode (OperatorAccountProvider)

For NATS deployments with operator/account hierarchy where the auth service runs in the AUTH account but authenticates users for all accounts. Each account has its own signing key.

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

#### Static Mode (StaticAccountProvider)

A simplified provider that uses a single signing key for all accounts. Useful for development and simple deployments where all accounts share the same credentials.

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
├── provider/               # Account, group, and policy providers
│   ├── entity.go           # Account type with Signer
│   ├── account_provider.go # AccountProvider interface
│   ├── operator_account_provider.go # OperatorAccountProvider (operator mode)
│   ├── static_account_provider.go # StaticAccountProvider
│   ├── group_provider.go   # GroupProvider interface
│   ├── file_group_provider.go # FileGroupProvider
│   ├── policy_provider.go  # PolicyProvider interface
│   ├── file_policy_provider.go # FilePolicyProvider
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
│ │    User     │ │    │ │GroupProvider│ │         │ │   Signer    │ │
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

### Component Responsibilities

| Package | Responsibility |
|---------|---------------|
| `policy/` | Policy specification, compilation, variable interpolation, action mapping |
| `provider/` | NATS account management, policy storage, group storage |
| `identity/` | User authentication and identity resolution |
| `jwt/` | NATS JWT creation and signing (with explicit deny for empty permissions) |
| `auth/` | Authentication orchestration and NATS auth callout service |

### JWT Permission Encoding

NATS JWT defaults to allowing everything when no permissions are specified. nauts handles this by explicitly denying all (`">"`)) when no allow permissions are granted:

- Empty pub permissions → `Pub.Deny: [">"]` (user cannot publish)
- Empty sub permissions → `Sub.Deny: [">"]` (user cannot subscribe)
- Non-empty permissions → only `Allow` list is set

This ensures the principle of least privilege: users are denied access by default unless explicitly granted.

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
  "group": {
    "type": "file",
    "file": {
      "path": "groups.json"
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

**Static mode** (simpler setup for development):

```json
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "AXXXXX...",
      "privateKeyPath": "/path/to/account.nk",
      "accounts": ["AUTH", "APP"]
    }
  },
  "group": { ... },
  "policy": { ... },
  "identity": { ... },
  "server": { ... }
}
```

#### Configuration Sections

| Section | Description |
|---------|-------------|
| `account` | Account provider configuration (operator/account info) |
| `group` | Group provider configuration (user groups) |
| `policy` | Policy provider configuration (access policies) |
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
  -token string          Token to authenticate (required, format depends on identity provider)
  -user-pubkey string    User's public key for JWT subject (optional, generates ephemeral key if not provided)
  -ttl duration          JWT time-to-live (default 1h, overrides config)

Environment variables:
  NAUTS_CONFIG    Path to configuration file
```

**Example**:
```bash
# For file identity provider, token format is "username:password"
./bin/nauts auth -c nauts.json -token alice:secret123

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

### Test Environments

Pre-configured test environments are provided in the `test/` directory for both operator and static modes:

```
test/
├── e2e_test.go         # Go e2e test suite
├── client/             # Test client for manual testing
│   └── main.go
├── policies.json       # Shared policies (read-access, write-access)
├── groups.json         # Shared groups (readonly, full)
├── users.json          # Shared users (alice, bob)
├── operator/           # Operator mode setup
│   ├── README.md       # Setup instructions
│   ├── nauts.json      # nauts configuration
│   ├── nats-server.conf# NATS server config with operator JWTs
│   ├── auth.creds      # Auth service credentials
│   ├── sentinel.creds  # Sentinel credentials for client auth
│   └── *.nk            # Signing keys and xkey
└── static/             # Static mode setup
    ├── README.md       # Setup instructions
    ├── nauts.json      # nauts configuration
    ├── nats-server.conf# NATS server config with accounts
    └── *.nk            # Account key and xkey
```

The test users are:
| User | Token | Groups | Account | Permissions |
|------|-------|--------|---------|-------------|
| alice | alice:secret | readonly | APP | Subscribe to `public.>` |
| bob | bob:secret | full | APP | Pub/Sub to `public.>` |

#### Quick Start (Static Mode)

```bash
# Build the CLI
go build -o bin/nauts ./cmd/nauts

# Get JWT for alice
./bin/nauts auth -c ./test/static/nauts.json -token alice:secret

# Or run the auth callout service
# 1. Start NATS server
nats-server -c ./test/static/nats-server.conf

# 2. Start nauts serve (in another terminal)
./bin/nauts serve -c ./test/static/nauts.json

# 3. Test with nats CLI (token format: username:password)
nats --token alice:secret sub "public.>"
nats --token bob:secret pub public.test "Hello World"
```

#### Quick Start (Operator Mode)

```bash
# 1. Start NATS server
nats-server -c ./test/operator/nats-server.conf

# 2. Start nauts serve (in another terminal)
./bin/nauts serve -c ./test/operator/nauts.json

# 3. Test with nats CLI (requires sentinel credentials + token)
nats --creds ./test/operator/sentinel.creds --token alice:secret sub "public.>"
nats --creds ./test/operator/sentinel.creds --token bob:secret pub public.test "Hello"
```

#### Running E2E Tests

```bash
cd test
go test -v -static .   # Run static mode e2e tests
go test -v -operator . # Run operator mode e2e tests
```

See `test/operator/README.md` and `test/static/README.md` for detailed setup instructions for each mode.

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
