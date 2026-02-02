package policy

import (
	"testing"
)

func TestAction_Def(t *testing.T) {
	tests := []struct {
		action     Action
		wantNil    bool
		wantAtomic bool
		wantInbox  bool
	}{
		{ActionNATSPub, false, true, false},
		{ActionJSConsume, false, true, true},
		{ActionGroupJSViewer, false, false, false},
		{Action("invalid"), true, false, false},
		{Action(""), true, false, false},
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
				if def.RequiresInbox != tt.wantInbox {
					t.Errorf("Action(%q).Def().RequiresInbox = %v, want %v", tt.action, def.RequiresInbox, tt.wantInbox)
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
		{ActionNATSReq, true},
		{ActionJSReadStream, true},
		{ActionJSWriteStream, true},
		{ActionJSDeleteStream, true},
		{ActionJSReadConsumer, true},
		{ActionJSWriteConsumer, true},
		{ActionJSDeleteConsumer, true},
		{ActionJSConsume, true},
		{ActionKVRead, true},
		{ActionKVWrite, true},
		{ActionKVWatchBucket, true},
		{ActionKVReadBucket, true},
		{ActionKVWriteBucket, true},
		{ActionKVDeleteBucket, true},
		{ActionGroupNATSAll, false},
		{ActionGroupJSViewer, false},
		{ActionGroupJSWorker, false},
		{ActionGroupJSAll, false},
		{ActionGroupKVReader, false},
		{ActionGroupKVWriter, false},
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
		{ActionGroupJSViewer, true},
		{ActionGroupJSWorker, true},
		{ActionGroupJSAll, true},
		{ActionGroupKVReader, true},
		{ActionGroupKVWriter, true},
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
		{ActionGroupJSViewer, true},
		{ActionGroupKVWriter, true},
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

func TestAction_RequiresInbox(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		// NATS actions don't require inbox
		{ActionNATSPub, false},
		{ActionNATSSub, false},
		{ActionNATSReq, false}, // nats.req handles inbox itself

		// All JS actions require inbox
		{ActionJSReadStream, true},
		{ActionJSWriteStream, true},
		{ActionJSDeleteStream, true},
		{ActionJSReadConsumer, true},
		{ActionJSWriteConsumer, true},
		{ActionJSDeleteConsumer, true},
		{ActionJSConsume, true},

		// All KV actions require inbox
		{ActionKVRead, true},
		{ActionKVWrite, true},
		{ActionKVWatchBucket, true},
		{ActionKVReadBucket, true},
		{ActionKVWriteBucket, true},
		{ActionKVDeleteBucket, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.RequiresInbox(); got != tt.want {
				t.Errorf("Action(%q).RequiresInbox() = %v, want %v", tt.action, got, tt.want)
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
			name:   "expand js.viewer group",
			input:  []Action{ActionGroupJSViewer},
			length: 2, // readStream, readConsumer
		},
		{
			name:   "expand js.worker group (nested)",
			input:  []Action{ActionGroupJSWorker},
			length: 4, // readStream, readConsumer, writeConsumer, consume
		},
		{
			name:   "expand js.* group",
			input:  []Action{ActionGroupJSAll},
			length: 7, // all JS actions
		},
		{
			name:   "expand kv.reader group",
			input:  []Action{ActionGroupKVReader},
			length: 2, // read, watchBucket
		},
		{
			name:   "expand kv.writer group (nested)",
			input:  []Action{ActionGroupKVWriter},
			length: 3, // read, watchBucket, write
		},
		{
			name:   "expand kv.* group",
			input:  []Action{ActionGroupKVAll},
			length: 6, // all KV actions
		},
		{
			name:   "mixed atomic and group with overlap",
			input:  []Action{ActionJSReadStream, ActionGroupJSViewer},
			length: 2, // readStream, readConsumer (deduplicated)
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

func TestResolveActions_JSViewer(t *testing.T) {
	// Verify js.viewer contains exactly the right actions
	result := ResolveActions([]Action{ActionGroupJSViewer})
	expected := map[Action]bool{
		ActionJSReadStream:   true,
		ActionJSReadConsumer: true,
	}

	if len(result) != len(expected) {
		t.Errorf("js.viewer should expand to %d actions, got %d", len(expected), len(result))
	}

	for _, a := range result {
		if !expected[a] {
			t.Errorf("js.viewer should not contain %v", a)
		}
	}
}

func TestResolveActions_JSWorker(t *testing.T) {
	// Verify js.worker contains exactly the right actions
	result := ResolveActions([]Action{ActionGroupJSWorker})
	expected := map[Action]bool{
		ActionJSReadStream:    true,
		ActionJSReadConsumer:  true,
		ActionJSWriteConsumer: true,
		ActionJSConsume:       true,
	}

	if len(result) != len(expected) {
		t.Errorf("js.worker should expand to %d actions, got %d", len(expected), len(result))
	}

	for _, a := range result {
		if !expected[a] {
			t.Errorf("js.worker should not contain %v", a)
		}
	}
}

func TestResolveActions_KVWriter(t *testing.T) {
	// Verify kv.writer contains exactly the right actions
	result := ResolveActions([]Action{ActionGroupKVWriter})
	expected := map[Action]bool{
		ActionKVRead:        true,
		ActionKVWatchBucket: true,
		ActionKVWrite:       true,
	}

	if len(result) != len(expected) {
		t.Errorf("kv.writer should expand to %d actions, got %d", len(expected), len(result))
	}

	for _, a := range result {
		if !expected[a] {
			t.Errorf("kv.writer should not contain %v", a)
		}
	}
}

func TestAnyRequiresInbox(t *testing.T) {
	tests := []struct {
		name    string
		actions []Action
		want    bool
	}{
		{
			name:    "only NATS actions",
			actions: []Action{ActionNATSPub, ActionNATSSub},
			want:    false,
		},
		{
			name:    "includes JS action",
			actions: []Action{ActionNATSPub, ActionJSConsume},
			want:    true,
		},
		{
			name:    "includes KV action",
			actions: []Action{ActionKVRead},
			want:    true,
		},
		{
			name: "all JS actions",
			actions: []Action{
				ActionJSReadStream,
				ActionJSWriteStream,
				ActionJSDeleteStream,
				ActionJSReadConsumer,
				ActionJSWriteConsumer,
				ActionJSDeleteConsumer,
				ActionJSConsume,
			},
			want: true,
		},
		{
			name:    "empty",
			actions: []Action{},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := AnyRequiresInbox(tt.actions); got != tt.want {
				t.Errorf("AnyRequiresInbox() = %v, want %v", got, tt.want)
			}
		})
	}
}
