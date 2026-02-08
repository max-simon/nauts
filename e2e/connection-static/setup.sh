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
XKEY_PUB=$(nk -inkey ../common/server-xkey.nk -pubout);

# 4. Write nats-server.conf
cat > nats-server.conf <<EOF
server_name: nauts-test
jetstream {
    store_dir: /tmp/nauts-test-connection-static-jetstream
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

RSA_PUB_B64=$(cat ../common/rsa.pem.b64);

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
      "policiesPath": "../common/policies.json",
      "bindingsPath": "../common/bindings.json"
    }
  },
  "auth": {
    "file": [
      {
        "id": "intro-file",
        "accounts": ["APP"],
        "userPath": "../common/users.json"
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
    ],
    "aws": [
      {
        "id": "intro-aws",
        "accounts": ["APP"],
        "awsAccount": "753923037904"
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
