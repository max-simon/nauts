package policy

import (
	"testing"
)

func TestCompile_BasicPolicy(t *testing.T) {
	policies := []*Policy{
		{
			ID:   "test-policy",
			Name: "Test Policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:orders"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice", Account: "ACME"}
	group := &GroupContext{ID: "workers", Name: "Workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()
	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "orders" {
		t.Errorf("expected [orders], got %v", pubList)
	}

	// nats.pub should not add inbox
	subList := perms.SubList()
	if len(subList) != 0 {
		t.Errorf("expected no sub permissions, got %v", subList)
	}
}

func TestCompile_WithInterpolation(t *testing.T) {
	policies := []*Policy{
		{
			ID: "user-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:user.{{ user.id }}.orders"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice", Account: "ACME"}
	group := &GroupContext{ID: "workers", Name: "Workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()
	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "user.alice.orders" {
		t.Errorf("expected [user.alice.orders], got %v", pubList)
	}
}

func TestCompile_WithGroupInterpolation(t *testing.T) {
	policies := []*Policy{
		{
			ID: "group-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSSub},
					Resources: []string{"nats:group.{{ group.id }}.>"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers", Name: "Workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()
	subList := perms.SubList()
	if len(subList) != 1 || subList[0] != "group.workers.>" {
		t.Errorf("expected [group.workers.>], got %v", subList)
	}
}

func TestCompile_AddsInboxForJSAction(t *testing.T) {
	policies := []*Policy{
		{
			ID: "js-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionJSReadStream},
					Resources: []string{"js:mystream"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()

	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "$JS.API.STREAM.INFO.mystream" {
		t.Errorf("expected [$JS.API.STREAM.INFO.mystream], got %v", pubList)
	}

	// js.readStream requires inbox, should be added directly
	subList := perms.SubList()
	if len(subList) != 1 || subList[0] != "_INBOX.>" {
		t.Errorf("expected [_INBOX.>], got %v", subList)
	}
}

func TestCompile_UnresolvedVariable(t *testing.T) {
	policies := []*Policy{
		{
			ID: "bad-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:{{ user.attr.missing }}"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %v", result.Warnings)
	}

	perms.Deduplicate()
	if !perms.IsEmpty() {
		t.Error("expected empty permissions for unresolved variable")
	}
}

func TestCompile_InvalidResource(t *testing.T) {
	policies := []*Policy{
		{
			ID: "bad-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"invalid:resource"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 1 {
		t.Errorf("expected 1 warning, got %v", result.Warnings)
	}

	perms.Deduplicate()
	if !perms.IsEmpty() {
		t.Error("expected empty permissions for invalid resource")
	}
}

func TestCompile_MultiplePolicies(t *testing.T) {
	policies := []*Policy{
		{
			ID: "policy-1",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:orders"},
				},
			},
		},
		{
			ID: "policy-2",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSSub},
					Resources: []string{"nats:events"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()

	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "orders" {
		t.Errorf("expected [orders], got %v", pubList)
	}

	subList := perms.SubList()
	if len(subList) != 1 || subList[0] != "events" {
		t.Errorf("expected [events], got %v", subList)
	}
}

func TestCompile_ActionGroup(t *testing.T) {
	policies := []*Policy{
		{
			ID: "nats-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionGroupNATSAll}, // Group action: nats.pub, nats.sub, nats.req
					Resources: []string{"nats:orders"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()

	// nats.* expands to nats.pub, nats.sub, nats.req
	// nats.pub -> PUB orders
	// nats.sub -> SUB orders
	// nats.req -> PUB orders, SUB _INBOX.>
	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "orders" {
		t.Errorf("expected [orders], got %v", pubList)
	}

	subList := perms.SubList()
	if len(subList) != 2 {
		t.Errorf("expected 2 sub permissions (orders and _INBOX.>), got %v", subList)
	}
}

func TestCompile_DenyEffect(t *testing.T) {
	policies := []*Policy{
		{
			ID: "deny-policy",
			Statements: []Statement{
				{
					Effect:    Effect("deny"), // Deny is not supported, should be skipped
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:orders"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}
	perms := NewNatsPermissions()

	result := Compile(policies, user, group, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()
	if !perms.IsEmpty() {
		t.Error("expected empty permissions for deny effect")
	}
}

func TestCompile_MergeIntoExisting(t *testing.T) {
	// Start with existing permissions
	perms := NewNatsPermissions()
	perms.Allow(Permission{Type: PermPub, Subject: "existing"})

	policies := []*Policy{
		{
			ID: "new-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:orders"},
				},
			},
		},
	}

	user := &UserContext{ID: "alice"}
	group := &GroupContext{ID: "workers"}

	Compile(policies, user, group, perms)
	perms.Deduplicate()

	pubList := perms.PubList()
	if len(pubList) != 2 {
		t.Errorf("expected 2 pub permissions, got %v", pubList)
	}
}

func TestCompile_NilContexts(t *testing.T) {
	policies := []*Policy{
		{
			ID: "simple-policy",
			Statements: []Statement{
				{
					Effect:    EffectAllow,
					Actions:   []Action{ActionNATSPub},
					Resources: []string{"nats:orders"},
				},
			},
		},
	}

	perms := NewNatsPermissions()

	// Nil user and group should still work for non-interpolated resources
	result := Compile(policies, nil, nil, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	perms.Deduplicate()
	pubList := perms.PubList()
	if len(pubList) != 1 || pubList[0] != "orders" {
		t.Errorf("expected [orders], got %v", pubList)
	}
}

func TestCompile_EmptyPolicies(t *testing.T) {
	perms := NewNatsPermissions()

	result := Compile(nil, nil, nil, perms)

	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}
	if !perms.IsEmpty() {
		t.Error("expected empty permissions")
	}
}
