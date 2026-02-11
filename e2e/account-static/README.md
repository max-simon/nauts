# NATS Static Mode Test Environment

This directory contains a standalone NATS and Nauts setup for testing the **Static Account Mode** and **Auth Callout**.

In this mode:
- Nauts acts as an auth callout service.
- NATS Server delegates authentication to Nauts.
- Nauts verifies credentials and issues a User JWT signed by a static Account Key.

## Prerequisites

- [nk](https://github.com/nats-io/nkeys/tree/main/nk) - NATS key generator
- [jq](https://stedolan.github.io/jq/) - Command-line JSON processor
- [openssl](https://www.openssl.org/) - For RSA key generation

## Setup

Run the setup script to generate fresh keys and update configuration files:

```bash
./setup.sh
```

This will generate:
- `account-AUTH.nk`: The Account Key acting as the Issuer
- `user-auth.nk`: The User Key for the Nauts service
- `nats-server.conf`: Updated NATS server config
- `nauts.json`: Updated Nauts config

## Usage

1. Start the NATS Server:
   ```bash
   nats-server -c nats-server.conf
   ```

2. Start Nauts (in a separate terminal):
   ```bash
   # Run from repository root
   go run cmd/nauts/main.go serve -config e2e/account-static/nauts.json
   ```

## Key Files

| File | Type | Purpose |
|------|------|---------|
| `account-AUTH.nk` | `user` | **Issuer**. Signs the User JWTs returned by Nauts. |
| `user-auth.nk` | `user` | **Service User**. Nauts uses this to connect to the `AUTH` account to listen for requests. |

