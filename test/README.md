# End-to-End Tests

This directory contains end-to-end integration tests for `nauts`. These tests spin up a real NATS server and a `nauts` service process, then run NATS clients against the setup to verify authentication and authorization flows.

## Structure

*   `common/`: Shared setup scripts and configuration (keys, policies, users).
*   `connection-static/`: Configuration for "Static Mode" (single key issuer) tests.
*   `connection-operator/`: Configuration for "Operator Mode" (NSC-managed operator/account) tests.
*   `env.go`: Test environment harness that manages process lifecycles.
*   `connection_test_procedure.go`: The actual test logic (Alice/Bob scenarios).
*   `connection_test.go`: Go test entry points (TestMain wrappers).

## Prerequisites

To run these tests, you need the following tools installed and available in your `$PATH`:

*   `go`
*   `nats-server` (must be installed for the tests to run the server)
*   `nsc` (NATS Account Server tool)
*   `nk` (NATS Key tool)
*   `jq` (Command-line JSON processor)
*   `openssl` (for RSA key generation)

## Setup

Before running tests for the first time or after a clean, you must generate the necessary configuration files and keys.

1.  **Common Setup (Dependencies for both modes):**
    ```bash
    cd test/common
    ./setup.sh
    cd ../..
    ```

2.  **Static Mode Setup:**
    ```bash
    cd test/connection-static
    ./setup.sh
    cd ../..
    ```

3.  **Operator Mode Setup:**
    ```bash
    cd test/connection-operator
    ./setup.sh
    cd ../..
    ```

## Build

The tests expect the `nauts` binary to be present in `bin/nauts`.

```bash
go build -o bin/nauts ./cmd/nauts
```

## Running Tests

Tests are integrated into the standard `go test` workflow, but rely on flags to enable specific modes (to avoid running heavy integration tests by default or when not set up).

### Static Mode Tests

Verifies `nauts` functioning with a static account provider configuration.

```bash
go test -v ./test/ -static
```

### Operator Mode Tests

Verifies `nauts` functioning with an operator account provider configuration (using `nsc` generated entities).

```bash
go test -v ./test/ -operator
```

### All Tests

```bash
go test -v ./test/ -static -operator
```

## Test Coverage

### Connect

The connection test procedure (`connection_test_procedure.go`) verifies:

1.  **Authentication**:
    *   Successful login with username/password (File Provider).
    *   Successful login with JWT (JWT Provider).
    *   Failed login with incorrect password.
    *   Failed login against wrong provider.

2.  **Authorization**:
    *   **Permission Enforcement**: Alice (role `APP.consumer`) cannot publish to `e2e.mytest`.
    *   **Pub/Sub Flow**: Bob (role `APP.worker`) publishes to `e2e.mytest`, Alice subscribes and receives the message.

## Troubleshooting

*   **Timeouts**: Integration tests rely on real network connections (localhost). Ensure ports 4222 (static) and 4223 (operator) are free.
