package policy

import "testing"

func TestInterpolateWithContext(t *testing.T) {
	ctx := &PolicyContext{}
	ctx.Set("user.id", "alice")
	ctx.Set("account.id", "ACME")
	ctx.Set("role.name", "workers")

	tests := []struct {
		name       string
		template   string
		ctx        *PolicyContext
		wantValue  string
		wantExcl   bool
		wantReason string
	}{
		// No variables
		{
			name:      "no variables",
			template:  "nats:orders",
			ctx:       ctx,
			wantValue: "nats:orders",
		},
		{
			name:      "empty template",
			template:  "",
			ctx:       ctx,
			wantValue: "",
		},

		// User variables
		{
			name:      "user.id",
			template:  "nats:user.{{ user.id }}",
			ctx:       ctx,
			wantValue: "nats:user.alice",
		},
		{
			name:      "account.id",
			template:  "nats:account.{{ account.id }}.orders",
			ctx:       ctx,
			wantValue: "nats:account.ACME.orders",
		},

		// Role variables
		{
			name:      "role.name",
			template:  "nats:role.{{ role.name }}.>",
			ctx:       ctx,
			wantValue: "nats:role.workers.>",
		},

		// Multiple variables
		{
			name:      "multiple variables",
			template:  "nats:{{ account.id }}.{{ user.id }}.orders",
			ctx:       ctx,
			wantValue: "nats:ACME.alice.orders",
		},

		// Whitespace handling
		{
			name:      "whitespace around variable",
			template:  "nats:{{ user.id }}",
			ctx:       ctx,
			wantValue: "nats:alice",
		},
		{
			name:      "extra whitespace",
			template:  "nats:{{  user.id  }}",
			ctx:       ctx,
			wantValue: "nats:alice",
		},
		{
			name:      "no whitespace",
			template:  "nats:{{user.id}}",
			ctx:       ctx,
			wantValue: "nats:alice",
		},

		// Excluded resources (unresolved)
		{
			name:       "nil context",
			template:   "nats:{{ user.id }}",
			ctx:        nil,
			wantExcl:   true,
			wantReason: "nil context",
		},
		{
			name:       "missing user.id claim",
			template:   "nats:{{ user.id }}",
			ctx:        &PolicyContext{},
			wantExcl:   true,
			wantReason: "unresolved variable: user.id",
		},
		{
			name:       "missing role.name claim",
			template:   "nats:{{ role.name }}",
			ctx:        &PolicyContext{},
			wantExcl:   true,
			wantReason: "unresolved variable: role.name",
		},
		{
			name:       "missing account.id claim",
			template:   "nats:{{ account.id }}",
			ctx:        &PolicyContext{},
			wantExcl:   true,
			wantReason: "unresolved variable: account.id",
		},
		{
			name:       "unknown root",
			template:   "nats:{{ unknown.var }}",
			ctx:        ctx,
			wantExcl:   true,
			wantReason: "unresolved variable: unknown.var",
		},
		{
			name:       "unknown user property",
			template:   "nats:{{ user.unknown }}",
			ctx:        ctx,
			wantExcl:   true,
			wantReason: "unresolved variable: user.unknown",
		},
		{
			name:       "unknown role property",
			template:   "nats:{{ role.unknown }}",
			ctx:        ctx,
			wantExcl:   true,
			wantReason: "unresolved variable: role.unknown",
		},

		// Excluded resources (invalid values)
		{
			name:       "user.id with wildcard",
			template:   "nats:{{ user.id }}",
			ctx:        func() *PolicyContext { c := &PolicyContext{}; c.Set("user.id", "alice*"); return c }(),
			wantExcl:   true,
			wantReason: "invalid value for user.id: alice*",
		},
		{
			name:       "account.id with wildcard",
			template:   "nats:{{ account.id }}",
			ctx:        func() *PolicyContext { c := &PolicyContext{}; c.Set("account.id", "ACME*"); return c }(),
			wantExcl:   true,
			wantReason: "invalid value for account.id: ACME*",
		},
		{
			name:       "user.id with gt",
			template:   "nats:{{ user.id }}",
			ctx:        func() *PolicyContext { c := &PolicyContext{}; c.Set("user.id", "alice>"); return c }(),
			wantExcl:   true,
			wantReason: "invalid value for user.id: alice>",
		},
		{
			name:       "user.id empty",
			template:   "nats:{{ user.id }}",
			ctx:        &PolicyContext{},
			wantExcl:   true,
			wantReason: "unresolved variable: user.id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := InterpolateWithContext(tt.template, tt.ctx)

			if result.Excluded != tt.wantExcl {
				t.Errorf("InterpolateWithContext().Excluded = %v, want %v", result.Excluded, tt.wantExcl)
			}

			if tt.wantExcl {
				if result.Warning != tt.wantReason {
					t.Errorf("InterpolateWithContext().Warning = %q, want %q", result.Warning, tt.wantReason)
				}
			} else {
				if result.Value != tt.wantValue {
					t.Errorf("InterpolateWithContext().Value = %q, want %q", result.Value, tt.wantValue)
				}
			}
		})
	}
}

func TestContainsVariables(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"nats:orders", false},
		{"nats:{{ user.id }}", true},
		{"nats:user.{{ user.id }}.orders", true},
		{"nats:{{user.id}}", true},
		{"nats:{{  user.id  }}", true},
		{"nats:{{ invalid", false}, // unclosed
		{"nats:invalid }}", false}, // no opening
		{"{{ }}", false},           // empty variable
		{"{{ a }}", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ContainsVariables(tt.input); got != tt.want {
				t.Errorf("ContainsVariables(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
