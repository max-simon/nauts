package policy

import (
	"testing"
)

func TestAction_Def(t *testing.T) {
	tests := []struct {
		action     Action
		wantNil    bool
		wantAtomic bool
	}{
		{ActionNATSPub, false, true},
		{ActionJSConsume, false, true},
		{ActionJSManage, false, true},
		{ActionGroupJSAll, false, false},
		{Action("invalid"), true, false},
		{Action(""), true, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			def := tt.action.Def()
			if (def == nil) != tt.wantNil {
				t.Errorf("Action(%q).Def() nil = %v, want %v", tt.action, def == nil, tt.wantNil)
				return
			}
			if def != nil {
				if def.IsAtomic != tt.wantAtomic {
					t.Errorf("Action(%q).Def().IsAtomic = %v, want %v", tt.action, def.IsAtomic, tt.wantAtomic)
				}
			}
		})
	}
}

func TestAction_IsAtomic(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		{ActionNATSPub, true},
		{ActionNATSSub, true},
		{ActionNATSService, true},
		{ActionJSManage, true},
		{ActionJSView, true},
		{ActionJSConsume, true},
		{ActionKVRead, true},
		{ActionKVEdit, true},
		{ActionKVView, true},
		{ActionKVManage, true},
		{ActionGroupNATSAll, false},
		{ActionGroupJSAll, false},
		{ActionGroupKVAll, false},
		{Action("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsAtomic(); got != tt.want {
				t.Errorf("Action(%q).IsAtomic() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestAction_IsGroup(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		{ActionGroupNATSAll, true},
		{ActionGroupJSAll, true},
		{ActionGroupKVAll, true},
		{ActionNATSPub, false},
		{ActionJSConsume, false},
		{ActionKVRead, false},
		{Action("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsGroup(); got != tt.want {
				t.Errorf("Action(%q).IsGroup() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestAction_IsValid(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		{ActionNATSPub, true},
		{ActionJSConsume, true},
		{ActionKVRead, true},
		{ActionGroupNATSAll, true},
		{ActionJSView, true},
		{ActionGroupKVAll, true},
		{Action("invalid"), false},
		{Action(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("Action(%q).IsValid() = %v, want %v", tt.action, got, tt.want)
			}
		})
	}
}

func TestResolveActions(t *testing.T) {
	tests := []struct {
		name   string
		input  []Action
		want   []Action
		length int
	}{
		{
			name:   "single atomic action",
			input:  []Action{ActionNATSPub},
			want:   []Action{ActionNATSPub},
			length: 1,
		},
		{
			name:   "multiple atomic actions",
			input:  []Action{ActionNATSPub, ActionNATSSub},
			want:   []Action{ActionNATSPub, ActionNATSSub},
			length: 2,
		},
		{
			name:   "deduplicate atomic actions",
			input:  []Action{ActionNATSPub, ActionNATSPub, ActionNATSSub},
			length: 2,
		},
		{
			name:   "expand nats.* group",
			input:  []Action{ActionGroupNATSAll},
			length: 3, // pub, sub, req
		},
		{
			name:   "expand js.* group",
			input:  []Action{ActionGroupJSAll},
			length: 1, // manage
		},
		{
			name:   "expand kv.* group",
			input:  []Action{ActionGroupKVAll},
			length: 1, // manage
		},
		{
			name:   "mixed atomic and group with overlap",
			input:  []Action{ActionJSManage, ActionGroupJSAll},
			length: 1, // manage (deduplicated)
		},
		{
			name:   "empty input",
			input:  []Action{},
			length: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveActions(tt.input)

			if len(got) != tt.length {
				t.Errorf("ResolveActions() length = %v, want %v", len(got), tt.length)
			}

			// Verify all results are atomic
			for _, a := range got {
				if !a.IsAtomic() {
					t.Errorf("ResolveActions() contains non-atomic action: %v", a)
				}
			}

			// Check specific expected results if provided
			if tt.want != nil {
				for i, w := range tt.want {
					if got[i] != w {
						t.Errorf("ResolveActions()[%d] = %v, want %v", i, got[i], w)
					}
				}
			}
		})
	}
}
