# Specification: Control Plane for Policies & Bindings (Angular Web UI)

**Date:** 2026-02-12
**Status:** Draft
**Package:** `control-plane` (Angular SPA)
**Dependencies:** NATS WebSocket, NATS KV (JetStream), `NatsPolicyProvider` key schema, Angular Material (M3)

---

## Goal

Provide a browser-based control plane for managing policies and bindings stored in the NATS KV bucket used by `NatsPolicyProvider`.

## Summary

The control plane is an Angular single-page application that connects to NATS via WebSocket and reads/writes policy and binding records from a shared KV bucket. It provides two main pages: **Policies** and **Bindings**. Users can list, search, filter, view details, and perform CRUD operations. The UI directly manages KV entries and uses optimistic concurrency via KV revision numbers.

## UI Framework & Design System

- **Component library:** Angular Material (`@angular/material`)
- **Design system:** Material Design 3 (M3)
- **Theming:** M3 baseline theme (default Angular Material M3 palette); customizable later via `@angular/material` theme API
- **Deployment:** Static files (`dist/`), served independently by any web server (nginx, CDN, etc.)
- **Key components used:**
  - `mat-toolbar` — top navigation bar with app title and account selector
  - `mat-sidenav` / `mat-nav-list` — side navigation between Policies and Bindings pages
  - `mat-table` — list views for policies and bindings with sorting
  - `mat-form-field`, `mat-input`, `mat-select` — form controls and filters
  - `mat-card` — details panel content
  - `mat-dialog` — confirmation dialogs (delete) and create/edit forms
  - `mat-chip` — action and resource tags in policy details
  - `mat-snack-bar` — transient notifications (save success, conflict warnings)
  - `mat-progress-bar` / `mat-spinner` — loading indicators
  - `mat-icon` — iconography throughout the UI
  - `mat-button`, `mat-fab` — action buttons (create, save, delete)

---

## Scope

- Angular SPA with two primary pages: Policies and Bindings
- WebSocket connection to NATS using browser-compatible client
- CRUD operations for policies and bindings stored in NATS KV
- Account-level filtering and search by name/id
- Details panel that opens on selection
- Optimistic concurrency using KV revisions

**Out of scope:**
- Server-side proxy or API layer
- Authentication and authorization for the control plane (delegated to NATS and deployment)
- Audit logging and change history
- Policy compilation preview or simulation
- Role or account management (only policies/bindings)

---

## User Experience Requirements

### 1) Policies Page

- **List view**: table of policies scoped to the selected account (plus global policies if enabled)
- **Filters**:
  - Account selector (required)
  - Search by policy `id` or `name`
- **Details panel** (right side):
  - Policy metadata: `id`, `name`, `account`
  - Statements list with `effect`, `actions`, `resources`
  - Display validation errors if policy is invalid or incomplete
- **Actions**:
  - Create policy
  - Edit policy
  - Delete policy

### 2) Bindings Page

- **List view**: table of bindings scoped to the selected account
- **Filters**:
  - Account selector (required)
  - Policy filter (policies applied by binding)
  - Role filter
- **Details panel** (right side):
  - Binding metadata: `role`, `account`, `policies`
- **Actions**:
  - Create binding
  - Edit binding
  - Delete binding

---

## Data Model (UI)

### Policy

Aligned with `policy.Policy` JSON schema (same as file policy provider):

```json
{
  "id": "read-access",
  "account": "APP",
  "name": "Read Access",
  "statements": [
    {
      "effect": "allow",
      "actions": ["nats.sub"],
      "resources": ["nats:public.>"]
    }
  ]
}
```

### Binding

Aligned with binding schema used by `NatsPolicyProvider`:

```json
{
  "role": "readonly",
  "account": "APP",
  "policies": ["read-access"]
}
```

---

## Storage Contract (NATS KV)

The control plane **must** use the same KV key schema as `NatsPolicyProvider`.

### KV Bucket

- Bucket name is configured in the control plane
- Bucket must already exist and be writable

### Key Schema

- **Policy keys:** `<account>.policy.<policy-id>`
- **Binding keys:** `<account>.binding.<role>`

