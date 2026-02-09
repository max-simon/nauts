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

#### Account

Refers to the requested NATS account and contains:
- `account.id`: NATS account

#### Role

Refers to the role this policy is attached to and contains:
- `role.name`: role name (e.g., "admin", "readonly")

#### Example

- role-wide subject for all members of a role: `nats:role.{{ role.name }}.>`
- user-specific subject: `nats:user.{{ user.id }}`
- account-scoped subject: `nats:{{ account.id }}.data.>`

#### Variable Resolution

If a variable cannot be resolved (e.g., `account.id` when no account is provided), the variable evaluates to `null` and the entire resource is excluded from the compiled permissions.

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

| Action         | Description                              | NRN                     | NATS Permissions               |
|----------------|------------------------------------------|-------------------------|--------------------------------|
| `nats.pub`     | Publish messages to subjects             | `nats:<subj>`           | PUB `<subj>`                   |
| `nats.sub`     | Subscribe to subjects (including queues) | `nats:<subj>[:<queue>]` | SUB `<subj>` [queue=`<queue>`] |
| `nats.service` | Subscribe subject and allow responses    | `nats:<subj>`           | SUB `<subj>`, allow responses  |

#### JetStream

All permissions to JetStream API correspond to PUB permissions to the specified subject.

> Note: JetStream domains use domain-prefixed API subjects like `$JS.<domain>.API.>`.
> nauts currently assumes the default prefix (`$JS.API.>`) and does not support domain-specific subjects yet.

##### `js.consume`

This action can be applied to a stream resource (`js:<stream>` or `js:<stream>:*`) and a consumer resource (`js:<stream>:<consumer>`).

If it is applied to a specific consumer, the client is only allowed to receive messages from a durable consumer with this name. This corresponds to the following NATS permissions:
- `$JS.API.CONSUMER.INFO.<stream>.<consumer>`
- `$JS.API.CONSUMER.DURABLE.CREATE.<stream>.<consumer>`
- `$JS.API.CONSUMER.MSG.NEXT.<stream>.<consumer>`
- `$JS.ACK.<stream>.<consumer>.>`
- `$JS.SNAPSHOT.RESTORE.<stream>.*`
- `$JS.SNAPSHOT.ACK.<stream>.*`
- `$JS.FC.<stream>.>`

If `<consumer>` is `*` or not given, the client can receive messages from any consumer, including new ones. This corresponds to the following NATS permissions:
- `$JS.API.CONSUMER.*.<stream>` (includes `CREATE`, `NAMES`, and `LIST`)
- `$JS.API.CONSUMER.*.<stream>.>` (includes `DELETE`, `INFO`)
- `$JS.API.CONSUMER.DURABLE.CREATE.<stream>.>`
- `$JS.API.CONSUMER.MSG.NEXT.<stream>.*`
- `$JS.ACK.<stream>.>`
- `$JS.SNAPSHOT.RESTORE.<stream>.*`
- `$JS.SNAPSHOT.ACK.<stream>.*`
- `$JS.FC.<stream>.>`

Clients are also allowed to request messages via JetStream Direct Get:
- `$JS.API.DIRECT.GET.<stream>`
- `$JS.API.DIRECT.GET.<stream>.>`

##### `js.manage`

This action is applied to stream resources (`js:<stream>`). It allows clients to manage a JetStream. This corresponds to the following NATS permissions:
- all `js.consume` permissions for this resource
- `$JS.API.STREAM.*.<stream>` (includes `CREATE`, `UPDATE`, `DELETE`, `INFO`, `PURGE`, `SNAPSHOT` and `RESTORE`)
- `$JS.API.STREAM.MSG.*.<stream>` (includes `GET` and `DELETE`)

If `<stream>` is `*`, the following permissions are added
- `$JS.API.STREAM.LIST`
- `$JS.API.STREAM.NAMES`

##### `js.view`

