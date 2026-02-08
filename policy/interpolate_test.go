package policy

import (
	"testing"
)

// newTestContext creates an InterpolationContext with user and role for testing.
func newTestContext(user *UserContext, role *RoleContext) *InterpolationContext {
	ctx := &InterpolationContext{}
	if user != nil {
		ctx.Add("user", user)
	}
	if role != nil {
		ctx.Add("role", role)
	}
	return ctx
}

func TestInterpolateWithContext(t *testing.T) {
	user := &UserContext{
		ID:      "alice",
		Account: "ACME",
		Attributes: map[string]string{
			"department": "engineering",
			"team":       "platform",
		},
	}

	role := &RoleContext{
		Name:    "workers",
		Account: "*",
	}

	ctx := newTestContext(user, role)

	tests := []struct {
		name       string
		template   string
		ctx        *InterpolationContext
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
			name:      "user.account",
			template:  "nats:account.{{ user.account }}.orders",
			ctx:       ctx,
			wantValue: "nats:account.ACME.orders",
		},
		{
			name:      "user.attr.department",
			template:  "nats:dept.{{ user.attr.department }}",
			ctx:       ctx,
			wantValue: "nats:dept.engineering",
		},
		{
			name:      "user.attr.team",
			template:  "nats:team.{{ user.attr.team }}.>",
			ctx:       ctx,
			wantValue: "nats:team.platform.>",
		},

		// Role variables
		{
			name:      "role.name",
			template:  "nats:role.{{ role.name }}.>",
			ctx:       ctx,
			wantValue: "nats:role.workers.>",
		},
		{
			name:      "role.account with local account",
			template:  "nats:role.{{ role.account }}.>",
			ctx:       newTestContext(user, &RoleContext{Name: "workers", Account: "ACME"}),
			wantValue: "nats:role.ACME.>",
		},
		{
			name:       "role.account with global account (excluded)",
			template:   "nats:role.{{ role.account }}.>",
			ctx:        ctx,
			wantExcl:   true,
			wantReason: "invalid value for role.account: *",
		},

		// Multiple variables
		{
			name:      "multiple variables",
			template:  "nats:{{ user.account }}.{{ user.id }}.orders",
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
			name:       "nil user",
			template:   "nats:{{ user.id }}",
			ctx:        newTestContext(nil, role),
			wantExcl:   true,
			wantReason: "unresolved variable: user.id",
		},
		{
			name:       "nil role",
			template:   "nats:{{ role.name }}",
			ctx:        newTestContext(user, nil),
			wantExcl:   true,
			wantReason: "unresolved variable: role.name",
		},
		{
			name:       "missing attribute",
			template:   "nats:{{ user.attr.missing }}",
			ctx:        ctx,
			wantExcl:   true,
			wantReason: "unresolved variable: user.attr.missing",
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
			ctx:        newTestContext(&UserContext{ID: "alice*"}, nil),
			wantExcl:   true,
			wantReason: "invalid value for user.id: alice*",
		},
		{
			name:       "user.id with gt",
			template:   "nats:{{ user.id }}",
			ctx:        newTestContext(&UserContext{ID: "alice>"}, nil),
			wantExcl:   true,
			wantReason: "invalid value for user.id: alice>",
		},
		{
			name:       "user.id empty",
			template:   "nats:{{ user.id }}",
			ctx:        newTestContext(&UserContext{ID: ""}, nil),
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

func TestUserContext_GetAttribute(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		user  *UserContext
		want  string
		found bool
	}{
		{
			name:  "nil user",
			path:  "id",
			user:  nil,
			found: false,
		},
		{
			name:  "empty id",
			path:  "id",
			user:  &UserContext{},
			found: false,
		},
		{
			name:  "valid id",
			path:  "id",
			user:  &UserContext{ID: "alice"},
			want:  "alice",
			found: true,
		},
		{
			name:  "empty account",
			path:  "account",
			user:  &UserContext{},
			found: false,
		},
		{
			name:  "valid account",
			path:  "account",
			user:  &UserContext{Account: "ACME"},
			want:  "ACME",
			found: true,
		},
		{
			name:  "nil attributes map",
			path:  "attr.key",
			user:  &UserContext{},
			found: false,
		},
		{
			name:  "empty attribute value",
			path:  "attr.key",
			user:  &UserContext{Attributes: map[string]string{"key": ""}},
			found: false,
		},
		{
			name:  "valid attribute",
			path:  "attr.dept",
			user:  &UserContext{Attributes: map[string]string{"dept": "engineering"}},
			want:  "engineering",
			found: true,
		},
		{
			name:  "unknown path",
			path:  "unknown",
			user:  &UserContext{ID: "alice"},
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := tt.user.GetAttribute(tt.path)
			if found != tt.found {
				t.Errorf("UserContext.GetAttribute(%q) found = %v, want %v", tt.path, found, tt.found)
			}
			if got != tt.want {
				t.Errorf("UserContext.GetAttribute(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestRoleContext_GetAttribute(t *testing.T) {
	tests := []struct {
		name  string
		path  string
		role  *RoleContext
		want  string
		found bool
	}{
		{
			name:  "nil role",
			path:  "name",
			role:  nil,
			found: false,
		},
		{
			name:  "empty name",
			path:  "name",
			role:  &RoleContext{},
			found: false,
		},
		{
			name:  "valid name",
			path:  "name",
			role:  &RoleContext{Name: "workers"},
			want:  "workers",
			found: true,
		},
		{
			name:  "empty account",
			path:  "account",
			role:  &RoleContext{},
			found: false,
		},
		{
			name:  "valid account",
			path:  "account",
			role:  &RoleContext{Account: "*"},
			want:  "*",
			found: true,
		},
		{
			name:  "unknown path",
			path:  "policies",
			role:  &RoleContext{Name: "test"},
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := tt.role.GetAttribute(tt.path)
			if found != tt.found {
				t.Errorf("RoleContext.GetAttribute(%q) found = %v, want %v", tt.path, found, tt.found)
			}
			if got != tt.want {
				t.Errorf("RoleContext.GetAttribute(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestInterpolationContext_GetAttribute(t *testing.T) {
	user := &UserContext{ID: "alice", Account: "ACME"}
	role := &RoleContext{Name: "workers", Account: "*"}

	ctx := &InterpolationContext{}
	ctx.Add("user", user)
	ctx.Add("role", role)

	tests := []struct {
		name  string
		path  string
		want  string
		found bool
	}{
		{
			name:  "user.id",
			path:  "user.id",
			want:  "alice",
			found: true,
		},
		{
			name:  "user.account",
			path:  "user.account",
			want:  "ACME",
			found: true,
		},
		{
			name:  "role.name",
			path:  "role.name",
			want:  "workers",
			found: true,
		},
		{
			name:  "role.account",
			path:  "role.account",
			want:  "*",
			found: true,
		},
		{
			name:  "unknown prefix",
			path:  "unknown.id",
			found: false,
		},
		{
			name:  "no dot in path",
			path:  "userid",
			found: false,
		},
		{
			name:  "unknown user attribute",
			path:  "user.unknown",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := ctx.GetAttribute(tt.path)
			if found != tt.found {
				t.Errorf("InterpolationContext.GetAttribute(%q) found = %v, want %v", tt.path, found, tt.found)
			}
			if got != tt.want {
				t.Errorf("InterpolationContext.GetAttribute(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestInterpolationContext_NilContext(t *testing.T) {
	var ctx *InterpolationContext
	_, found := ctx.GetAttribute("user.id")
	if found {
		t.Error("nil InterpolationContext should return found=false")
	}
}

func TestInterpolationContext_EmptyContexts(t *testing.T) {
	ctx := &InterpolationContext{}
	_, found := ctx.GetAttribute("user.id")
	if found {
		t.Error("empty InterpolationContext should return found=false")
	}
}
