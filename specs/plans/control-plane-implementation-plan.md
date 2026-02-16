# Control Plane (Policies & Bindings) - Implementation Plan

**Spec:** [2026-02-12-control-plane.md](../2026-02-12-control-plane.md)
**Status:** Ready for implementation
**UI Framework:** Angular 19 + Angular Material (M3)
**Deployment:** Static files (`dist/`)

---

## Phase 1: Project Scaffold & Angular Material Setup

### Task 1.1: Create Angular Workspace

- [ ] Scaffold Angular 19 app with standalone components (`ng new control-plane --standalone --style=scss --routing`)
- [ ] Configure project inside `ctrlp/` directory at repository root
- [ ] Set up strict TypeScript (`strict: true`)
- [ ] Configure linting (ESLint) and formatting (Prettier)
- [ ] Add `.editorconfig` consistent with project conventions

**Deliverables:** Clean Angular workspace in `ctrlp/`

### Task 1.2: Angular Material with M3 Theming

- [ ] Install Angular Material (`ng add @angular/material`)
- [ ] Configure M3 baseline theme using `@angular/material` theme API in `styles.scss`
- [ ] Set up Material typography (M3 type scale)
- [ ] Import required Material modules (toolbar, sidenav, table, form-field, button, icon, dialog, snack-bar, chips, progress-bar, select)
- [ ] Register Material icons font

**Deliverables:** M3-themed Angular Material setup with all required components available

### Task 1.3: Application Shell & Routing

- [ ] Create shell layout component:
  - `mat-toolbar` — top bar with app title ("nauts Control Plane") and account selector (`mat-select`)
  - `mat-sidenav-container` — side navigation with `mat-nav-list` (Policies, Bindings links)
  - `<router-outlet>` — main content area
- [ ] Configure routes:
  - `/policies` — Policies page (default redirect from `/`)
  - `/bindings` — Bindings page
- [ ] Add `mat-icon` for nav items (e.g., `policy` for Policies, `link` for Bindings)

**Deliverables:** Working shell with navigation between two placeholder pages

### Task 1.4: Runtime Configuration

- [ ] Create `AppConfig` interface:
  ```typescript
  interface AppConfig {
    nats: {
      url: string;
      bucket: string;
      credentials?: string;  // token
      nkey?: string;
    };
    ui: {
      defaultAccount: string;
      showGlobalPolicies: boolean;
    };
  }
  ```
- [ ] Implement `ConfigService` that loads `assets/config.json` at startup via `APP_INITIALIZER`
- [ ] Provide typed access to config values
- [ ] Include sample `config.json` with placeholder values

**Deliverables:** `ConfigService` with typed config model, loaded before app bootstrap

---

## Phase 2: NATS Integration (WebSocket + KV)

### Task 2.1: NATS Client Service

- [ ] Install `nats.ws` package (NATS WebSocket client for browsers)
- [ ] Implement `NatsService`:
  - `connect()` — establish WebSocket connection using config URL and credentials
  - `disconnect()` — drain and close connection
  - `connectionStatus$` — `BehaviorSubject<'connected' | 'reconnecting' | 'disconnected' | 'error'>`
  - `getKvBucket()` — return JetStream KV bucket handle
- [ ] Handle reconnection with exponential backoff
- [ ] Initialize connection on app startup (via `APP_INITIALIZER` after config loads)

**Deliverables:** `NatsService` with reactive connection status

### Task 2.2: KV Store Service

- [ ] Implement `KvStoreService` wrapping JetStream KV operations:
  - `list(prefix: string): Promise<KvEntry[]>` — list keys by prefix, fetch values
  - `get(key: string): Promise<KvEntry | null>` — get single entry with revision
  - `put(key: string, value: object, revision?: number): Promise<number>` — put with optional optimistic concurrency (revision check)
  - `create(key: string, value: object): Promise<number>` — create only if key does not exist
  - `delete(key: string, revision: number): Promise<void>` — delete with expected revision
