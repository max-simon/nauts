// Package policy provides policy-related types and functions for nauts.
// This file contains the policy compilation logic.
package policy

// CompileResult contains the result of policy compilation.
type CompileResult struct {
	Warnings []string // Warnings generated during compilation
}

// Compile compiles a set of policies with the given context and merges
// the results into the provided NatsPermissions.
//
// The compilation process:
// 1. For each policy statement with effect "allow"
// 2. Expand action groups to atomic actions
// 3. Interpolate variables in resources
// 4. Parse and validate resources
// 5. Map actions + resources to NATS permissions
// 6. Merge into the result permissions
//
// After calling Compile, the caller should call perms.Deduplicate()
// when all policies are compiled.
func Compile(policies []*Policy, ctx *PolicyContext, perms *NatsPermissions) CompileResult {
	result := CompileResult{}

	if ctx == nil {
		result.Warnings = append(result.Warnings, "policy skipped (nil context)")
		return result
	}

	// Always grant permission to subscribe to user's personalized inbox
	if userID := ctx.User; userID != "" {
		// INBOX prefix is _INBOX_{{user.id}}
		// We allow subscription to _INBOX_{{user.id}}.>
		perms.Allow(Permission{Type: PermSub, Subject: "_INBOX_" + userID + ".>"})
	}

	for _, pol := range policies {
		if pol == nil {
			continue
		}

		// Defense-in-depth: only compile policies applicable to the requested account.
		// Global policies (Account="*") always apply.
		switch {
		case ctx.Account == "":
			result.Warnings = append(result.Warnings, "policy skipped (missing account.id): "+pol.ID)
			continue
		case pol.Account == "_global":
			// ok
		case pol.Account == ctx.Account:
			// ok
		default:
			result.Warnings = append(result.Warnings, "policy skipped (account mismatch): "+pol.ID)
			continue
		}

		policyResult := compilePolicy(pol, ctx, perms)
		result.Warnings = append(result.Warnings, policyResult.Warnings...)
	}

	return result
}

// compilePolicy compiles a single policy with user and role context.
func compilePolicy(pol *Policy, ctx *PolicyContext, perms *NatsPermissions) CompileResult {
	result := CompileResult{}

	for _, stmt := range pol.Statements {
		if stmt.Effect != EffectAllow {
			continue // Only "allow" is supported
		}

		// Expand action groups to atomic actions
		actions := ResolveActions(stmt.Actions)

		// Process each resource
		for _, resource := range stmt.Resources {
			resourceResult := compileResource(resource, actions, ctx, perms)
			result.Warnings = append(result.Warnings, resourceResult.Warnings...)
		}
	}

	return result
}

// compileResource compiles permissions for a single resource with the given actions.
func compileResource(resource string, actions []Action, ctx *PolicyContext, perms *NatsPermissions) CompileResult {
	result := CompileResult{}

	// Interpolate variables if present
	var resolvedResource string
	if ContainsVariables(resource) {
		interpResult := InterpolateWithContext(resource, ctx)
		if interpResult.Excluded {
			result.Warnings = append(result.Warnings, "resource excluded: "+resource+" ("+interpResult.Warning+")")
			return result
		}
		resolvedResource = interpResult.Value
	} else {
		resolvedResource = resource
	}

	// Parse and validate resource
	n, err := ParseAndValidateResource(resolvedResource)
	if err != nil {
		result.Warnings = append(result.Warnings, "invalid resource: "+resolvedResource+" ("+err.Error()+")")
		return result
	}

	// Map each action to permissions
	for _, action := range actions {
		actionPerms := MapActionToPermissions(action, n)

		// Implicit JetStream info permission: any effective JS action grants $JS.API.INFO.
		// This is added only when the action successfully maps to at least one permission
		// for a valid resource.
		if len(actionPerms) > 0 && action.RequiresJetstream() {
			perms.Allow(Permission{Type: PermPub, Subject: "$JS.API.INFO"})
		}

		for _, p := range actionPerms {
			perms.Allow(p)
		}
	}

	return result
}
