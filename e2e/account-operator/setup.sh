#!/bin/bash

#
# NATS Server Configuration
#
echo "Generating NATS Server configuration";

# Suite-local xkey seed (used for encrypted auth callout)
if [ ! -f server-xkey.nk ]; then
  nk -gen curve > server-xkey.nk
fi
XKEY_PUB=$(nk -pubout -inkey ./server-xkey.nk);

# Remove existing operator
# export NSC_PATH=$(nsc env 2>&1 >/dev/null | grep -i "current store dir" | awk -F'|' '{print $4}' | xargs);
# NSC_PATH="${NSC_PATH/#\~/$HOME}"
# rm -rf "$NSC_PATH/nauts-test";

# Create operator
nsc describe operator --name nauts-test &> /dev/null;
if [ $? -ne 0 ]; then
  nsc add operator nauts-test --generate-signing-key --sys;
  nsc edit operator --require-signing-keys;
fi
nsc select operator nauts-test;

# Create AUTH account with signing key
nsc describe account --name AUTH &> /dev/null;
if [ $? -ne 0 ]; then
  nsc add account --name AUTH;
  nsc edit account AUTH --sk generate;
  # Create auth user for NAUTS
  nsc add user --account AUTH --name auth;
  # Create dummy user for all clients
  nsc add user --account AUTH --name dummy --deny-pubsub ">";
  # Enable auth callout in AUTH account
  USER_AUTH_PUB=$(nsc describe user --account AUTH --name auth --json | jq -r .sub);
  nsc edit authcallout --account AUTH --curve $XKEY_PUB --auth-user $USER_AUTH_PUB --allowed-account '*';
fi

# Create APP account with signing key
nsc describe account --name APP &> /dev/null;
if [ $? -ne 0 ]; then
  nsc add account --name APP;
  nsc edit account APP --sk generate;
fi

# Save creds
nsc generate creds --account AUTH --name auth > auth.creds;
USER_AUTH_PUB=$(nsc describe user --account AUTH --name auth --json | jq -r .sub);
nsc generate creds --account AUTH --name dummy > dummy.creds;


# Write configuration
rm -f nats-server.conf;
nsc generate config --mem-resolver --config-file nats-server.conf;
cat >> nats-server.conf <<EOF

server_name: nauts-test
jetstream {
  store_dir: /tmp/nauts-test-account-operator-jetstream
    max_mem: 128M
    max_file: 1G
}
EOF

#
# NAUTS Configuration
#
echo "Generating NAUTS Configuration...";

# Suite-local fixtures
cat > policies.json <<EOF
[
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
  }
]
EOF

cat > bindings.json <<EOF
[
  { "role": "consumer", "account": "APP", "policies": ["app-consumer"] }
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

cat > nauts.json <<EOF
{
  "account": {},
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
    ]
  },
  "server": {
    "natsUrl": "nats://localhost:4222",
    "natsCredentials": "./auth.creds",
    "xkeySeedFile": "./server-xkey.nk"
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
for ACC_NAME in "AUTH" "APP"; do
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
