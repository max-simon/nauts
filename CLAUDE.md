# CLAUDE.md

This file provides context to Claude Code for working on the nauts project.

## Project Overview

nauts (**N**ATS **Aut**hentication **S**ervice) is a framework for scalable, human-friendly permission management for NATS. It provides:

- **Policy specification and compilation engine**: Translates high-level policies to low-level NATS permissions
- **Authentication service**: NATS auth callout implementation
- **Control plane**: Management API for policies, groups, and accounts (future)

See [README.md](./README.md) for architecture and [POLICY.md](./POLICY.md) for policy specification.

## Tech Stack

- **Language**: Go 1.22+
- **NATS Client**: github.com/nats-io/nats.go
- **JWT Handling**: github.com/nats-io/jwt/v2
- **NKeys**: github.com/nats-io/nkeys
- **Testing**: Standard library + github.com/stretchr/testify
- **Configuration**: Environment variables + JSON files

## Project Structure

```
nauts/
├── cmd/
│   └── nauts/           # Main entrypoint
├── internal/
│   ├── policy/          # Policy parsing, validation, compilation
│   ├── nrn/             # NRN (naut resource name) parsing
│   ├── auth/            # Auth callout service
│   ├── identity/        # User identity resolvers
│   ├── store/           # Policy/group storage backends
│   └── compile/         # Permission compilation engine
├── pkg/
│   └── nauts/           # Public API (if needed)
├── testdata/            # Test fixtures (policies, groups, users)
└── docs/                # Additional documentation
```

## Go Conventions

### Code Style

- Follow standard Go conventions (gofmt, go vet, staticcheck)
- Use meaningful variable names; avoid single letters except in loops/short lambdas
- Prefer early returns over deep nesting
- Group imports: stdlib, external, internal

### Error Handling

- Wrap errors with context: `fmt.Errorf("compiling policy %s: %w", id, err)`
- Define sentinel errors for expected failure modes
- Use custom error types when callers need to inspect error details

### Naming

- NRN types: `NRN`, `NRNType`, `ParseNRN()`
- Policy types: `Policy`, `Statement`, `Effect`
- Actions: `Action`, `ActionGroup`, `ResolveActions()`
- Compilers: `Compiler`, `CompileResult`, `Permission`

### Testing

- Table-driven tests for parsing and compilation
- Test files alongside implementation: `policy.go` + `policy_test.go`
- Use testdata/ for fixture files
- Aim for >80% coverage on core logic (policy, nrn, compile packages)

## Common Commands

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run linter
golangci-lint run

# Build binary
go build -o bin/nauts ./cmd/nauts

# Run with file-based config
./bin/nauts --config config.json
```

## Key Implementation Notes

### NRN Parsing

NRNs follow pattern `<type>:<identifier>[:<sub-identifier>]`. Parser must:
- Validate type is one of: `nats`, `js`, `kv`
- Validate wildcards (`*`, `>`) are only in allowed positions per type
- Handle variable interpolation `{{ var.path }}`

### Variable Interpolation

- Use a simple template engine (text/template or custom)
- Sanitize interpolated values: reject `*`, `>`, empty strings
- Allow only: `[a-zA-Z0-9_-]+`
- Return `null` (skip resource) on validation failure

### Permission Compilation

The compiler transforms policies to NATS permissions:
1. Resolve user's groups (including default group)
2. Collect all policies from groups
3. Expand action groups to atomic actions
4. Interpolate variables in NRNs
5. Map actions + NRNs to NATS PUB/SUB permissions
6. Deduplicate and merge permissions
7. Add implicit permissions (`_INBOX.>` for JS/KV)

### Auth Callout

The auth service receives NATS auth requests and:
1. Extracts credentials from the request
2. Resolves identity via configured resolver
3. Compiles permissions for the user
4. Signs and returns a NATS JWT

## Dependencies to Consider

```go
require (
    github.com/nats-io/nats.go v1.x
    github.com/nats-io/jwt/v2 v2.x
    github.com/nats-io/nkeys v0.x
    github.com/stretchr/testify v1.x
    golang.org/x/crypto v0.x  // for bcrypt
)
```

## Current Status

- [ ] Policy specification: Defined in POLICY.md
- [ ] NRN parser: Not implemented
- [ ] Policy parser: Not implemented
- [ ] Compilation engine: Not implemented
- [ ] File-based store: Not implemented
- [ ] Static identity resolver: Not implemented
- [ ] Auth callout service: Not implemented
- [ ] NATS KV store: Future
- [ ] JWT identity resolver: Future
- [ ] Control plane: Future
