# Specification: Control Plane for Policies & Bindings (Angular Web UI)

**Date:** 2026-02-12
**Updated:** 2026-02-16
**Status:** Implemented
**Package:** `ctrlp/` (Angular SPA)
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

**Implemented Features:**
- Angular SPA with three primary pages: **Policies**, **Bindings**, and **Simulator**
- WebSocket connection to NATS using browser-compatible client (nats.ws)
- CRUD operations for policies and bindings stored in NATS KV
- Account-level filtering and search by name/id
- 50/50 split layout with list on left and details panel on right
- Optimistic concurrency using KV revisions
- **Simulator page** for testing permission compilation via NATS debug service
- **Compiled statements view** showing aggregated permissions from multiple policies
- **Bucket import/export** for backup and restore of entire KV bucket
- **Global policy support** with `_global:` prefix convention
- **Key mismatch validation** warnings for policies and bindings

**Out of scope:**
- Server-side proxy or API layer
- Authentication and authorization for the control plane (delegated to NATS and deployment)
- Audit logging and change history
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
  - Policy filter (policies applied by binding, shows only relevant policies)
  - Role filter
- **Details panel** (right side):
  - Binding metadata: `role`, `account`, `policies`
  - Compiled statements showing aggregated permissions from all policies
  - Related policies with hyperlinks
  - Key mismatch warnings if binding data doesn't match KV key
- **Actions**:
  - Create binding (supports both form and JSON editing)
  - Edit binding (supports both form and JSON editing)
  - Delete binding

### 3) Simulator Page

- **Input form** (left side, 50% width):
  - User name field
  - Target account selector (defaults to currently selected account)
  - Roles multi-select (shows roles as `account.role`)
- **Results panel** (right side, 50% width):
  - Compilation result status (success/error)
  - Aggregated permissions (publish and subscribe subject lists)
  - Roles & policies section with hyperlinks to binding and policy details
  - Raw JSON response (expandable)
- **Features**:
  - Sends requests to `nauts.debug` NATS subject
  - Persists simulation state in browser localStorage
  - Shows real-time permission compilation results

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

### Global Policy References in Bindings

When bindings reference global policies, they must use the `_global:` prefix in the policy ID:

**Storage format:**
```json
{
  "role": "admin",
  "account": "APP",
  "policies": ["app-admin", "_global:base-permissions"]
}
```

The `_global:` prefix tells the `NatsPolicyProvider` to look up the policy with `account="*"` instead of the binding's account. The control plane automatically adds this prefix when saving bindings that reference global policies, and strips it when displaying policies in the UI.

**Key lookup behavior:**
- Without prefix: `APP.policy.base-permissions` (account-specific)
- With prefix `_global:`: Strips prefix, looks up with `account="*"`

---

## NATS Connection

### Transport

- WebSocket connection to NATS (e.g., `wss://nats.example.com:443`)
- Browser-compatible NATS client (nats.js)

### Connection Settings

- URL, credentials, and TLS settings are provided to the UI via configuration
- Supported auth: **NKey** and **Credentials File**
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

## Implementation Details

### Layout & UI Structure

**Responsive Design:**
- Sidebar navigation for page switching
- Top toolbar with account selector and user actions
- 50/50 split layout on policies and bindings pages:
  - Left: List view with search/filter controls
  - Right: Details panel with full entity information
- Simulator uses 50/50 grid: form left, results right
- Fixed FAB buttons for create actions

**Material Design Components:**
- Tables with sorting for list views
- Expansion panels for grouped content (statements, compiled permissions)
- Dialogs for create/edit with dual mode (form and JSON)
- Snackbars for transient notifications
- Progress bars for loading states

### Key Features

**1. Compiled Statements View**
- Bindings details page shows aggregated permissions from all policies
- Statement-level details with source policy attribution
- Hyperlinks to source policies

**2. Global Policy Handling**
- UI displays global policies with italic "global" text
- Policy dropdown filters show relevant policies (account-specific + global)
- Automatic `_global:` prefix handling in storage layer
- Policy store service checks both prefixed and non-prefixed IDs

**3. Bucket Import/Export**
- Export entire KV bucket as JSON (key-value pairs)
- Import with validation and conflict handling
- Progress indicators and error reporting
- Date-stamped export filenames

**4. Key Mismatch Validation**
- Compares entity data (account/role/ID) with KV key parts
- Shows warning banner in details panel if mismatch detected
- Helps identify data corruption or migration issues

**5. Simulator Features**
- Real-time permission compilation via `nauts.debug` subject
- Request/response persistence in localStorage
- Aggregated permission display (publish/subscribe)
- Role-to-policy navigation with hyperlinks
- Raw JSON response viewer

### Angular Services

**NatsService** (`services/nats.service.ts`):
- WebSocket connection management
- Request/reply pattern support
- Connection state observables

**KvStoreService** (`services/kv-store.service.ts`):
- JetStream KV operations (get, put, delete, list, watch)
- Generic typed interface
- Optimistic concurrency with revision tracking

**PolicyStoreService** (`services/policy-store.service.ts`):
- In-memory cache of policies and bindings
- Reactive data streams with RxJS
- KV watcher for live updates
- Policy/binding CRUD operations
- Global policy reference handling

**NavigationService** (`services/navigation.service.ts`):
- Account selection state management
- Sidebar navigation state
- Shared state across components

---

## Known Limitations / Future Work

- **No server-side API**: All operations occur directly against NATS KV.
- **No role/account management**: Only policies and bindings are editable.
- **No history/versioning**: KV history is not surfaced in UI.
- **No authentication UX**: Login/identity flows are deployment-specific.
- **No real-time collaboration**: No presence awareness when multiple admins edit simultaneously.