- [ ] `KvEntry` type: `{ key: string; value: T; revision: number }`
- [ ] JSON encode/decode on put/get
- [ ] Map KV errors to typed errors: `ConflictError`, `NotFoundError`

**Deliverables:** `KvStoreService` with typed operations and error handling

---

## Phase 3: Domain Services & Validation

### Task 3.1: Domain Models

- [ ] Define TypeScript interfaces matching the Go types:
  ```typescript
  interface Policy {
    id: string;
    account: string;
    name: string;
    statements: Statement[];
  }

  interface Statement {
    effect: 'allow';
    actions: string[];
    resources: string[];
  }

  interface Binding {
    role: string;
    account: string;
    policies: string[];
  }
  ```

**Deliverables:** Shared type definitions in `src/app/models/`

### Task 3.2: KV Key Helpers

- [ ] Implement key construction functions matching `NatsPolicyProvider` schema:
  - `policyKey(account, id)` → `<account>.policy.<id>`
  - `bindingKey(account, role)` → `<account>.binding.<role>`
  - `globalPolicyKey(id)` → `_global.policy.<id>`
  - `globalBindingKey(role)` → `_global.binding.<role>`
  - `accountFromKeyPrefix(prefix)` → map `_global` back to `*`
- [ ] Implement key parsing functions:
  - `parsePolicyKey(key)` → `{ account, id }`
  - `parseBindingKey(key)` → `{ account, role }`
- [ ] Unit tests for all key helpers

**Deliverables:** Key helper utilities with tests in `src/app/services/kv-keys.ts`

### Task 3.3: Policy Service

- [ ] Implement `PolicyService`:
  - `listPolicies(account: string): Promise<PolicyEntry[]>` — list by `<account>.policy.` prefix
  - `listGlobalPolicies(): Promise<PolicyEntry[]>` — list by `_global.policy.` prefix
  - `getPolicy(account: string, id: string): Promise<PolicyEntry>` — single fetch
  - `createPolicy(policy: Policy): Promise<number>` — generate UUID for `id`, use `KvStoreService.create()`
  - `updatePolicy(account: string, id: string, policy: Policy, revision: number): Promise<number>` — use `KvStoreService.put()` with revision
  - `deletePolicy(account: string, id: string, revision: number): Promise<void>`
- [ ] `PolicyEntry` type: `{ policy: Policy; revision: number }`
- [ ] UUID generation for new policy IDs (use `crypto.randomUUID()`)

**Deliverables:** `PolicyService` in `src/app/services/policy.service.ts`

### Task 3.4: Binding Service

- [ ] Implement `BindingService`:
  - `listBindings(account: string): Promise<BindingEntry[]>` — list by `<account>.binding.` prefix
  - `getBinding(account: string, role: string): Promise<BindingEntry>` — single fetch
  - `createBinding(binding: Binding): Promise<number>` — use `KvStoreService.create()`
  - `updateBinding(account: string, role: string, binding: Binding, revision: number): Promise<number>`
  - `deleteBinding(account: string, role: string, revision: number): Promise<void>`
- [ ] `BindingEntry` type: `{ binding: Binding; revision: number }`

**Deliverables:** `BindingService` in `src/app/services/binding.service.ts`

### Task 3.5: Account Discovery Service

- [ ] Implement `AccountService`:
  - `discoverAccounts(): Promise<string[]>` — scan KV keys, extract unique account prefixes
  - Cache discovered accounts (refresh on demand)
- [ ] Handle `_global` → display as "Global" in the UI

**Deliverables:** `AccountService` in `src/app/services/account.service.ts`

### Task 3.6: Validation

- [ ] Implement `PolicyValidator`:
  - `id`: required, non-empty
  - `account`: required
  - `name`: required, non-empty
  - `statements`: at least one statement
  - Each statement: `effect` must be `"allow"`, `actions` non-empty array, `resources` non-empty array
- [ ] Implement `BindingValidator`:
  - `role`: required, non-empty
  - `account`: required
  - `policies`: non-empty array of policy IDs
