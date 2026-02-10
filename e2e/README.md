# End-to-End Tests

This directory contains end-to-end integration tests for `nauts`. These tests spin up a real NATS server and a `nauts` service process, then run NATS clients against the setup to verify authentication and authorization flows.

## Structure

Suites are split by concern. Each suite folder is self-contained and includes a `setup.sh` that generates:
- `nats-server.conf`
- `nauts.json`
- `policies.json` / `bindings.json`
- `users.json`
- any required keys/creds (e.g. issuer/account keys, xkeys)

Edit `setup.sh` (not the generated JSON files) when changing fixtures.

*   `account-static/`: Static account provider suite environment.
*   `account-operator/`: Operator account provider suite environment.
*   `auth/`: Authentication provider suite environment (file/jwt/aws).
*   `policy-nats/`: Policy engine suite for Core NATS actions.
*   `policy-jetstream/`: Policy engine suite for JetStream actions.
*   `policy-kv/`: Policy engine suite for KV actions.

The harness lives in `env.go` and is responsible for starting/stopping `nats-server` and `nauts` per suite.

## Policy Engine Tests

The policy engine tests (`TestPolicies*`) verify that the `nauts` authorization logic correctly enforces permissions defined in `POLICY.md`.

**Note:** Each suite uses its own JetStream `store_dir` under `/tmp/nauts-test-<suite>-jetstream`. The test harness (`env.go`) removes that directory before starting the suite to ensure isolation.

### Users and Roles

Users/roles are suite-specific and intentionally minimal. See each suite's `setup.sh` for the exact fixture definitions.

Current policy suites focus on:
- `policy-nats/`: core NATS actions with variable-scoped subjects using `{{ user.id }}` and `{{ role.name }}`.
- `policy-jetstream/`: JetStream actions including explicit coverage for consumer wildcard `*`.
- `policy-kv/`: KV actions including key wildcard `>` and a user-scoped key prefix test with `{{ user.id }}.>`.

### Test Coverage

#### Core NATS (`policy-nats/`)
*   **`nats.pub`**: publish allowed only on the user/role-scoped subject.
*   **`nats.sub`**: subscribe allowed only on the user/role-scoped subject.
*   **`nats.service`**: service can listen and respond on the scoped subject.
*   **Variables**: exercises `{{ user.id }}` and `{{ role.name }}` interpolation.

#### JetStream (`policy-jetstream/`)
*   **`js.manage`**: can create/update/purge/delete an allowed stream.
*   **`js.view`**: can view stream/consumer info without gaining data read permissions.
*   **`js.consume`**: covers both a specific consumer and consumer wildcard `*`.

#### Key-Value (`policy-kv/`)
*   **`kv.manage`**: create/delete an allowed bucket.
*   **`kv.view`**: view bucket status without read/write access to keys.
*   **`kv.read`**: read any key via `>` wildcard and watch updates.
*   **`kv.edit`**: edit keys via `>` wildcard.
*   **Variables**: user-scoped read to `{{ user.id }}.>` within a shared bucket.

## Running the Tests

Ensure `nats-server`, `nk`, and `go` are installed.

The E2E harness builds `bin/nauts` automatically (once per `go test` run), so you usually don't need to build it manually.

1.  **Generate suite configuration:**
    ```bash
    for d in account-static account-operator auth policy-nats policy-jetstream policy-kv; do
      (cd e2e/$d && ./setup.sh)
    done
    ```

2.  **Run all e2e tests:**
    ```bash
    go test -v ./e2e/...
    ```

3.  **Run only policy tests:**
    ```bash
    go test -v ./e2e/... -run TestPolicies
    ```

## AWS SigV4 (Optional)

The AWS SigV4 auth test is best-effort and will automatically `Skip` if token generation fails (missing credentials, cannot assume role, etc).

- Override the AWS profile used by setting `NAUTS_E2E_AWS_PROFILE`.

## Legacy Environments

These folders are kept as historical references for previous layouts and may be removed later:

*   `connection-static/`
*   `connection-operator/`
*   `policy-static/`

### Ports

Suites use fixed localhost ports by default (e.g. 4222-4224, 4230-4232). If you see timeouts, make sure those ports are free.

## Troubleshooting

*   **Timeouts**: Integration tests rely on real network connections (localhost). Ensure ports 4222 (static) and 4223 (operator) are free.
*   **Bcrypt hashes**: If you edit `setup.sh`, remember `$` is special in shell heredocs. Use `\$2a\$...` (as shown in the suite scripts) so generated `users.json` contains a literal bcrypt hash.
