# nauts Specification Database

This directory contains the authoritative specifications for nauts components. Each spec describes the **public API** and **design decisions** of a component — not implementation details.

## Short summary

- These specs are the source of truth for component contracts (public APIs, invariants, error semantics).
- Change the spec when you change what callers can rely on.
- Prefer clarity and testable contracts over implementation detail.

## How to use (quick)

### When to write or update a spec

- **Before implementing**: capture the API and key design decisions.
- **When changing behavior/APIs**: update the spec in the same PR.
- **When deprecating**: mark **Status: Deprecated** and explain migration; don’t delete.

### Creating a new spec file

1. Pick a name: `YYYY-MM-DD-<component-name>.md`
2. Start from the template below.
3. Add the new spec to the index list in this README.

## Template (minimal, copy/paste)

Use this for new specs to keep structure consistent. Add extra sections only when they improve understanding.

```markdown
# Specification: <Component Name> (`<package>/`)

**Date:** YYYY-MM-DD  
**Status:** Current | Draft | Deprecated  
**Package:** `<package_name>`  
**Dependencies:** None | `pkg1`, `pkg2`

---

## Goal

[One sentence: why this exists]

## Summary

[Short paragraph: what it does and what it does NOT do]

## Scope

- [In-scope item]
- [In-scope item]

**Out of scope:** [Explicitly list non-goals]

## Public API

[Types, interfaces, functions, and error semantics callers rely on]

## Design decisions

| Decision | Rationale |
|----------|-----------|
| **[Decision]** | [Why] |

## Examples (optional)

[One small example that clarifies usage or edge cases]

## Known limitations / future work

- **[Limitation]**: [What’s missing] (planned: [idea] | unplanned)
```

## What are these specs?

Living documentation that serves two audiences:

1. **Code agents** — Get exact type signatures, interfaces, error conditions, and contracts without reading implementation code
2. **Humans** — Understand the "why" behind design decisions and how components fit together

Think of these as the "contract" each component promises to uphold. When you change a public API, update the spec. When you add a feature, document the design decision.

## How to use

### Reading order for new contributors

Start with the system overview, then work through standalone components before dependent ones:

1. **[system-overview](2026-02-06-system-overview.md)** — Dependency graph, end-to-end flow, deployment modes
2. **[policy-engine](2026-02-06-policy-engine.md)** — NRN, actions, interpolation, compilation (no dependencies)
3. **[jwt-issuance](2026-02-06-jwt-issuance.md)** — Signer interface, IssueUserJWT (depends on policy)
4. **[identity-authentication](2026-02-06-identity-authentication.md)** — User, AuthProvider, Manager (no dependencies)
5. **[providers](2026-02-06-providers.md)** — AccountProvider, PolicyProvider (depends on policy, jwt)
6. **[auth-controller-callout](2026-02-06-auth-controller-callout.md)** — AuthController, CalloutService (depends on all)
7. **[auth-debug-service](2026-02-12-auth-debug-service.md)** — Debug endpoint for auth pipeline introspection (Draft)
8. **[cli](2026-02-08-cli.md)** — Command-line interface (depends on auth)

### Additional Specifications (Draft/Proposed)

- **[aws-sigv4-authentication](2026-02-08-aws-sigv4-authentication.md)** — AWS IAM role-based authentication provider (Draft)
- **[control-plane](2026-02-11-control-plane.md)** — Angular web UI for policy and binding management in NATS KV (Draft)

### For code agents

When working on a feature:

1. Read the relevant component spec to understand the current API
2. Check the "Known Limitations / Future Work" section — your feature may be planned
3. Before implementing, update the spec with your design decisions
4. After implementing, update the spec with any new public APIs
5. Run `go test ./...` to ensure contracts are upheld

### For humans

- **Goal** section — one sentence: why does this component exist?
- **Summary** — paragraph: what does it do?
- **Design Decisions** table — the "why" behind non-obvious choices
- **Public API** — types, interfaces, functions you can depend on
- **Known Limitations** — what's missing and what's next

## Template (extended)

Use this when a component needs more structure (method tables, error catalogs, flows). For most new specs, start with the minimal template above and expand only as needed.

```markdown
# Specification: <Component Name> (`<package>/`)

**Date:** YYYY-MM-DD  
**Status:** Current | Deprecated | Draft  
**Package:** `<package_name>`  
**Dependencies:** None | `package1`, `package2`

---

## Goal

[One sentence explaining why this component exists]

## Summary

[Human-readable paragraph describing what the component does. Focus on the value it provides, not implementation details.]

---

## Scope

[Bullet list of what IS in scope]

**Out of scope:** [What is NOT in scope — helps set expectations]

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **[Decision name]** | [Why this choice was made, alternatives considered] |
| **[Decision name]** | [Why this choice was made] |

---

## Public API

### Types

#### `TypeName`
<markdown Go script>
[Brief description of what this type represents]

### Interfaces

#### `InterfaceName`

<markdown Go script>

| Method | Purpose |
|--------|---------|
| `Method` | [What it does] |

### Functions

| Function | Signature | Purpose |
|----------|-----------|---------|
| `FuncName` | `(args) (returns, error)` | [What it does] |

### Error Types

| Error | Sentinel | Meaning |
|-------|----------|---------|
| `ErrName` | ✓ | [When this error occurs] |

---

## [Component-Specific Sections]

[Add sections as needed for tables, diagrams, flows, examples]

---

## Known Limitations / Future Work

| Area | Limitation | Planned Enhancement |
|------|-----------|-------------------|
| [Feature area] | [Current limitation] | [What's planned] |

[Or use bullet format for simpler cases]

- **[Limitation]**: [Description]. [Planned fix if any].
```

## File naming convention

`YYYY-MM-DD-<component-name>.md`

- Date: When the spec was created or last major revision
- Component name: Lowercase, hyphen-separated, descriptive
- Examples: `2026-02-06-policy-engine.md`, `2026-02-06-system-overview.md`

## Implementation plans

Implementation plans for specifications should be stored in `specs/plans/` with the naming convention `<spec-name>-implementation-plan.md`. These plans break down the specification into executable tasks with estimates, dependencies, and testing requirements.

Example: `specs/plans/aws-sigv4-implementation-plan.md`

## Maintaining specs

### When to update a spec

- ✅ Adding a public function, type, or interface
- ✅ Changing a function signature
- ✅ Adding a new design decision or changing an existing one
- ✅ Deprecating a feature (mark as deprecated, don't delete)
- ✅ Implementing something from "Future Work"
- ❌ Internal refactoring that doesn't change the public API
- ❌ Bug fixes that don't change behavior

### How to update

1. **In-place updates** for existing components — edit the spec file directly
2. **New files** for new components — use the template above
3. **Deprecation notes** — mark sections as deprecated rather than deleting them
4. **System overview** — update the spec index table when adding/removing specs

### Status values

- **Current** — Active, maintained component
- **Draft** — In development, not yet implemented
- **Deprecated** — No longer recommended, marked for removal

## Philosophy

> "A spec should be just detailed enough that a code agent can implement the component without reading the existing implementation, but not so detailed that it becomes a maintenance burden."

Focus on:
- **Contracts** (what a component promises)
- **Decisions** (why it works this way)
- **Dependencies** (what it needs)

Avoid:
- Implementation details (algorithms, data structures)
- Exhaustive examples (one or two illustrative cases is enough)
- Repeating information from code comments (link to godoc instead)

## Questions?

If you're unsure whether something belongs in a spec, ask: "Would a code agent need to know this to use the component correctly?" If yes, add it. If no, leave it in code comments or implementation docs.