**Examples:**
```
APP.policy.read-access
APP.binding.readonly
```

### Global Policies

Global policies use the `_global` account segment. Example:
```
_global.policy.base-permissions
```

---

## NATS Connection

### Transport

- WebSocket connection to NATS (e.g., `wss://nats.example.com:443`)
- Browser-compatible NATS client (nats.js)

### Connection Settings

- URL, credentials, and TLS settings are provided to the UI via configuration
- Supported auth: **token** and **NKey** (no encryption required)
- The UI uses JetStream KV APIs over the WebSocket connection

### Required Permissions

The control plane NATS credentials must allow:
- `GET` and `WATCH` access for KV bucket keys
- `PUT` and `DELETE` access for KV bucket keys

---

## CRUD Operations and Concurrency

### Read

- List keys by prefix for the selected account
- Fetch values and decode JSON
- Display validation errors if decoding or validation fails

### Create / Update

- Use KV `Put` with optimistic concurrency:
  - On create: require key to not exist
  - On update: use `ExpectedLastSubjSeq` / KV revision
- Validate JSON before write
- After write, refresh list and details panel

### Delete

- Use KV `Delete` with expected revision
- Confirm in UI before delete

---

## Validation Rules (UI)

### Policy Validation

- `id`: required, non-empty, unique within account
- `account`: required
- `statements`: at least one
- `effect`: must be `allow`
- `actions`: non-empty array
- `resources`: non-empty array

### Binding Validation

- `role`: required, non-empty
- `account`: required
- `policies`: non-empty array of policy IDs

---

## Error Handling

- **Connection errors**: show banner with retry action
- **KV conflicts**: show "Update conflict" and prompt to reload
- **Validation errors**: inline form errors
- **Not found**: show stale selection warning, remove from list

---

## Configuration

The control plane is configured via a build-time or runtime JSON config (exact mechanism is TBD):

```json
{
  "nats": {
    "url": "wss://nats.example.com:443",
    "credentials": "<jwt or token>",
    "bucket": "nauts-policies"
  },
  "ui": {
    "defaultAccount": "APP",
    "showGlobalPolicies": true
  }
}
```

---

## Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Direct NATS KV access from browser** | Avoids building a separate backend API and keeps the control plane aligned with the provider’s storage contract. |
| **Same key schema as `NatsPolicyProvider`** | Ensures changes are immediately visible to the service without translation. |
| **Optimistic concurrency** | Prevents silent overwrites when multiple admins edit the same policy. |
| **Two-page layout with details panel** | Supports fast browsing while keeping the editing context visible. |
| **Angular Material with M3 theming** | Official Angular component library with Material Design 3 support. Provides accessible, consistent UI components out of the box. M3 baseline theme avoids custom design work upfront. |
| **Static file deployment** | Decouples the SPA from the Go service. Can be served by nginx, CDN, or any static file host. Simplifies CI/CD and avoids coupling frontend releases to backend. |

---

## Resolved Decisions

1. **Global policy key prefix**: Use `_global.policy.<id>` for global policies.
2. **Account list source**: Discover accounts by KV key scan (prefix-based listing).
3. **NATS authentication in browser**: Support **token** and **NKey** auth. No encryption is required.
4. **Multi-tenant access control**: Rely entirely on NATS permissions (no client-side enforcement).
5. **Policy ID immutability**: Policy `id` is a UUID created on creation and never changes.
6. **Validation source of truth**: UI enforces the same validation rules as the server-side policy validator.
7. **Conflict UX**: On KV revision conflicts, force reload.
8. **Pagination**: Load all items at once for now; pagination can be added later.
9. **Bindings policy references**: Allow dangling references, but show warnings in the UI.
10. **Audit trail**: Not required.

## Open Questions / Ambiguities

None.

---

## Known Limitations / Future Work

- **No server-side API**: All operations occur directly against NATS KV.
- **No role/account management**: Only policies and bindings are editable.
- **No history/versioning**: KV history is not surfaced in UI.
- **No policy simulation**: UI does not compile or simulate permissions.
- **No authentication UX**: Login/identity flows are deployment-specific.
