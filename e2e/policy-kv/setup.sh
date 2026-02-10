#!/bin/bash
set -e

#
# NATS Server Configuration
#
echo "Generating NATS Server Configuration..."

# Suite-local xkey seed (used for encrypted auth callout)
if [ ! -f server-xkey.nk ]; then
  nk -gen curve > server-xkey.nk
fi

# 1. Account Key (Issuer)
# In static mode, this single key identifies the issuer for all accounts
if [ ! -f account-AUTH.nk ]; then
  nk -gen account > account-AUTH.nk
fi
ISSUER_PUB=$(nk -inkey account-AUTH.nk -pubout)
echo "Generated account-AUTH.nk (Issuer: $ISSUER_PUB)"

# 2. Auth Service User
# The nauts process uses this credentials to connect to NATS
if [ ! -f user-auth.nk ]; then
  nk -gen user > user-auth.nk
fi
AUTH_USER_PUB=$(nk -inkey user-auth.nk -pubout)
echo "Generated user-auth.nk (Auth User: $AUTH_USER_PUB)"

# 3. Get public key of server xkey
XKEY_PUB=$(nk -inkey ./server-xkey.nk -pubout);

# 5. Write nats-server.conf
cat > nats-server.conf <<EOF
server_name: nauts-test-policy
jetstream {
  store_dir: /tmp/nauts-test-policy-kv-jetstream
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
    "account": "POLICY",
    "name": "Admin",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.*", "js.manage", "js.view", "kv.manage", "kv.view"],
        "resources": ["nats:>", "js:*", "kv:*"]
      }
    ]
  },
  {
    "id": "kv-manage",
    "account": "POLICY",
    "name": "KV Manage",
    "statements": [
      {
        "effect": "allow",
        "actions": ["kv.manage"],
        "resources": ["kv:BUCKET_MGR"]
      }
    ]
  },
  {
    "id": "kv-view",
    "account": "POLICY",
    "name": "KV View",
    "statements": [
      {
        "effect": "allow",
        "actions": ["kv.view"],
        "resources": ["kv:BUCKET_VIEW"]
      }
    ]
  },
  {
    "id": "kv-read-any",
    "account": "POLICY",
    "name": "KV Read Any Key",
    "statements": [
      {
        "effect": "allow",
        "actions": ["kv.read"],
        "resources": ["kv:BUCKET_DATA:>"]
      }
    ]
  },
  {
    "id": "kv-edit-any",
    "account": "POLICY",
    "name": "KV Edit Any Key",
    "statements": [
      {
        "effect": "allow",
        "actions": ["kv.edit"],
        "resources": ["kv:BUCKET_EDIT:>"]
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
    "role": "manager",
    "account": "POLICY",
    "policies": ["kv-manage"]
  },
  {
    "role": "viewer",
    "account": "POLICY",
    "policies": ["kv-view"]
  },
  {
    "role": "reader",
    "account": "POLICY",
    "policies": ["kv-read-any"]
  },
  {
    "role": "editor",
    "account": "POLICY",
    "policies": ["kv-edit-any"]
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
    "manager": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.manager"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "viewer": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.viewer"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "reader": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.reader"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "editor": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.editor"],
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
    "xkeySeedFile": "./server-xkey.nk"
  }
}
EOF
