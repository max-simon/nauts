#!/bin/bash
set -e

#
# NATS Server Configuration
#
echo "Generating NATS Server Configuration..."

# 1. Account Key (Issuer)
# In static mode, this single key identifies the issuer for all accounts
nk -gen account > account-AUTH.nk
ISSUER_PUB=$(nk -inkey account-AUTH.nk -pubout)
echo "Generated account-AUTH.nk (Issuer: $ISSUER_PUB)"

# 2. Auth Service User
# The nauts process uses this credentials to connect to NATS
nk -gen user > user-auth.nk
AUTH_USER_PUB=$(nk -inkey user-auth.nk -pubout)
echo "Generated user-auth.nk (Auth User: $AUTH_USER_PUB)"

# 3. Get public key of server xkey
# Assuming ../common/server-xkey.nk exists relative to this script execution directory
XKEY_PUB=$(nk -inkey ../common/server-xkey.nk -pubout);

# 5. Write nats-server.conf
cat > nats-server.conf <<EOF
server_name: nauts-test-policy
jetstream {
    store_dir: /tmp/nauts-test-policy-static-jetstream
    max_mem: 128M
    max_file: 1G
}

# Account definitions
accounts {
    
    # AUTH account - used by the nauts auth callout service
    AUTH {
        users: [
            { nkey: $AUTH_USER_PUB }
        ]
    }

    POLICY {
        # No static users - all users are authenticated via nauts
        jetstream: enabled
    }

    SYS {
        # System account
    }
}

# System account configuration
system_account: SYS

# Authorization with auth callout
authorization {

    auth_callout {
        # The account that issues user JWTs
        issuer: $ISSUER_PUB

        # Allow users to login with nkeys
        # - auth-service user to handle auth callout requests
        users: [ 
            $AUTH_USER_PUB
        ]

        # Account where auth callout service connects
        account: AUTH

        # XKey for encrypted auth callout
        xkey: $XKEY_PUB
    }
}
EOF

#
# NAUTS Configuration
#
echo "Generating NAUTS Configuration...";

# Create policies.json
cat > policies.json <<EOF
[
  {
    "id": "admin",
    "name": "Admin Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.*", "js.*", "kv.*"],
        "resources": ["nats:>", "js:*", "kv:*"]
      }
    ]
  },
  {
    "id": "writer",
    "name": "Writer Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:public.>"]
      },
      {
        "effect": "allow",
        "actions": ["js.manage"],
        "resources": ["js:STREAM_WRITE"]
      },
      {
        "effect": "allow",
        "actions": ["kv.edit"],
        "resources": ["kv:BUCKET_WRITE"]
      }
    ]
  },
  {
    "id": "reader",
    "name": "Reader Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:public.>"]
      },
      {
        "effect": "allow",
        "actions": ["js.view"],
        "resources": ["js:STREAM_WRITE"]
      },
      {
        "effect": "allow",
        "actions": ["js.consume"],
        "resources": ["js:STREAM_READ:CONSUMER_READ"]
      },
      {
        "effect": "allow",
        "actions": ["kv.view"],
        "resources": ["kv:BUCKET_WRITE"]
      },
      {
        "effect": "allow",
        "actions": ["kv.read"],
        "resources": ["kv:BUCKET_READ"]
      }
    ]
  },
  {
    "id": "service",
    "name": "Service Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.service"],
        "resources": ["nats:service.>"]
      }
    ]
  },
  {
    "id": "user-vars",
    "name": "User Variables Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub", "nats.sub"],
        "resources": ["nats:user.{{user.id}}.>"]
      },
      {
        "effect": "allow",
        "actions": ["kv.edit"],
        "resources": ["kv:USER_{{user.id}}"]
      },
      {
        "effect": "allow",
        "actions": ["js.consume"],
        "resources": ["js:STREAM_VARS:{{user.id}}"]
      }
    ]
  }
]
EOF

# Create bindings.json
cat > bindings.json <<EOF
[
  {
    "role": "admin",
    "account": "POLICY",
    "policies": ["admin"]
  },
  {
    "role": "writer",
    "account": "POLICY",
    "policies": ["writer"]
  },
  {
    "role": "reader",
    "account": "POLICY",
    "policies": ["reader"]
  },
  {
    "role": "service",
    "account": "POLICY",
    "policies": ["service"]
  },
  {
    "role": "user",
    "account": "POLICY",
    "policies": ["user-vars"]
  }
]
EOF

# Create users.json
# Password hash for "secret": \$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi
cat > users.json <<EOF
{
  "users": {
    "admin": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.admin"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "writer": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.writer"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "reader": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.reader"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "service": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.service"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "alice": {
      "id": "alice",
      "accounts": ["POLICY"],
      "roles": ["POLICY.user"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "bob": {
      "id": "bob",
      "accounts": ["POLICY"],
      "roles": ["POLICY.user"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    }
  }
}
EOF

# Create nauts.json
cat > nauts.json <<EOF
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "$ISSUER_PUB",
      "privateKeyPath": "./account-AUTH.nk",
      "accounts": [
        "AUTH", "POLICY"
      ]
    }
  },
  "policy": {
    "type": "file",
    "file": {
      "policiesPath": "./policies.json",
      "bindingsPath": "./bindings.json"
    }
  },
  "auth": {
    "file": [
      {
        "id": "policy-file",
        "accounts": ["POLICY"],
        "userPath": "./users.json"
      }
    ]
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsNkey": "./user-auth.nk",
    "xkeySeedFile": "../common/server-xkey.nk"
  }
}
EOF
