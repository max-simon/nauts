#!/bin/bash
set -e

# Setup script for nauts-static-mode test environment
# Generates all necessary keys and updates configuration files

echo "Generating keys..."

# 1. Account Key (Issuer)
# In static mode, this single key identifies the issuer for all accounts
nk -gen user > account-AUTH.nk
ISSUER_PUB=$(nk -inkey account-AUTH.nk -pubout)
echo "Generated account-AUTH.nk (Issuer: $ISSUER_PUB)"

# 2. Auth Service User
# The nauts process uses this credentials to connect to NATS
nk -gen user > user-auth.nk
AUTH_USER_PUB=$(nk -inkey user-auth.nk -pubout)
echo "Generated user-auth.nk (Auth User: $AUTH_USER_PUB)"

# 3. System User
# For sys account
nk -gen user > user-sys.nk
SYS_USER_PUB=$(nk -inkey user-sys.nk -pubout)
echo "Generated user-sys.nk (Sys User: $SYS_USER_PUB)"

# 4. Server XKey
# For encrypted auth callout
nk -gen curve > server-xkey.nk
XKEY_PUB=$(nk -inkey server-xkey.nk -pubout)
echo "Generated server-xkey.nk (XKey: $XKEY_PUB)"

echo "Updating configuration..."

# Update nauts.json using jq
# - Update account verification key
# - Update RSA provider public key
jq --arg issuer "$ISSUER_PUB" \
   --arg rsa "$RSA_PUB_B64" \
   '.account.static.publicKey = $issuer | .auth.jwt[0].publicKey = $rsa' \
   nauts.json > nauts.json.tmp && mv nauts.json.tmp nauts.json

# Regenerate nats-server.conf
cat > nats-server.conf <<EOF
# NATS Server Configuration for nauts auth callout testing

# Server settings
server_name: nauts-test
port: 4222
http_port: 8222

# Logging
debug: false
trace: false
logtime: true

# Account definitions
accounts {
    
    # AUTH account - used by the nauts auth callout service
    AUTH {
        users: [
            { nkey: $AUTH_USER_PUB }
        ]
    }

    # APP account - target account for authenticated users
    APP {
        # No static users - all users are authenticated via nauts
    }

    # APP2 account
    APP2 {
    }

    # SYS account for system events
    SYS {
        users: [
            { nkey: $SYS_USER_PUB }
        ]
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
            $AUTH_USER_PUB, 
            $SYS_USER_PUB 
        ]

        # Account where auth callout service connects
        account: AUTH

        # XKey for encrypted auth callout
        xkey: $XKEY_PUB
    }
}

# JetStream configuration (for testing js.* and kv.* actions)
jetstream {
    store_dir: /tmp/nauts-test-jetstream
    max_mem: 128M
    max_file: 1G
}
EOF

echo "Done. Test environment is ready."