- [ ] Return structured validation errors suitable for inline form display
- [ ] Unit tests for both validators

**Deliverables:** Validator utilities with tests in `src/app/validators/`

---

## Phase 4: Policies Page

### Task 4.1: Policies List Component

- [ ] Master-detail layout: list on left, details panel on right
- [ ] `mat-table` with columns: Name, ID, Account, Statements count
- [ ] `mat-sort` for column sorting
- [ ] Account selector in toolbar (`mat-select`) — filters policies to selected account
- [ ] Search field (`mat-form-field` with `mat-input`) — filter by `id` or `name`
- [ ] Toggle for showing global policies (`mat-slide-toggle`) if `ui.showGlobalPolicies` is enabled
- [ ] Loading state: `mat-progress-bar` (indeterminate) while fetching
- [ ] Empty state: centered message with `mat-icon` when no policies exist
- [ ] FAB button (`mat-fab`) to create new policy

**Deliverables:** Policies list with filters, sorting, and loading states

### Task 4.2: Policy Details Panel

- [ ] Opens when a row in the table is selected (right-side panel or expanding section)
- [ ] Display policy metadata in `mat-card`:
  - ID (read-only, displayed as `mat-chip`)
  - Name
  - Account
- [ ] Statements list:
  - Each statement rendered as a section with:
    - Effect badge (`mat-chip` with color)
    - Actions displayed as `mat-chip` set
    - Resources displayed as `mat-chip` set
- [ ] Show validation warnings/errors inline if policy data is malformed
- [ ] Edit and Delete action buttons (`mat-button`)

**Deliverables:** Policy details panel with statement rendering

### Task 4.3: Create/Edit Policy Dialog

- [ ] `mat-dialog` for create and edit operations
- [ ] Form fields:
  - Name (`mat-form-field` + `mat-input`)
  - Account (`mat-select` — locked on edit, selectable on create)
  - Statements section:
    - Add/remove statement buttons
    - Per statement: effect (locked to "allow"), actions (`mat-form-field` with comma-separated or chip input), resources (`mat-form-field` with comma-separated or chip input)
- [ ] `mat-chip-grid` for actions and resources input (type and press Enter to add)
- [ ] Real-time validation with inline `mat-error` messages
- [ ] On create: auto-generate UUID for `id`, call `PolicyService.createPolicy()`
- [ ] On edit: pass current revision, call `PolicyService.updatePolicy()`
- [ ] On success: `mat-snack-bar` confirmation, refresh list
- [ ] On conflict: `mat-snack-bar` error, prompt to reload

**Deliverables:** Fully functional create/edit dialog with validation

### Task 4.4: Delete Policy

- [ ] Confirmation dialog (`mat-dialog`) with policy name and warning text
- [ ] On confirm: call `PolicyService.deletePolicy()` with revision
- [ ] On success: `mat-snack-bar` confirmation, deselect, refresh list
- [ ] On conflict: show conflict error message

**Deliverables:** Delete flow with confirmation

---

## Phase 5: Bindings Page

### Task 5.1: Bindings List Component

- [ ] Same master-detail layout as Policies page
- [ ] `mat-table` with columns: Role, Account, Policies count
- [ ] `mat-sort` for column sorting
- [ ] Account selector filter (`mat-select`)
- [ ] Role filter (`mat-form-field` + `mat-input`)
- [ ] Policy filter (`mat-select` — populated from policies in selected account)
- [ ] Loading, empty states matching policies page
- [ ] FAB button for create

**Deliverables:** Bindings list with filters and sorting

### Task 5.2: Binding Details Panel

- [ ] Display binding metadata in `mat-card`:
  - Role name
  - Account
- [ ] Policies list:
  - Each policy ID as `mat-chip`
  - Show warning icon (`mat-icon: warning`) next to dangling references (policy ID not found in KV)
- [ ] Edit and Delete action buttons

**Deliverables:** Binding details with dangling reference warnings

### Task 5.3: Create/Edit Binding Dialog

