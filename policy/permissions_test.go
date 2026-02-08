package policy

import (
	"reflect"
	"sort"
	"testing"
)

func TestIsCoveredBy(t *testing.T) {
	tests := []struct {
		name    string
		subject Permission
		pattern Permission
		want    bool
	}{
		// Exact match
		{"exact match", Permission{Subject: "foo.bar"}, Permission{Subject: "foo.bar"}, true},
		{"exact match single", Permission{Subject: "foo"}, Permission{Subject: "foo"}, true},

		// Star wildcard
		{"star covers single token", Permission{Subject: "foo.bar"}, Permission{Subject: "foo.*"}, true},
		{"star in middle", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "foo.*.baz"}, true},
		{"star multiple", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "*.*.*"}, true},
		{"star doesn't cover multiple", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "foo.*"}, false},
		{"star at start", Permission{Subject: "foo.bar"}, Permission{Subject: "*.bar"}, true},

		// GT wildcard
		{"gt covers single", Permission{Subject: "foo.bar"}, Permission{Subject: "foo.>"}, true},
		{"gt covers multiple", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "foo.>"}, true},
		{"gt covers many", Permission{Subject: "foo.bar.baz.qux"}, Permission{Subject: "foo.>"}, true},
		{"gt at root", Permission{Subject: "foo.bar"}, Permission{Subject: ">"}, true},
		{"gt requires at least one", Permission{Subject: "foo"}, Permission{Subject: "foo.>"}, false},

		// Combined
		{"star then gt", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "*.>"}, true},
		{"star then gt covers", Permission{Subject: "a.b.c.d"}, Permission{Subject: "*.>"}, true},

		// Non-matches
		{"different prefix", Permission{Subject: "foo.bar"}, Permission{Subject: "baz.*"}, false},
		{"too short for pattern", Permission{Subject: "foo"}, Permission{Subject: "foo.bar"}, false},
		{"longer than pattern without wildcard", Permission{Subject: "foo.bar.baz"}, Permission{Subject: "foo.bar"}, false},

		// Queue handling logic tests
		// - if only subject has queue but not pattern, return current result of isCoveredBy
		{"subject queue only - match", Permission{Subject: "foo", Queue: "q1"}, Permission{Subject: "foo"}, true},
		{"subject queue only - wildcard match", Permission{Subject: "foo.bar", Queue: "q1"}, Permission{Subject: "foo.>"}, true},
		{"subject queue only - no match", Permission{Subject: "foo", Queue: "q1"}, Permission{Subject: "bar"}, false},

		// - if both, subject and pattern, have a queue return false if they queues do not match. If the queues match, return current result of isCoveredBy
		{"both queue - match", Permission{Subject: "foo", Queue: "q1"}, Permission{Subject: "foo", Queue: "q1"}, true},
		{"both queue - diff queue", Permission{Subject: "foo", Queue: "q1"}, Permission{Subject: "foo", Queue: "q2"}, false},
		{"both queue - same queue subject mismatch", Permission{Subject: "foo", Queue: "q1"}, Permission{Subject: "bar", Queue: "q1"}, false},

		// - if only pattern has a queue, return false.
		{"pattern queue only - subject match", Permission{Subject: "foo"}, Permission{Subject: "foo", Queue: "q1"}, false},
		{"pattern queue only - no match", Permission{Subject: "bar"}, Permission{Subject: "foo", Queue: "q1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCoveredBy(tt.subject, tt.pattern); got != tt.want {
				t.Errorf("isCoveredBy(%v, %v) = %v, want %v", tt.subject, tt.pattern, got, tt.want)
			}
		})
	}
}

