# Operator Mode

## Setup

### Setup NATS

#### Adjust NSC paths

```bash
nsc env --all-dirs ...
```

#### Create an operator

```bash
nsc add operator nauts-test --generate-signing-key --sys
nsc edit operator --require-signing-keys
```

#### Add AUTH account

```bash
nsc add account --name AUTH
```

Add a signing key:

```bash
nsc edit account AUTH --sk generate
```

#### Create user and creds for callout service

```bash
nsc add user --account AUTH --name auth
nsc generate creds --account AUTH --name auth > auth.creds
# save public key
AUTH_USER_PUB=$(nsc describe user --account AUTH --name auth --json | jq -r .sub)
```

#### Generate xkey file for request encryption

```bash
nk -gen curve > server-xkey.nkey
# save public key
XKEY_PUB=$(nk -pubout -inkey server-xkey.nkey)
```

#### Enable auth callout

```bash
nsc edit authcallout \
  --account AUTH \
  --curve $XKEY_PUB \
  --auth-user $AUTH_USER_PUB \
  --allowed-account '*'  # allowed accounts can be scoped
```

#### Generate NATS server config

```bash
nsc generate config --mem-resolver --config-file nats-server.conf
```

Add JetStream and general configuration to the server config:

```
# Server settings
server_name: nauts-test
port: 4222
http_port: 8222

# Logging
debug: false
trace: false
logtime: true

# JetStream configuration (for testing js.* and kv.* actions)
jetstream {
    store_dir: /tmp/nauts-test-jetstream
    max_mem: 128M
    max_file: 1G
}
```

#### Add sentinel user

```bash
nsc add user --account AUTH --name sentinel --deny-pubsub \>
nsc generate creds --account AUTH --name sentinel > sentinel.creds
```


### Setup NAUTS

#### Setup groups 

See [groups.json](../groups.json)

#### Setup policies

See [policies.json](../policies.json)

#### Setup users 

See [users.json](../users.json)

#### Setup nauts configuration

See [nauts.json](./nauts.json)

```json
{
  "entity": {
    "type": "operator",
    "operator": {
      "accounts": {
        "AUTH": {
          "publicKey": "<AUTH public key>",
          "signingKeyPath": "<path to signing key>"
        },
        "APP": {
          "publicKey": "<APP public key>",
          "signingKeyPath": "<path to signing key>"
        }
      }
    }
  },
  "nauts": {
    "type": "file",
    "file": {
      "policiesPath": "./policies.json",
      "groupsPath": "./groups.json"
    }
  },
  "identity": {
    "type": "file",
    "file": {
      "usersPath": "./users.json"
    }
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsNkey": "$XKEY_PUB",
    "xkeySeedFile": "./server-xkey.nkey",
  }
}
```