- [ ] `mat-dialog` for create/edit
- [ ] Form fields:
  - Role (`mat-form-field` + `mat-input` — locked on edit, editable on create)
  - Account (`mat-select` — locked on edit)
  - Policies (`mat-chip-grid` or `mat-select` with multiple selection from available policies)
- [ ] Real-time validation with `mat-error`
- [ ] CRUD calls to `BindingService` with revision handling
- [ ] Success/conflict notifications via `mat-snack-bar`

**Deliverables:** Create/edit binding dialog

### Task 5.4: Delete Binding

- [ ] Confirmation dialog with role name
- [ ] Same pattern as policy delete

**Deliverables:** Delete flow with confirmation

---

## Phase 6: Error Handling & UX Polish

### Task 6.1: Connection Status Banner

- [ ] Sticky banner at top of page (below toolbar) when NATS connection is lost
- [ ] Show connection state: "Reconnecting..." with `mat-progress-bar`, or "Disconnected" with retry button (`mat-button`)
- [ ] Auto-dismiss when connection is restored
- [ ] Disable CRUD actions while disconnected (buttons greyed out)

**Deliverables:** Connection error banner

### Task 6.2: Optimistic Concurrency Conflict Handling

- [ ] On `ConflictError` from KV put/delete:
  - Show `mat-snack-bar` with message: "This item was modified by another user. Reloading..."
  - Auto-reload the item details and list
  - Close any open edit dialog
- [ ] Track revision numbers through the entire create/edit/delete flow

**Deliverables:** Conflict detection and recovery UX

### Task 6.3: Empty & Loading States

- [ ] Empty state for policies list: illustration/icon + "No policies found" message + "Create Policy" button
- [ ] Empty state for bindings list: same pattern
- [ ] Loading spinners (`mat-spinner`) for initial data fetch
- [ ] Skeleton loading or `mat-progress-bar` for subsequent fetches

**Deliverables:** Polished empty and loading states

### Task 6.4: Form Validation UX

- [ ] All form fields show `mat-error` on blur and on submit attempt
- [ ] Disable submit button when form is invalid
- [ ] Show server-side errors (e.g., key already exists on create) in `mat-snack-bar`

**Deliverables:** Consistent validation feedback

---

## Phase 7: Testing

### Task 7.1: Unit Tests — Utilities & Validators

- [ ] KV key helpers: construction and parsing (all patterns including `_global`)
- [ ] Policy validator: all rules (required fields, statement structure)
- [ ] Binding validator: all rules
- [ ] UUID generation for policy IDs

**Deliverables:** Unit tests for pure logic

### Task 7.2: Unit Tests — Services

- [ ] `PolicyService`: mock `KvStoreService`, test CRUD flows
- [ ] `BindingService`: mock `KvStoreService`, test CRUD flows
- [ ] `AccountService`: mock KV key listing, verify account extraction
- [ ] `ConfigService`: mock HTTP, test config loading

**Deliverables:** Service unit tests with mocked dependencies

### Task 7.3: Component Tests

- [ ] Shell layout: navigation renders, account selector works
- [ ] Policies list: renders table, filters work, selection triggers details
- [ ] Bindings list: same patterns
- [ ] Create/edit dialogs: form validation, submit calls service
- [ ] Delete dialogs: confirmation flow

**Deliverables:** Component tests using Angular testing utilities

### Task 7.4: Integration Tests (Optional)

- [ ] End-to-end CRUD flow against a real NATS server (test container or local)
- [ ] Conflict simulation: two concurrent edits
- [ ] Connection loss and recovery

**Deliverables:** Integration test suite (if time permits)

---

## Phase 8: Build & Deployment

### Task 8.1: Production Build

- [ ] Configure Angular production build (`ng build --configuration production`)
- [ ] Output to `ctrlp/dist/`
- [ ] Verify bundle size is reasonable (tree-shaking, lazy loading if needed)
- [ ] Add `ctrlp/dist/` to `.gitignore`

**Deliverables:** Production-ready static build

