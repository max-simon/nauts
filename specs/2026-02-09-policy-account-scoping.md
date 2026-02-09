# Policy account scoping

Date: 2026-02-09

## Summary

Policies are now scoped to NATS accounts. Each policy declares which account it applies to via a required `account` field, and the policy engine compiles only policies that apply to the authenticated user's account.

This prevents accidental cross-account policy leakage when policies are stored centrally.

The policy evaluation context also exposes the requested account as the interpolation claim `account.id`.

## Goals

- Ensure a policy is only applicable within its intended NATS account.
- Make policy listing account-aware.
- Enforce account scoping in the policy compiler as a defense-in-depth measure.

## Non-goals

- Changing how roles are resolved (global vs local roles) beyond existing behavior.
- Changing the semantics of actions or resources.

## Data model changes

### `policy.Policy`

A new required field is added:

- `Account string` (JSON: `account`): the NATS account ID this policy applies to.

Special value:

- `"*"` means **global policy** (applies to any account).

Validation updates:

- `Policy.Validate()` now returns an error if `account` is missing/empty.

## Provider interface changes

### `provider.PolicyProvider`

`ListPolicies` is now account-scoped:

- Before: `ListPolicies(ctx) ([]*policy.Policy, error)`
- After:  `ListPolicies(ctx, account string) ([]*policy.Policy, error)`

Expected behavior:

- Returns policies where `policy.Account == account`.
- Also returns global policies where `policy.Account == "*"`.

## Compiler changes

### `policy.Compile`

When a `user` context is provided, compilation is scoped to the user's account:

- If `user.Account` is empty, **no** account-scoped policies are compiled.
- If `policy.Account == user.Account`, the policy is compiled.
- If `policy.Account == "*"`, the policy is compiled (global policy).
- Otherwise, the policy is skipped.

This is defense-in-depth: even if an upstream provider accidentally returns policies from other accounts, they are not compiled into the user's permissions.

## Migration notes

- All JSON policy documents must be updated to include an `account` field.
- If the same policy ID should be usable across multiple accounts, set `account` to `"*"` (global) or duplicate the policy per account with distinct IDs.

## Examples

Account-local policy:

```json
{
  "id": "writer",
  "account": "APP",
  "name": "Writer",
  "statements": [
    {
      "effect": "allow",
      "actions": ["nats.pub"],
      "resources": ["nats:public.>"]
    }
  ]
}
```

Global policy:

```json
{
  "id": "shared-reader",
  "account": "*",
  "name": "Shared Reader",
  "statements": [
    {
      "effect": "allow",
      "actions": ["nats.sub"],
      "resources": ["nats:public.>"]
    }
  ]
}
```
