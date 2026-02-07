#!/bin/bash
set -e

# generate server xkey
nk -gen curve > server-xkey.nk
XKEY_PUB=$(nk -inkey server-xkey.nk -pubout)
echo "Generated server-xkey.nk (XKey: $XKEY_PUB)"

# generate RSA Keys for JWT Provider
openssl genpkey -algorithm RSA -out rsa.key -pkeyopt rsa_keygen_bits:2048
openssl rsa -in rsa.key -pubout -out rsa.pem 2>/dev/null
cat rsa.pem | base64 | tr -d '\n' > rsa.pem.b64
echo "Generated rsa.key/pem"

# generate users.json
# single user: `alice`, password `secret`.`
cat> users.json << EOF
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

# generate policies.json
cat> policies.json << EOF
[
  {
    "id": "e2e-reader",
    "name": "E2E Reader",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.sub"],
        "resources": ["nats:e2e.>"]
      }
    ]
  },
  {
    "id": "e2e-writer",
    "name": "E2E Writer",
    "statements": [
      {
        "effect": "allow",
        "actions": ["nats.pub"],
        "resources": ["nats:e2e.>"]
      }
    ]
  }
]
EOF

# generate bindings.json
cat> bindings.json << EOF
[
  {
    "role": "consumer",
    "account": "APP",
    "policies": ["e2e-reader"]
  },
  {
    "role": "worker",
    "account": "APP",
    "policies": ["e2e-reader", "e2e-writer"]
  },
  {
    "role": "producer",
    "account": "APP2",
    "policies": ["e2e-writer"]
  },
  {
    "role": "consumer",
    "account": "APP2",
    "policies": ["e2e-reader"]
  }
]
EOF