### Task 8.2: Deployment Configuration

- [ ] Document how to serve static files (nginx example config)
- [ ] Document WebSocket proxy configuration for NATS (nginx `proxy_pass` for `wss://`)
- [ ] Add sample `config.json` for different environments

**Deliverables:** Deployment documentation

---

## Phase 9: Documentation

### Task 9.1: Control Plane README

- [ ] Add `ctrlp/README.md`:
  - Prerequisites (Node.js, npm)
  - Development setup (`npm install`, `ng serve`)
  - Configuration (`config.json` format)
  - Build (`ng build`)
  - Deployment (static files + NATS WebSocket)
- [ ] Document required NATS permissions for the control plane user

### Task 9.2: Update Project Documentation

- [ ] Update root `README.md` to mention control plane
- [ ] Update `specs/README.md` if needed

**Deliverables:** Complete setup and usage documentation

---

## Dependency Graph

```
Phase 1 (Scaffold + Material + Config)
  └── Phase 2 (NATS Integration)
        └── Phase 3 (Domain Services + Validation)
              ├── Phase 4 (Policies Page)
              └── Phase 5 (Bindings Page)
                    └── Phase 6 (Error Handling & UX Polish)
                          └── Phase 7 (Testing)
                                └── Phase 8 (Build & Deployment)
                                      └── Phase 9 (Documentation)
```

Phases 4 and 5 can be developed in parallel once Phase 3 is complete.

---

## Milestone Checklist

- [ ] Angular workspace with M3 theme and shell layout working
- [ ] NATS WebSocket connection and KV operations working
- [ ] Domain services with validation passing unit tests
- [ ] Policies page: list, details, create, edit, delete
- [ ] Bindings page: list, details, create, edit, delete
- [ ] Error handling: connection banner, conflicts, empty states
- [ ] All tests passing
- [ ] Production build generating clean static files
- [ ] Documentation complete

---

## File Structure (Planned)

```
ctrlp/
├── src/
│   ├── app/
│   │   ├── app.component.ts           # Shell layout (toolbar + sidenav + router-outlet)
│   │   ├── app.config.ts              # App configuration (providers, routes)
│   │   ├── app.routes.ts              # Route definitions
│   │   ├── models/
│   │   │   ├── policy.model.ts        # Policy, Statement interfaces
│   │   │   ├── binding.model.ts       # Binding interface
│   │   │   └── config.model.ts        # AppConfig interface
│   │   ├── services/
│   │   │   ├── config.service.ts      # Runtime config loader
│   │   │   ├── nats.service.ts        # NATS WebSocket client
│   │   │   ├── kv-store.service.ts    # KV abstraction
│   │   │   ├── kv-keys.ts            # Key construction/parsing helpers
│   │   │   ├── policy.service.ts      # Policy CRUD
│   │   │   ├── binding.service.ts     # Binding CRUD
│   │   │   └── account.service.ts     # Account discovery
│   │   ├── validators/
│   │   │   ├── policy.validator.ts    # Policy validation rules
│   │   │   └── binding.validator.ts   # Binding validation rules
│   │   ├── pages/
│   │   │   ├── policies/
│   │   │   │   ├── policies.component.ts        # List + filters
│   │   │   │   ├── policy-details.component.ts  # Details panel
│   │   │   │   └── policy-dialog.component.ts   # Create/edit dialog
│   │   │   └── bindings/
│   │   │       ├── bindings.component.ts        # List + filters
│   │   │       ├── binding-details.component.ts # Details panel
│   │   │       └── binding-dialog.component.ts  # Create/edit dialog
│   │   └── shared/
│   │       ├── confirm-dialog.component.ts      # Reusable delete confirmation
│   │       ├── connection-banner.component.ts   # NATS status banner
│   │       └── empty-state.component.ts         # Reusable empty state
│   ├── assets/
│   │   └── config.json                # Runtime configuration
│   └── styles.scss                    # M3 theme + global styles
├── angular.json
├── package.json
├── tsconfig.json
└── README.md
```
