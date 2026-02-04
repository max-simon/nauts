# Static Mode Test Environment

Follow instructions for [Static Mode Test Environment](../static/README.md).

## Configuration

Create a JWT and keypair using Python script 

```bash
python3 generate_jwt.py APP.readonly AUTH.full SYS.full
```

Copy the public key (base64 encoded) and paste it to the identity provider configuration section in `nauts.json`:


```json
"identity": {
    "type": "jwt",
    "jwt": {
      "issuers": {
        "e2e": {
          "publicKey": "<issuer public key>",
          "accounts": ["APP", "AUTH"],
          "rolesClaimPath": "nauts.roles"
        }
      }
    }
  },
```

Save the JWT to file, e.g. `sample.jwt`.

## Test

```bash
# succeeds: has role APP.readonly
nats --token '{"account":"APP","token":"'$(cat sample.jwt)'"}' sub public.test

# fails: only has role APP.readonly, not role full
nats --token '{"account":"APP","token":"'$(cat sample.jwt)'"}' pub public.test fail

# succeeds: has role AUTH.full
nats --token '{"account":"AUTH","token":"'$(cat sample.jwt)'"}' pub public.test success

# fails: has role SYS.full, but issuer not allowed to manage SYS
nats --token '{"account":"SYS","token":"'$(cat sample.jwt)'"}' pub public.test fail
```