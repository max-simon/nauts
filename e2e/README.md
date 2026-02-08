# End-to-End Tests

This directory contains end-to-end integration tests for `nauts`. These tests spin up a real NATS server and a `nauts` service process, then run NATS clients against the setup to verify authentication and authorization flows.

## Structure

*   `policy-static/`: Configuration for policy engine tests.
*   `common/`: Shared setup scripts and keys.
*   `env.go`: Test environment harness that manages process lifecycles.
*   `policy_nats_test.go`: Tests for Core NATS actions.
*   `policy_jetstream_test.go`: Tests for JetStream actions.
*   `policy_kv_test.go`: Tests for KV actions.

## Policy Engine Tests

The policy engine tests (`TestPolicies*`) verify that the `nauts` authorization logic correctly enforces permissions defined in `POLICY.md`. The setup is defined in `e2e/policy-static/setup.sh` which generates a self-contained environment with users, policies, and roles.

**Note:** The setup script configures the NATS server to use a local `./jetstream` directory for state. The test harness (`env.go`) cleans this directory before every test run to ensure isolation.

### Users and Roles

The test setup creates the following users and assigns them to specific roles:

| User | Role | Description |
|---|---|---|
| `admin` | `admin` | **Superuser**. Full access (`*`) to all resources (`nats`, `js`, `kv`). |
| `writer` | `writer` | **Publisher/Editor**. Can publish to `public.>`, manage `STREAM_WRITE`, and edit `BUCKET_WRITE`. |
| `reader` | `reader` | **Subscriber/Viewer**. Can subscribe to `public.>`, view `STREAM_WRITE`, consume from `CONSUMER_READ`, and read `BUCKET_READ`. |
| `service` | `service` | **Service**. specific permission to handle requests on `service.>`. |
| `alice` | `user` | **Regular User**. Uses **variable interpolation**. Isolated access to `nats:user.alice.>`, `kv:USER_alice`, etc. |
| `bob` | `user` | **Regular User**. Isolated access to `nats:user.bob.>`, etc. |

### Test Coverage

#### Core NATS (`policy_nats_test.go`)
*   **`nats.pub`**: Verified that `writer` can publish to public subjects but not private ones.
*   **`nats.sub`**: Verified that `reader` can subscribe to public subjects but not publish.
*   **`nats.service`**: Verified that `service` user can implement a request-reply service but cannot subscribe to unrelated subjects.
*   **Variables**: Verified that `alice` cannot publish/subscribe to `bob`'s subjects.

#### JetStream (`policy_jetstream_test.go`)
*   **`js.manage`**: Verified that `writer` can update/purge streams it manages, but not others.
*   **`js.view`**: Verified that `reader` can view stream info but cannot create consumers on read-only streams.
*   **`js.consume`**: Verified that `reader` can consume from a specifically permitted consumer, but cannot create arbitrary new consumers.
*   **Variables**: Verified that `alice` cannot consume from `bob`'s permitted consumer.

#### Key-Value (`policy_kv_test.go`)
*   **`kv.edit`**: Verified that `writer` can put, get, and delete keys.
*   **`kv.read`**: Verified that `reader` can get values but fails to put (write).
*   **`kv.view`**: Verified that users with view-only or no access cannot modify bucket data.
*   **Variables**: Verified that `alice` can write to her personal bucket `USER_alice`.

## Running the Tests

Ensure `nats-server`, `nk`, and `go` are installed.

1.  **Regenerate Configuration (if needed):**
    ```bash
    cd e2e/policy-static
    ./setup.sh
    cd ../..
    ```

2.  **Run All Tests:**
    ```bash
    go test -v ./e2e/...
    ```

3.  **Run Specific Policy Tests:**
    ```bash
    go test -v ./e2e/ -run TestPolicies
    ```

## Old Connection Tests

*   `connection-static/`: Configuration for "Static Mode" (single key issuer) tests.
*   `connection-operator/`: Configuration for "Operator Mode" (NSC-managed operator/account) tests.
*   `connection_test_procedure.go`: The actual test logic (Alice/Bob scenarios).
*   `connection_test.go`: Go test entry points (TestMain wrappers).

### Prerequisites

To run these tests, you need the following tools installed and available in your `$PATH`:

*   `go`
*   `nats-server` (must be installed for the tests to run the server)
*   `nsc` (NATS Account Server tool)
*   `nk` (NATS Key tool)
*   `jq` (Command-line JSON processor)
*   `openssl` (for RSA key generation)

### Setup

Before running tests for the first time or after a clean, you must generate the necessary configuration files and keys.

1.  **Common Setup (Dependencies for both modes):**
    ```bash
    cd e2e/common
    ./setup.sh
    cd ../..
    ```

2.  **Static Mode Setup:**
    ```bash
    cd e2e/connection-static
    ./setup.sh
    cd ../..
    ```

3.  **Operator Mode Setup:**
    ```bash
    cd e2e/connection-operator
    ./setup.sh
    cd ../..
    ```

### Build

The tests expect the `nauts` binary to be present in `bin/nauts`.

```bash
go build -o bin/nauts ./cmd/nauts
```

## Running Tests

Tests are integrated into the standard `go test` workflow, but rely on flags to enable specific modes (to avoid running heavy integration tests by default or when not set up).

### Static Mode Tests

Verifies `nauts` functioning with a static account provider configuration.

```bash
cd e2e
go test -v . -run TestNatsConnectionStatic
# OR just run all
go test -v .
```

### Operator Mode Tests

Verifies `nauts` functioning with an operator account provider configuration (using `nsc` generated entities).

```bash
cd e2e
go test -v . -run TestNatsConnectionOperator
```

### Policy Engine Tests

Verify specific policy behaviors (Split into NATS, JetStream, and KV domains).

```bash
cd e2e
go test -v . -run TestPolicies
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
