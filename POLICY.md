# Policy Specification

naut policies grant permissions by allowing or denying (`effect`) specific `actions` to NATS `resources`.

## Resources (NRN)

Resources are identified by their _naut resource name_ (NRN). A NRN follows the pattern

```
<type>:<identifier>[:<sub-identifier>]
```

The following NATS objects can be referenced with a NRN:

| NRN Pattern              | Description        | Example                 |
|---                       |---                 |---                      |
| `nats:<subject>`         | NATS subject       | `nats:orders.>`         |
| `nats:<subject>:<queue>` | Queue subscription | `nats:orders.*:workers` |
| `js:<stream>`            | JetStream stream   | `js:ORDERS`             |
| `js:<stream>:<consumer>` | Durable consumer   | `js:ORDERS:processor`   |
| `kv:<bucket>`            | KV bucket          | `kv:config`             |
| `kv:<bucket>:<key>`      | KV key             | `kv:config:app.>`       |

### Wildcards

NATS wildcard `*` is supported for.
- subject names
- queue names
- stream names
- consumer names
- bucket names
- bucket keys

NATS wildcard `>` is supported for:
- subject names
- bucket keys

**Example:** Valid NRN:
- `nats:prod.>:my-queue`: `my-queue` subscriber for `prod.>`
- `js:*:*`: all consumers on all streams

**Example:** Invalid NRN:
- `kv:prod.>`: not a valid bucket name
- `js:ORDERS:test.>`: not a valid consumer name

### Variable interpolation

NRNs support variable interpolation using `{{ }}` to scope resources to given context objects. Currently, the following context objects can be used:

#### User

Refers to the user identity and contains:
- `user.id`: user identifier
- `user.account`: NATS account
- `user.attr.<key>`: any additional attribute of the user identity

#### Role

Refers to the role this policy is attached to and contains:
- `role.name`: role name (e.g., "admin", "readonly")
- `role.account`: account this role belongs to ("*" for global roles)

#### Example

- role-wide subject for all members of a role: `nats:role.{{ role.name }}.>`
- user-specific subject: `nats:user.{{ user.id }}`
- account-scoped subject: `nats:{{ role.account }}.data.>`

#### Variable Resolution

If a variable cannot be resolved (e.g., `user.attr.department` when the user has no `department` attribute), the variable evaluates to `null` and the entire resource is excluded from the compiled permissions.

#### Variable Sanitization

To prevent injection attacks, interpolated values are validated:

- Empty strings resolve to `null` (resource excluded)
- The `>` wildcard character is **not allowed** in interpolated values
- The `*` wildcard character is **not allowed** in interpolated values
- Values must only contain valid NATS subject tokens (alphanumeric, `-`, `_`)

If a value fails validation, the resource is excluded and a warning is logged.

#### System

> Not implemented yet

