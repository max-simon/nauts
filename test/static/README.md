# Static Mode

## Setup

### Setup AUTH Account

1. Create nkey user for callout service

```bash
nk -gen user > user-auth.nk
AUTH_USER_PUB=$(nk -pubout -inkey user-auth.nk)
```

2. Add `AUTH` account to server config

```
accounts {
    AUTH {
        users: [
            { nkey: $AUTH_USER_PUB }
        ]
    }

    APP {} # no explicit users
    
    # ...
}
```

3. Create nkey for `AUTH` account

```bash
nk -gen account > account-auth.nk
AUTH_ACC_PUB=$(nk -pubout -inkey account-auth.nk)
```

4. Generate xkey file for request encryption

```bash
nk -gen curve > server-xkey.nk
# save public key
XKEY_PUB=$(nk -pubout -inkey server-xkey.nk)
```

5. Add callout authorization block to server config

```
authorization {
    auth_callout {
        issuer: $AUTH_ACC_PUB
        account: AUTH
        xkey: $XKEY_PUB
        # these users can login without auth callout service
        users: [ 
            $AUTH_USER_PUB, 
            # other system users, i.e. for SYS account
        ] 
    }
}
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
    "type": "static",
    "static": {
      "publicKey": "$AUTH_ACC_PUB",
      "privateKeyPath": "./account-auth.nk",
      "accounts": [
        "APP"
        // list of supported account names
      ]
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
    "xkeySeedFile": "./server-xkey.nk",
  }
}
```