func TestDeduplicateWithWildcards(t *testing.T) {
	tests := []struct {
		name   string
		input  []Permission
		expect []Permission
	}{
		{
			name:   "no duplicates",
			input:  []Permission{{Subject: "foo.bar"}, {Subject: "baz.qux"}},
			expect: []Permission{{Subject: "baz.qux"}, {Subject: "foo.bar"}},
		},
		{
			name:   "exact duplicate",
			input:  []Permission{{Subject: "foo.bar"}, {Subject: "foo.bar"}},
			expect: []Permission{{Subject: "foo.bar"}},
		},
		{
			name:   "star covers specific",
			input:  []Permission{{Subject: "foo.bar"}, {Subject: "foo.*"}},
			expect: []Permission{{Subject: "foo.*"}},
		},
		{
			name:   "gt covers multiple",
			input:  []Permission{{Subject: "foo.bar"}, {Subject: "foo.baz"}, {Subject: "foo.>"}},
			expect: []Permission{{Subject: "foo.>"}},
		},
		{
			name:   "queue logic",
			input:  []Permission{{Subject: "foo", Queue: "q1"}, {Subject: "foo"}}, // foo covers foo q1
			expect: []Permission{{Subject: "foo"}},
		},
		{
			name:   "queue logic distinct",
			input:  []Permission{{Subject: "foo", Queue: "q1"}, {Subject: "foo", Queue: "q2"}}, // distinct queues
			expect: []Permission{{Subject: "foo", Queue: "q1"}, {Subject: "foo", Queue: "q2"}},
		},
		{
			name:   "queue logic pattern queue",
			input:  []Permission{{Subject: "foo"}, {Subject: "foo", Queue: "q1"}}, // foo covers foo q1
			expect: []Permission{{Subject: "foo"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert to map
			input := make(map[Permission]struct{})
			for _, s := range tt.input {
				input[s] = struct{}{}
			}

			result := deduplicateWithWildcards(input)

			// Convert back to slice for comparison, sorted
			got := make([]Permission, 0, len(result))
			for s := range result {
				got = append(got, s)
			}
			sort.Slice(got, func(i, j int) bool {
				if got[i].Subject != got[j].Subject {
					return got[i].Subject < got[j].Subject
				}
				return got[i].Queue < got[j].Queue
			})

			// Sort expected
			sort.Slice(tt.expect, func(i, j int) bool {
				if tt.expect[i].Subject != tt.expect[j].Subject {
					return tt.expect[i].Subject < tt.expect[j].Subject
				}
				return tt.expect[i].Queue < tt.expect[j].Queue
			})

			if !reflect.DeepEqual(got, tt.expect) {
				t.Errorf("deduplicateWithWildcards() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestNatsPermissions_Allow(t *testing.T) {
	p := NewNatsPermissions()

	p.Allow(Permission{Type: PermPub, Subject: "orders"})
	p.Allow(Permission{Type: PermSub, Subject: "events"})
	p.Allow(Permission{Type: PermSub, Subject: "tasks", Queue: "workergroup.>"})

	p.Deduplicate()

	pubList := p.PubList()
	if len(pubList) != 1 || pubList[0].Subject != "orders" {
		t.Errorf("Allow(PermPub) failed: got %v", pubList)
	}

	subList := p.SubList()
	// Should have both "events" and "tasks" (queue: workergroup.>)
	// Sorted: events < tasks
	if len(subList) != 2 {
		t.Errorf("Allow(PermSub) failed expected 2 items: got %v", subList)
	}
	if subList[0].Subject != "events" {
		t.Errorf("Expected first sub 'events', got %v", subList[0])
	}
	if subList[1].Subject != "tasks" || subList[1].Queue != "workergroup.>" {
		t.Errorf("Expected second sub 'tasks' queue 'workergroup.>', got %v", subList[1])
	}

	// Queue permissions are subject-ified for JWTs
	jwtPerms := p.ToNatsJWT()
	foundTask := false
	for _, s := range jwtPerms.Sub.Allow {
		if s == "tasks workergroup.>" {
			foundTask = true
			break
		}
	}
	if !foundTask {
		t.Errorf("Allow(PermSub with queue) failed: tasks not found in JWT permissions: %v", jwtPerms.Sub.Allow)
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
	if len(pubList) != 1 || pubList[0].Subject != "orders.>" {
		t.Errorf("Pub dedup failed: got %v, want [orders.>]", pubList)
	}

	subList := p.SubList()
	if len(subList) != 1 || subList[0].Subject != "_INBOX.>" {
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

// TestToNatsJWT verifies conversion to NATS JWT permissions.
// Checks empty deny-all behavior and queue subscription handling.
func TestToNatsJWT(t *testing.T) {
	tests := []struct {
		name         string
		perms        *NatsPermissions
		wantPubAllow []string
		wantPubDeny  []string
		wantSubAllow []string
		wantSubDeny  []string
	}{
		{
			name:         "empty permissions should deny all pub and sub",
			perms:        NewNatsPermissions(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: nil,
			wantSubDeny:  []string{">"},
		},
		{
			name: "pub only should deny all sub",
			perms: func() *NatsPermissions {
				p := NewNatsPermissions()
				p.Allow(Permission{Type: PermPub, Subject: "foo.>"})
				return p
			}(),
			wantPubAllow: []string{"foo.>"},
			wantPubDeny:  nil,
			wantSubAllow: nil,
			wantSubDeny:  []string{">"},
		},
		{
			name: "sub only should deny all pub",
			perms: func() *NatsPermissions {
				p := NewNatsPermissions()
				p.Allow(Permission{Type: PermSub, Subject: "foo.>"})
				return p
			}(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: []string{"foo.>"},
			wantSubDeny:  nil,
		},
		{
			name: "queue sub should be added to allow list",
			perms: func() *NatsPermissions {
				p := NewNatsPermissions()
				p.Allow(Permission{Type: PermSub, Subject: "q.sub", Queue: "workers"})
				return p
			}(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: []string{"q.sub workers"},
			wantSubDeny:  nil,
		},
		{
			name: "mixed regular and queue sub should be deduplicated",
			perms: func() *NatsPermissions {
				p := NewNatsPermissions()
				p.Allow(Permission{Type: PermSub, Subject: "foo"})
				p.Allow(Permission{Type: PermSub, Subject: "foo", Queue: "q1"})
				p.Allow(Permission{Type: PermSub, Subject: "bar", Queue: "q2"})
				return p
			}(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: []string{"bar q2", "foo"},
			wantSubDeny:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.perms.Deduplicate()
			got := tt.perms.ToNatsJWT()

			if !stringSliceEqual(got.Pub.Allow, tt.wantPubAllow) {
				t.Errorf("Pub.Allow = %v, want %v", got.Pub.Allow, tt.wantPubAllow)
			}
			if !stringSliceEqual(got.Pub.Deny, tt.wantPubDeny) {
				t.Errorf("Pub.Deny = %v, want %v", got.Pub.Deny, tt.wantPubDeny)
			}
			if !stringSliceEqual(got.Sub.Allow, tt.wantSubAllow) {
				t.Errorf("Sub.Allow = %v, want %v", got.Sub.Allow, tt.wantSubAllow)
			}
			if !stringSliceEqual(got.Sub.Deny, tt.wantSubDeny) {
				t.Errorf("Sub.Deny = %v, want %v", got.Sub.Deny, tt.wantSubDeny)
			}
		})
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