- `client`: `client_info` data available in the [authentication request](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout#schema)
- `nats`: `server_id` data available in the [authentication request](https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout#schema)


## Actions

### Atomic Actions

naut implements the following atomic actions:

#### Core NATS

| Action     | Description                              | NRN                     | NATS Permissions               |
|------------|------------------------------------------|-------------------------|--------------------------------|
| `nats.pub` | Publish messages to subjects             | `nats:<subj>`           | PUB `<subj>`                   |
| `nats.sub` | Subscribe to subjects (including queues) | `nats:<subj>[:<queue>]` | SUB `<subj>` [queue=`<queue>`] |
| `nats.req` | Request/reply pattern                    | `nats:<subj>`           | PUB `<subj>`, SUB `_INBOX.>`   |

#### JetStream

| Action              | Description                           | NRN                  | NATS Permissions                                                                                    |
|---------------------|---------------------------------------|----------------------|-----------------------------------------------------------------------------------------------------|
| `js.readStream`     | Get stream info, list streams         | `js:<stream>`        | `$JS.API.STREAM.INFO.<stream>` (if `js:*`: `$JS.API.STREAM.LIST`, `$JS.API.STREAM.NAMES`)           |
| `js.writeStream`    | Create, update, seal, or purge stream | `js:<stream>`        | `$JS.API.STREAM.CREATE.<stream>`, `$JS.API.STREAM.UPDATE.<stream>`, `$JS.API.STREAM.PURGE.<stream>` |
| `js.deleteStream`   | Delete stream and all data            | `js:<stream>`        | `$JS.API.STREAM.DELETE.<stream>`                                                                    |
| `js.readConsumer`   | Get consumer info, list consumers     | `js:<stream>:<cons>` | `$JS.API.CONSUMER.INFO.<stream>.<cons>` (for `js:<stream>:*`: `$JS.API.CONSUMER.LIST.<stream>`, `$JS.API.CONSUMER.NAMES.<stream>`) |
| `js.writeConsumer`  | Create or update consumer             | `js:<stream>:<cons>` | `$JS.API.CONSUMER.CREATE.<stream>.<cons>.>`, `$JS.API.CONSUMER.DURABLE.CREATE.<stream>.<cons>`      |
| `js.deleteConsumer` | Delete consumer                       | `js:<stream>:<cons>` | `$JS.API.CONSUMER.DELETE.<stream>.<cons>`                                                           |
| `js.consume`        | Fetch messages and acknowledge        | `js:<stream>:<cons>` | `$JS.API.CONSUMER.MSG.NEXT.<stream>.<cons>`, `$JS.ACK.<stream>.<cons>.>`                            |

**Note:** Push consumers also require `SUB` on the delivery subject and `PUB` to `$JS.FC.<stream>.>` for flow control.

**Note:** All JetStream operations use request/reply and implicitly require `SUB _INBOX.>`.

#### KV

| Action           | Description                   | NRN                   | NATS Permissions                                        |
|------------------|-------------------------------|-----------------------|---------------------------------------------------------|
| `kv.read`        | Get key values, bucket info   | `kv:<bucket>:<key>`   | PUB `$JS.API.DIRECT.GET.KV_<bucket>.$KV.<bucket>.<key>` |
| `kv.write`       | Write key values              | `kv:<bucket>:<key>`   | PUB `$KV.<bucket>.<key>`                                |
| `kv.watchBucket` | Watch for changes, list keys  | `kv:<bucket>`         | PUB `$JS.API.CONSUMER.CREATE.KV_<bucket>.*.$KV.<bucket>.>`, PUB `$JS.API.CONSUMER.DELETE.KV_<bucket>.>`, SUB delivery, PUB `$JS.FC.KV_<bucket>.>` |
| `kv.readBucket`  | Get bucket info               | `kv:<bucket>`         | PUB `$JS.API.STREAM.INFO.KV_<bucket>`                   |
| `kv.writeBucket` | Create or update bucket       | `kv:<bucket>`         | PUB `$JS.API.STREAM.CREATE.KV_<bucket>`                 |
| `kv.deleteBucket`| Delete KV bucket              | `kv:<bucket>`         | PUB `$JS.API.STREAM.DELETE.KV_<bucket>`                 |

**Note:** All KV operations use request/reply and implicitly require `SUB _INBOX.>`.

### Implicit Permissions

Certain actions require implicit permissions that are automatically granted:

#### Reply Inbox Subscription

All JetStream (`js.*`) and KV (`kv.*`) actions use the request/reply pattern and require subscribing to reply inboxes. When **any** `js.*` or `kv.*` action is granted, nauts automatically adds `SUB _INBOX.>` to the compiled permissions.

**Known limitation:** This grants access to all inbox subjects. A future enhancement will scope inboxes per-user (e.g., `_INBOX.<user-prefix>.*`) to prevent potential reply interception.

### Action Groups

Action groups can be used to resolve multiple atomic actions using a single action name. The following action groups are defined:

| Group       | Resolves To                                        |
|-------------|----------------------------------------------------|
| `nats.*`    | All `nats.*` actions                               |
| `js.viewer` | `js.readStream`, `js.readConsumer`                 |
| `js.worker` | `js.viewer`, `js.writeConsumer`, `js.consume`      |
| `js.*`      | All `js.*` actions                                 |
| `kv.reader` | `kv.read`, `kv.watchBucket`                        |
| `kv.writer` | `kv.reader`, `kv.write`                            |
| `kv.*`      | All `kv.*` actions                                 |

## Policy

A policy is a collection of permission _statements_. A statement contains a set of `actions` that should be allowed or denied for a set of `resources`. 

**Important:** Currently, actions can only be allowed but not explicitly denied.

```typescript
interface Statement {
    effect: "allow"        // explicit deny not implemented
    actions: list[Action]  // list of actions to allow on resources
    resources: list[str]   // list of resources to allow actions on
}

interface Policy {
    id: str    // unique identifier
    name: str  // human-readable name
    statements: list[Statement]  // list of permission statements
}
```