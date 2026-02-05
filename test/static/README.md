# Static Mode Test Environment

Static mode uses a single signing key for all accounts. This is simpler to set up and suitable for development or single-account deployments.

## Quick Start

```bash
# 1. Start NATS server
nats-server -c nats-server.conf

# 2. Start nauts auth callout service (in another terminal)
../../bin/nauts serve -c nauts.json

# 3. Test authentication
nats --token '{"account":"APP","token":"alice:secret"}' sub "public.>"
nats --token '{"account":"APP","token":"bob:secret"}' pub public.test "Hello"
```

## Test Users

| User  | Password | Roles    | Permissions |
|-------|----------|----------|-------------|
| alice | secret   | readonly | Subscribe to `public.>` |
| bob   | secret   | full     | Pub/Sub to `public.>` |

## Files

| File | Description |
|------|-------------|
| `nauts.json` | nauts configuration |
| `nats-server.conf` | NATS server configuration |
| `account-AUTH.nk` | Account signing key (signs user JWTs) |
| `user-auth.nk` | User nkey for callout service connection |
| `server-xkey.nk` | Curve key for request encryption |

## Configuration

```json
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "<account-public-key>",
      "privateKeyPath": "./account-AUTH.nk",
      "accounts": ["AUTH", "APP"]
    }
  },
  "policy": {
    "type": "file",
    "file": { "policiesPath": "../policies.json", "bindingsPath": "../bindings.json" }
  },
  "auth": {
    "file": [
      {
        "id": "local",
        "accounts": ["*", "AUTH"],
        "userPath": "../users.json"
      }
    ]
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsNkey": "./user-auth.nk",
    "xkeySeedFile": "./server-xkey.nk"
  }
}
```

## Setup from Scratch

### 1. Create Keys

```bash
# Account signing key (signs user JWTs)
nk -gen account > account-AUTH.nk
AUTH_ACC_PUB=$(nk -pubout -inkey account-AUTH.nk)

# User nkey for callout service
nk -gen user > user-auth.nk
AUTH_USER_PUB=$(nk -pubout -inkey user-auth.nk)

# Curve key for encryption
nk -gen curve > server-xkey.nk
XKEY_PUB=$(nk -pubout -inkey server-xkey.nk)
```

### 2. Configure NATS Server

```
accounts {
    AUTH { users: [ { nkey: <AUTH_USER_PUB> } ] }
    APP {}
}

authorization {
    auth_callout {
        issuer: <AUTH_ACC_PUB>
        account: AUTH
        xkey: <XKEY_PUB>
        users: [ <AUTH_USER_PUB> ]
    }
}
```

### 3. Update nauts.json

Set `account.static.publicKey` to `<AUTH_ACC_PUB>`.
