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

# 4. Write nats-server.conf
cat > nats-server.conf <<EOF
server_name: nauts-test
jetstream {
  store_dir: /tmp/nauts-test-auth-jetstream
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

    APP {
        # No static users - all users are authenticated via nauts
    }

    APP2 {
        # No static users - all users are authenticated via nauts
    }

    SYS {
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
        # - sys user for system events
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

# generate RSA Keys for JWT Provider
if [ ! -f rsa.key ]; then
  openssl genpkey -algorithm RSA -out rsa.key -pkeyopt rsa_keygen_bits:2048
fi
openssl rsa -in rsa.key -pubout -out rsa.pem 2>/dev/null
cat rsa.pem | base64 | tr -d '\n' > rsa.pem.b64
echo "Generated rsa.key/pem"

# Suite-local fixtures
cat > policies.json <<EOF
[
  {
    "id": "app-worker",
    "account": "APP",
    "name": "APP worker",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:e2e.mytest"]
      }
    ]
  },
  {
    "id": "app-consumer",
    "account": "APP",
    "name": "APP consumer",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:e2e.mytest"]
      }
    ]
  },
  {
    "id": "app2-producer",
    "account": "APP2",
    "name": "APP2 producer",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:e2e.mytest"]
      }
    ]
  }
]
EOF

cat > bindings.json <<EOF
[
  { "role": "worker", "account": "APP", "policies": ["app-worker"] },
  { "role": "consumer", "account": "APP", "policies": ["app-consumer"] },
  { "role": "producer", "account": "APP2", "policies": ["app2-producer"] }
]
EOF

# Password hash for "secret": $2a$10$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi
cat > users.json <<EOF
{
  "users": {
    "alice": {
      "accounts": ["APP"],
      "roles": ["APP.consumer"],
      "passwordHash": "\$2a\$10\$yRjAZrdk2RhF0LB/Hf2b5./.06Alk8Zy1Pis8acjM298NPSTB/iwi"
    }
  }
}
EOF

RSA_PUB_B64=$(openssl pkey -in rsa.key -pubout | base64 | tr -d '\n');

cat > nauts.json <<EOF
{
  "account": {
    "type": "static",
    "static": {
      "publicKey": "$ISSUER_PUB",
      "privateKeyPath": "./account-AUTH.nk",
      "accounts": [
        "AUTH", "APP", "APP2"
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
        "id": "intro-file",
        "accounts": ["APP"],
        "userPath": "./users.json"
      }
    ],
    "jwt": [
      {
        "id": "intro-jwt",
        "accounts": ["APP", "APP2"],
        "issuer": "e2e",
        "rolesClaimPath": "nauts.roles",
        "publicKey": "$RSA_PUB_B64"
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
