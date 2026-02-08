#!/bin/bash
set -e

#
# NATS Server Configuration
#
echo "Generating NATS Server configuration";

# Remove existing operator
export NSC_PATH=$(nsc env 2>&1 >/dev/null | grep -i "current store dir" | awk -F'|' '{print $4}' | xargs);
NSC_PATH="${NSC_PATH/#\~/$HOME}"
rm -rf "$NSC_PATH/nauts-test";

# Create operator
nsc add operator nauts-test --generate-signing-key --sys;
nsc edit operator --require-signing-keys;

# Create AUTH account with signing key
nsc add account --name AUTH;
nsc edit account AUTH --sk generate;

# Create APP account with signing key
nsc add account --name APP;
nsc edit account APP --sk generate;

# Create APP2 account with signing key
nsc add account --name APP2;
nsc edit account APP2 --sk generate;

# Create auth user for NAUTS
nsc add user --account AUTH --name auth;
nsc generate creds --account AUTH --name auth > auth.creds;
USER_AUTH_PUB=$(nsc describe user --account AUTH --name auth --json | jq -r .sub);

# Create dummy user for all clients
nsc add user --account AUTH --name dummy --deny-pubsub ">";
nsc generate creds --account AUTH --name dummy > dummy.creds;

# Enable auth callout in AUTH account
XKEY_PUB=$(nk -pubout -inkey ../common/server-xkey.nk);
nsc edit authcallout --account AUTH --curve $XKEY_PUB --auth-user $USER_AUTH_PUB --allowed-account '*';

# Write configuration
rm -f nats-server.conf;
nsc generate config --mem-resolver --config-file nats-server.conf;
cat >> nats-server.conf <<EOF

server_name: nauts-test
jetstream {
    store_dir: /tmp/nauts-test-connection-operator-jetstream
    max_mem: 128M
    max_file: 1G
}
EOF

#
# NAUTS Configuration
#
echo "Generating NAUTS Configuration...";

RSA_PUB_B64=$(cat ../common/rsa.pem.b64);

# write base config
cat > nauts.json <<EOF
{
  "account": {},
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
        "accounts": [
          "APP"
        ],
        "userPath": "../common/users.json"
      }
    ],
    "jwt": [
      {
        "id": "intro-jwt",
        "accounts": [
          "APP",
          "APP2"
        ],
        "issuer": "e2e",
        "rolesClaimPath": "nauts.roles",
        "publicKey": "$RSA_PUB_B64"
      }
    ]
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsCredentials": "./auth.creds",
    "xkeySeedFile": "../common/server-xkey.nk"
  }
}


EOF

# Get path to nkeys
export NKEYS_PATH=$(nsc env 2>&1 >/dev/null | grep -i nkeys | awk -F'|' '{print $4}' | xargs);
NKEYS_PATH="${NKEYS_PATH/#\~/$HOME}"

# utility function to export details for account
export_account() {
  local name=$1
  local seed_output="./sk-${name}.nk"
  
  # Get account public key
  local account_pub=$(nsc describe account --name "$name" --json | jq -r .sub)
  
  # Get first signing key public key
  local account_sk_pub=$(nsc describe account "$name" --json | jq -r '.nats.signing_keys[0]')
  
  # Find and copy seed file
  local seed_file=$(find "$NKEYS_PATH" -name "${account_sk_pub}.nk" -print -quit)
  
  if [ -f "$seed_file" ]; then
    cp "$seed_file" "$seed_output"
    echo "$name|$account_pub|$seed_output"
  else
    echo "Error: Seed for $name not found" >&2
    return 1
  fi
}

# Generate NAUTS account config
NAUTS_ACCOUNT_CONFIG=$(jq -n '{"type": "operator", "operator": {"accounts": {}}}')

# Iterate through all arguments passed to the script
for ACC_NAME in "AUTH" "APP" "APP2"; do
    # Get details (Format: NAME|PUBKEY|PATH)
    ACCOUNT_DATA=$(export_account "$ACC_NAME")
    
    if [ $? -eq 0 ]; then
        NAME=$(echo $ACCOUNT_DATA | cut -d'|' -f1)
        PUB=$(echo $ACCOUNT_DATA | cut -d'|' -f2)
        SK_PATH=$(echo $ACCOUNT_DATA | cut -d'|' -f3)

        # 3. Use jq to inject the account into the object dynamically
        NAUTS_ACCOUNT_CONFIG=$(echo "$NAUTS_ACCOUNT_CONFIG" | jq \
            --arg name "$NAME" \
            --arg pub "$PUB" \
            --arg path "$SK_PATH" \
            '.operator.accounts[$name] = {"publicKey": $pub, "signingKeyPath": $path}')
    else
        echo "Failed to export account: $ACC_NAME" >&2;
    fi
done

# write account config to nauts.json
jq --argjson account "$NAUTS_ACCOUNT_CONFIG" '.account = $account' nauts.json > nauts.tmp && mv nauts.tmp nauts.json;