This is a read-only role for stream resources (`js:<stream>`). It allows clients view JetStream resources without giving any read permissions on the data. This corresponds to the following NATS permissions:
- `$JS.API.STREAM.INFO.<stream>`
- `$JS.API.CONSUMER.INFO.<stream>.*` (or `$JS.API.CONSUMER.INFO.*.*` if `<stream>` is `*`)
- `$JS.API.CONSUMER.LIST.<stream>`
- `$JS.API.CONSUMER.NAMES.<stream>`

If `<stream>` is `*`, the following permissions are added
- `$JS.API.STREAM.LIST`
- `$JS.API.STREAM.NAMES`


#### KV

##### `kv.read`

This action can be applied to a KV bucket resource (`kv:<bucket>` or `kv:<bucket>:>`) and a KV key resource (`kv:<bucket>:<key>`). `<bucket>` must not be `*`.

If it is applied to a specific key (`<key>` is not `>`), the client is only allowed to receive the value for this key. This corresponds to the following NATS permissions:
- `$JS.API.STREAM.INFO.KV_<bucket>`
- `$JS.API.DIRECT.GET.KV_<bucket>.$KV.<bucket>.<key>`

Clients are also allowed to _subscribe_ to the following subject for live updates:
- `$KV.<bucket>.<key>`

If `<key>` is `>` or not given, the client can receive the value for any key in the bucket. This corresponds to the following NATS permissions:
- `$JS.API.STREAM.INFO.KV_<bucket>`
- `$JS.API.DIRECT.GET.KV_<bucket>.$KV.<bucket>.>`
- `$JS.API.CONSUMER.CREATE.KV_<bucket>`
- `$JS.API.CONSUMER.CREATE.KV_<bucket>.>`
- `$JS.FC.KV_<bucket>.>`

If `<key>` is `>` or not given, clients are also allowed to _subscribe_ to the following subject for live updates:
- `$KV.<bucket>.>`

##### `kv.edit`

This action can be applied to a KV bucket resource (`kv:<bucket>` or `kv:<bucket>:>`) and a KV key resource (`kv:<bucket>:<key>`). `<bucket>` must not be `*`. 

It grants all `kv.read` permissions for the resource plus the following permissions:
- `$KV.<bucket>.<key>` (`$KV.<bucket>.>` for bucket resources)

##### `kv.view`

This is a read-only role for bucket resources (`kv:<bucket>`). It allows clients to view KV buckets without giving any read permissions on the data. 

If `<bucket>` is not `*`, the following NATS permissions are added:
- `$JS.API.STREAM.INFO.KV_<bucket>`

If `<bucket>` is `*`, the following NATS permissions are added:
- `$JS.API.STREAM.LIST`
- `$JS.API.STREAM.INFO.*`

> Note: this allows to list all streams, not only KV buckets.

##### `kv.manage`

This action is applied to bucket resources (`kv:<bucket>`). It allows clients to manage KV buckets. 

If `<bucket>` is not `*`, the following NATS permissions are added:
- all `kv.read` permissions for this resource
- `$JS.API.STREAM.*.KV_<bucket>` (includes `INFO`, `CREATE` and `DELETE`)

If `<bucket>` is `*`, the following NATS permissions are added:
- all `kv.read` permissions for this resource
- `$JS.API.STREAM.LIST`
- `$JS.API.STREAM.INFO.*`
- `$JS.API.STREAM.*.*`

> Note: this allows to manage all streams, not only KV buckets.


### Implicit Permissions

Certain actions require implicit permissions that are automatically granted:

#### Reply Inbox Subscription

Every user gets permissions to subscribe to its personalized inbox subject, namely `SUB _INBOX_{{ user.id }}.>`. This inbox subject should be used as an inbox prefix for all req/reply actions, including all JetStream and Key-Value requests.

#### JetStream Info

Every user that has at least one JetStream action gets `PUB $JS.API.INFO` permissions to retrieve general information about JetStream.

### Action Groups

Action groups can be used to resolve multiple atomic actions using a single action name. The following action groups are defined:

| Group       | Resolves To             |
|-------------|-------------------------|
| `nats.*`    | All `nats.*` actions    |
| `js.*`      | `js.manage`             |
| `kv.*`      | `kv.manage`             |

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