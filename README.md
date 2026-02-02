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
2. For each group, fetch policies from the store
3. For each policy, expand action groups to atomic actions
4. Interpolate variables in resources (e.g., `{{ user.id }}`)
5. Map actions + resources to NATS PUB/SUB permissions
6. Add implicit permissions (`_INBOX.>` for JetStream/KV actions)
7. Deduplicate permissions using wildcard-aware logic
8. Return final `NatsPermissions`

#### Store

A Store contains policies and groups for a NATS account. The following store types can be used:

##### File-based

Policies and groups are read from JSON files. The data is only read once during start, afterwards it is kept in memory.

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

> Not implemented yet

nauts can be deployed as an [auth callout service](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout) for NATS.

### Authentication Flow

When a client connects to NATS, the authentication is delegated to nauts which performs the following steps:

1. _Resolve user identity_: nauts verfies an _identity token_ provided by the connecting client and resolves the user id, the target NATS account id and optional group assignments.
2. _Compile permissions_: nauts uses its compilation engine to compile NATS permissions for the verified user.
3. _Audit logs_: for any connection attempt an audit log is sent to NATS containing the user id and result
4. _Sign JWT for NATS_: nauts signs a JWT encoding the compiled core NATS permissions and returns it.

### User Identity Resolver

nauts implements the following identity resolvers used to verify an _identity token_ provided by the client:

#### Static

Static list of users and group assignments. Verification is based on username and password check.

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

## Package Structure

```
nauts/
├── policy/           # Policy types, compilation, interpolation
│   ├── action.go     # Action types and group expansion
│   ├── compile.go    # Compile() function
│   ├── context.go    # UserContext, GroupContext
│   ├── mapper.go     # Action+Resource to permissions
│   ├── permissions.go # NatsPermissions with wildcard dedup
│   └── resource.go   # Resource parsing
├── auth/             # Authentication service
│   ├── service.go    # AuthService
│   ├── model/        # User, Group types
│   └── store/        # Store interface and implementations
└── testdata/         # Test fixtures
```

## Future Enhancements

The following features are planned for future versions:

- **Explicit deny rules**: Support `effect: "deny"` in policy statements with evaluation order: explicit deny > explicit allow > implicit deny.
- **Resource limits**: Allow policies to specify connection limits (`maxSubscriptions`, `maxPayload`, `maxData`).
- **Policy simulation API**: Dry-run endpoint to test compiled permissions without authenticating.
- **Per-user inbox scoping**: Replace global `_INBOX.>` with user-specific prefixes to prevent reply interception.
- **JWT based User Identity Resolver**
- **nkey based User Identity Resolver**
- **Control Plane**
