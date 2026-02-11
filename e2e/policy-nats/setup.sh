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
  store_dir: /tmp/nauts-test-policy-nats-jetstream
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
    "name": "Admin Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.*"],
        "resources": ["nats:>"]
      }
    ]
  },
  {
    "id": "pub",
    "account": "POLICY",
    "name": "Publish Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:pub.{{ user.id }}.{{ role.id }}"]
      }
    ]
  },
  {
    "id": "sub",
    "account": "POLICY",
    "name": "Subscribe Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:sub.*"]
      }
    ]
  },
  {
    "id": "svc",
    "account": "POLICY",
    "name": "Service Policy",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.service"],
        "resources": ["nats:svc.{{ role.id }}"]
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
    "role": "pub",
    "account": "POLICY",
    "policies": ["pub"]
  },
  {
    "role": "sub",
    "account": "POLICY",
    "policies": ["sub"]
  },
  {
    "role": "service",
    "account": "POLICY",
    "policies": ["svc"]
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
    "pubber": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.pub"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "subber": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.sub"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    },
    "service": {
      "accounts": ["POLICY"],
      "roles": ["POLICY.service"],
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
