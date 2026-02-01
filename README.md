# nauts

**N**ATS **Aut**hentication **S**ervice

## TL;DR

nauts simplifies permission and token management for NATS by granting NATS permissions to external users using access policies. It contains the following components:
- _policy specification and compilation engine_ for usecase driven access policies that provide a scalable abstraction of low-level NATS permissions.
- _authentication service_ for [NATS auth callout](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout) making use of nauts policies.
- _control plane_ to manage policies, groups and accounts within NATS.

## Policy Specification

nauts policies are specified in [POLICY.md](./POLICY.md).

Policies are not attached to users directly. Instead they are assigned to user `groups`. The users of a group inherit NATS permissions via the attached policies. An user can be part of multiple groups and inherits permissions from all its groups.

```typescript
interface Group {
    id: str              // unique identifier
    name: str            // human-readable name
    policies: list[str]  // policy ids of attached policies
}
```

Each NATS account has a default group with id `default`. All users are member of this group allowing to grant default permissions to all users of an account.

### Compilation Engine

nauts provides an engine to compile NATS permissions for a given user identity. 
To this end, it resolves the user's groups and policies from an external store and translates the policies to NATS core permissions. It deduplicates permissions to keep the permission set small.

#### Policy and Group Store

nauts supports the following stores to read policies and groups:

##### File-based

Policies and groups are read from JSON files. The data is only read once during start, afterwards it is kept in memory.

##### NATS KV

> Not implemented yet

Policies and groups are read from NATS Key-Value store. The bucket name defaults to `NAUTS` and can be configured. Policies are stored with key `policy.<policy id>`. Groups are stored with key `group.<group id>`. nauts watches changes on the bucket, so policies and groups can be modified without restart.

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

#### JWT

> Not implemented yet

#### nkeys

> Not implemented yet

## Control Plane

> Not implemented yet