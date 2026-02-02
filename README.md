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

// Authenticate
result, err := controller.Authenticate(ctx, identity.UsernamePassword{
    Username: "alice",
    Password: "secret",
}, userPublicKey, time.Hour)
// result.User contains user info
// result.Permissions contains compiled NATS permissions
// result.JWT contains signed NATS user JWT
```

> **Note**: NATS auth callout protocol integration (NATS message handling) is not yet implemented.

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

Entity providers supply NATS operator and account information, including signing keys for JWT issuance.

#### Nsc-based (NscEntityProvider)

Reads operator and account information from an [nsc](https://docs.nats.io/using-nats/nats-tools/nsc) directory structure:

```
~/.nsc/
├── nats/
│   └── <operator>/
│       ├── <operator>.jwt
│       └── accounts/
│           └── <account>/
│               └── <account>.jwt
└── keys/
    └── keys/
        ├── O/<prefix>/<operator-pubkey>.nk
        └── A/<prefix>/<account-pubkey>.nk
```

## Package Structure

```
nauts/
├── cmd/
│   └── nauts/              # CLI entrypoint
│       └── main.go         # Authentication CLI
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
│   ├── nsc_entity_provider.go # NscEntityProvider
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
├── auth/                   # Authentication controller
│   ├── controller.go       # AuthController
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
| `auth/` | Authentication orchestration combining all components |

## CLI

nauts provides a command-line interface for authenticating users and generating NATS JWTs.

### Installation

```bash
go build -o bin/nauts ./cmd/nauts
```

### Usage

```bash
./bin/nauts [options]

Options:
  -nsc-dir string      Path to nsc directory (required)
  -operator string     Operator name (required)
  -policies string     Path to policies JSON file (required)
  -groups string       Path to groups JSON file (required)
  -users string        Path to users JSON file (required)
  -username string     Username to authenticate (required)
  -password string     Password for authentication (required)
  -user-pubkey string  User's public key for JWT subject (optional, generates ephemeral key if omitted)
  -ttl duration        JWT time-to-live (default 1h)
```

### Example

```bash
# Authenticate user and get JWT (with ephemeral key)
./bin/nauts \
  -nsc-dir ~/.nsc \
  -operator myoperator \
  -policies /path/to/policies.json \
  -groups /path/to/groups.json \
  -users /path/to/users.json \
  -username alice \
  -password secret123

# Or with a specific user public key
./bin/nauts \
  -nsc-dir ~/.nsc \
  -operator myoperator \
  -policies /path/to/policies.json \
  -groups /path/to/groups.json \
  -users /path/to/users.json \
  -username alice \
  -password secret123 \
  -user-pubkey UABC123XYZ...

# Output: eyJ0eXAiOiJKV1QiLCJhbGciOiJlZDI1NTE5LW5rZXkifQ...
```

The CLI outputs the signed JWT to stdout on success. The JWT can be used with NATS clients that support JWT-based authentication.

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
# └── groups.json       # Sample groups (admin, workers, readonly)
```

The script creates three test users:
| User | Password | Groups |
|------|----------|--------|
| alice | alice123 | admin, workers |
| bob | bob456 | workers |
| charlie | charlie789 | readonly |

After running the setup script, authenticate with:

```bash
# Build the CLI
go build -o bin/nauts ./cmd/nauts

# Get JWT for alice (admin + workers permissions)
./bin/nauts \
  -nsc-dir ./testenv/nsc \
  -operator test-operator \
  -policies ./testenv/policies.json \
  -groups ./testenv/groups.json \
  -users ./testenv/users.json \
  -username alice \
  -password alice123
```

**Requirements**: The setup script requires `nsc` and `go` to be installed.

## Future Enhancements

The following features are planned for future versions:

- **NATS auth callout integration**: Wire up `CalloutService` to NATS auth callout protocol with JWT signing.
- **Explicit deny rules**: Support `effect: "deny"` in policy statements with evaluation order: explicit deny > explicit allow > implicit deny.
- **Resource limits**: Allow policies to specify connection limits (`maxSubscriptions`, `maxPayload`, `maxData`).
- **Policy simulation API**: Dry-run endpoint to test compiled permissions without authenticating.
- **Per-user inbox scoping**: Replace global `_INBOX.>` with user-specific prefixes to prevent reply interception.
- **JWT based User Identity Provider**
- **nkey based User Identity Provider**
- **NATS KV Group/Policy Provider**
- **Control Plane**
