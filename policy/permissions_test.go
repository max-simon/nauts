package policy

import (
	"testing"
)

func TestIsCoveredBy(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		pattern string
		want    bool
	}{
		// Exact match
		{"exact match", "foo.bar", "foo.bar", true},
		{"exact match single", "foo", "foo", true},

		// Star wildcard
		{"star covers single token", "foo.bar", "foo.*", true},
		{"star in middle", "foo.bar.baz", "foo.*.baz", true},
		{"star multiple", "foo.bar.baz", "*.*.*", true},
		{"star doesn't cover multiple", "foo.bar.baz", "foo.*", false},
		{"star at start", "foo.bar", "*.bar", true},

		// GT wildcard
		{"gt covers single", "foo.bar", "foo.>", true},
		{"gt covers multiple", "foo.bar.baz", "foo.>", true},
		{"gt covers many", "foo.bar.baz.qux", "foo.>", true},
		{"gt at root", "foo.bar", ">", true},
		{"gt requires at least one", "foo", "foo.>", false},

		// Combined
		{"star then gt", "foo.bar.baz", "*.>", true},
		{"star then gt covers", "a.b.c.d", "*.>", true},

		// Non-matches
		{"different prefix", "foo.bar", "baz.*", false},
		{"too short for pattern", "foo", "foo.bar", false},
		{"longer than pattern without wildcard", "foo.bar.baz", "foo.bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCoveredBy(tt.subject, tt.pattern); got != tt.want {
				t.Errorf("isCoveredBy(%q, %q) = %v, want %v", tt.subject, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestDeduplicateWithWildcards(t *testing.T) {
	tests := []struct {
		name   string
		input  []string
		expect []string
	}{
		{
			name:   "no duplicates",
			input:  []string{"foo.bar", "baz.qux"},
			expect: []string{"baz.qux", "foo.bar"},
		},
		{
			name:   "exact duplicate",
			input:  []string{"foo.bar", "foo.bar"},
			expect: []string{"foo.bar"},
		},
		{
			name:   "star covers specific",
			input:  []string{"foo.bar", "foo.*"},
			expect: []string{"foo.*"},
		},
		{
			name:   "gt covers multiple",
			input:  []string{"foo.bar", "foo.baz", "foo.>"},
			expect: []string{"foo.>"},
		},
		{
			name:   "gt covers star",
			input:  []string{"foo.*", "foo.>"},
			expect: []string{"foo.>"},
		},
		{
			name:   "multiple wildcards",
			input:  []string{"a.b.c", "a.b.d", "a.*.*", "x.y"},
			expect: []string{"a.*.*", "x.y"},
		},
		{
			name:   "root gt covers all",
			input:  []string{"foo.bar", "baz.qux", ">"},
			expect: []string{">"},
		},
		{
			name:   "empty input",
			input:  []string{},
			expect: []string{},
		},
		{
			name:   "inbox pattern",
			input:  []string{"_INBOX.abc123", "_INBOX.def456", "_INBOX.>"},
			expect: []string{"_INBOX.>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to map
			input := make(map[string]struct{})
			for _, s := range tt.input {
				input[s] = struct{}{}
			}

			result := deduplicateWithWildcards(input)

			// Convert back to slice for comparison
			got := make([]string, 0, len(result))
			for s := range result {
				got = append(got, s)
			}

			if len(got) != len(tt.expect) {
				t.Errorf("deduplicateWithWildcards() got %d items, want %d", len(got), len(tt.expect))
				t.Errorf("got: %v", got)
				t.Errorf("want: %v", tt.expect)
				return
			}

			// Check all expected items are present
			for _, e := range tt.expect {
				if _, ok := result[e]; !ok {
					t.Errorf("deduplicateWithWildcards() missing expected %q", e)
				}
			}
		})
	}
}

func TestNatsPermissions_Allow(t *testing.T) {
	p := NewNatsPermissions()

	p.Allow(Permission{Type: PermPub, Subject: "orders"})
	p.Allow(Permission{Type: PermSub, Subject: "events"})
	p.Allow(Permission{Type: PermSub, Subject: "tasks", Queue: "workers"})

	p.Deduplicate()

	pubList := p.PubList()
	if len(pubList) != 1 || pubList[0] != "orders" {
		t.Errorf("Allow(PermPub) failed: got %v", pubList)
	}

	subList := p.SubList()
	if len(subList) != 1 || subList[0] != "events" {
		t.Errorf("Allow(PermSub) failed: got %v", subList)
	}

	subQueueList := p.SubWithQueueList()
	if len(subQueueList) != 1 {
		t.Errorf("Allow(PermSub with queue) failed: got %v", subQueueList)
	}
}

func TestNatsPermissions_Merge(t *testing.T) {
	p1 := NewNatsPermissions()
	p1.Allow(Permission{Type: PermPub, Subject: "foo"})
	p1.Allow(Permission{Type: PermSub, Subject: "bar"})

	p2 := NewNatsPermissions()
	p2.Allow(Permission{Type: PermPub, Subject: "baz"})
	p2.Allow(Permission{Type: PermSub, Subject: "qux"})

	p1.Merge(p2)
	p1.Deduplicate()

	if len(p1.PubList()) != 2 {
		t.Errorf("Merge Pub failed: got %v", p1.PubList())
	}

	if len(p1.SubList()) != 2 {
		t.Errorf("Merge Sub failed: got %v", p1.SubList())
	}
}

func TestNatsPermissions_DeduplicateWithWildcards(t *testing.T) {
	p := NewNatsPermissions()

	// Add specific subjects and wildcards
	p.Allow(Permission{Type: PermPub, Subject: "orders.created"})
	p.Allow(Permission{Type: PermPub, Subject: "orders.updated"})
	p.Allow(Permission{Type: PermPub, Subject: "orders.>"})
	p.Allow(Permission{Type: PermSub, Subject: "_INBOX.abc123"})
	p.Allow(Permission{Type: PermSub, Subject: "_INBOX.>"})

	p.Deduplicate()

	pubList := p.PubList()
	// Wildcards should cover specifics
	if len(pubList) != 1 || pubList[0] != "orders.>" {
		t.Errorf("Pub dedup failed: got %v, want [orders.>]", pubList)
	}

	subList := p.SubList()
	if len(subList) != 1 || subList[0] != "_INBOX.>" {
		t.Errorf("Sub dedup failed: got %v, want [_INBOX.>]", subList)
	}
}

func TestNatsPermissions_IsEmpty(t *testing.T) {
	p := NewNatsPermissions()
	if !p.IsEmpty() {
		t.Error("New NatsPermissions should be empty")
	}

	p.Allow(Permission{Type: PermPub, Subject: "test"})
	if p.IsEmpty() {
		t.Error("NatsPermissions with pub should not be empty")
	}
}

func TestNatsPermissions_GetPermissions(t *testing.T) {
	p := NewNatsPermissions()
	p.Allow(Permission{Type: PermPub, Subject: "orders"})
	p.Allow(Permission{Type: PermSub, Subject: "events"})

	perms := p.GetPermissions()

	pub, ok := perms["publish"].(map[string]interface{})
	if !ok {
		t.Fatal("GetPermissions() missing publish")
	}
	if _, ok := pub["allow"]; !ok {
		t.Error("GetPermissions() publish missing allow")
	}

	sub, ok := perms["subscribe"].(map[string]interface{})
	if !ok {
		t.Fatal("GetPermissions() missing subscribe")
	}
	if _, ok := sub["allow"]; !ok {
		t.Error("GetPermissions() subscribe missing allow")
	}
}

func TestPermissionSet_Basic(t *testing.T) {
	ps := NewPermissionSet()

	ps.Add("foo")
	ps.Add("bar")
	ps.Add("foo") // duplicate

	list := ps.AllowList()
	if len(list) != 2 {
		t.Errorf("PermissionSet.AllowList() got %d items, want 2", len(list))
	}
}

func TestPermissionSet_IsEmpty(t *testing.T) {
	ps := NewPermissionSet()
	if !ps.IsEmpty() {
		t.Error("New PermissionSet should be empty")
	}

	ps.Add("test")
	if ps.IsEmpty() {
		t.Error("PermissionSet with item should not be empty")
	}
}

func TestNatsPermissions_SubWithQueueList(t *testing.T) {
	p := NewNatsPermissions()
	p.Allow(Permission{Type: PermSub, Subject: "tasks", Queue: "workers"})
	p.Allow(Permission{Type: PermSub, Subject: "tasks", Queue: "admins"})
	p.Allow(Permission{Type: PermSub, Subject: "events", Queue: "logger"})

	list := p.SubWithQueueList()
	if len(list) != 3 {
		t.Errorf("SubWithQueueList() got %d items, want 3", len(list))
	}

	// Check sorting
	if list[0].Subject != "events" {
		t.Errorf("SubWithQueueList() not sorted by subject")
	}
}
