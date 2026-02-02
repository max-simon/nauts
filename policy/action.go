// Package policy defines types for nauts policies, statements, and actions.
package policy

// Action represents an action that can be performed on a NATS resource.
// It is a string type for JSON compatibility.
type Action string

// ActionDef defines the attributes of an action.
type ActionDef struct {
	Name          string   // Action name (e.g., "nats.pub")
	IsAtomic      bool     // true for atomic actions, false for groups
	RequiresInbox bool     // true if needs _INBOX.> subscription
	ExpandsTo     []Action // for groups: list of actions to expand to
}

// Core NATS actions
const (
	ActionNATSPub Action = "nats.pub" // Publish messages to subjects
	ActionNATSSub Action = "nats.sub" // Subscribe to subjects (including queues)
	ActionNATSReq Action = "nats.req" // Request/reply pattern
)

// JetStream actions
const (
	ActionJSReadStream     Action = "js.readStream"     // Get stream info, list streams
	ActionJSWriteStream    Action = "js.writeStream"    // Create, update, seal, or purge stream
	ActionJSDeleteStream   Action = "js.deleteStream"   // Delete stream and all data
	ActionJSReadConsumer   Action = "js.readConsumer"   // Get consumer info, list consumers
	ActionJSWriteConsumer  Action = "js.writeConsumer"  // Create or update consumer
	ActionJSDeleteConsumer Action = "js.deleteConsumer" // Delete consumer
	ActionJSConsume        Action = "js.consume"        // Fetch messages and acknowledge
)

// KV actions
const (
	ActionKVRead         Action = "kv.read"         // Get key values, bucket info
	ActionKVWrite        Action = "kv.write"        // Write key values
	ActionKVWatchBucket  Action = "kv.watchBucket"  // Watch for changes, list keys
	ActionKVReadBucket   Action = "kv.readBucket"   // Get bucket info
	ActionKVWriteBucket  Action = "kv.writeBucket"  // Create or update bucket
	ActionKVDeleteBucket Action = "kv.deleteBucket" // Delete KV bucket
)

// Action groups
const (
	ActionGroupNATSAll  Action = "nats.*"    // All nats.* actions
	ActionGroupJSViewer Action = "js.viewer" // js.readStream, js.readConsumer
	ActionGroupJSWorker Action = "js.worker" // js.viewer + js.writeConsumer + js.consume
	ActionGroupJSAll    Action = "js.*"      // All js.* actions
	ActionGroupKVReader Action = "kv.reader" // kv.read, kv.watchBucket
	ActionGroupKVWriter Action = "kv.writer" // kv.reader + kv.write
	ActionGroupKVAll    Action = "kv.*"      // All kv.* actions
)

// actionRegistry maps action names to their definitions.
var actionRegistry = map[Action]*ActionDef{
	// Core NATS actions
	ActionNATSPub: {
		Name:          "nats.pub",
		IsAtomic:      true,
		RequiresInbox: false,
	},
	ActionNATSSub: {
		Name:          "nats.sub",
		IsAtomic:      true,
		RequiresInbox: false,
	},
	ActionNATSReq: {
		Name:          "nats.req",
		IsAtomic:      true,
		RequiresInbox: false, // nats.req handles inbox itself in mapping
	},

	// JetStream actions (all require inbox for request/reply)
	ActionJSReadStream: {
		Name:          "js.readStream",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSWriteStream: {
		Name:          "js.writeStream",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSDeleteStream: {
		Name:          "js.deleteStream",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSReadConsumer: {
		Name:          "js.readConsumer",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSWriteConsumer: {
		Name:          "js.writeConsumer",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSDeleteConsumer: {
		Name:          "js.deleteConsumer",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionJSConsume: {
		Name:          "js.consume",
		IsAtomic:      true,
		RequiresInbox: true,
	},

	// KV actions (all require inbox for request/reply)
	ActionKVRead: {
		Name:          "kv.read",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionKVWrite: {
		Name:          "kv.write",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionKVWatchBucket: {
		Name:          "kv.watchBucket",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionKVReadBucket: {
		Name:          "kv.readBucket",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionKVWriteBucket: {
		Name:          "kv.writeBucket",
		IsAtomic:      true,
		RequiresInbox: true,
	},
	ActionKVDeleteBucket: {
		Name:          "kv.deleteBucket",
		IsAtomic:      true,
		RequiresInbox: true,
	},

	// Action groups
	ActionGroupNATSAll: {
		Name:     "nats.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionNATSPub,
			ActionNATSSub,
			ActionNATSReq,
		},
	},
	ActionGroupJSViewer: {
		Name:     "js.viewer",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionJSReadStream,
			ActionJSReadConsumer,
		},
	},
	ActionGroupJSWorker: {
		Name:     "js.worker",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionGroupJSViewer, // Will be expanded recursively
			ActionJSWriteConsumer,
			ActionJSConsume,
		},
	},
	ActionGroupJSAll: {
		Name:     "js.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionJSReadStream,
			ActionJSWriteStream,
			ActionJSDeleteStream,
			ActionJSReadConsumer,
			ActionJSWriteConsumer,
			ActionJSDeleteConsumer,
			ActionJSConsume,
		},
	},
	ActionGroupKVReader: {
		Name:     "kv.reader",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionKVRead,
			ActionKVWatchBucket,
		},
	},
	ActionGroupKVWriter: {
		Name:     "kv.writer",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionGroupKVReader, // Will be expanded recursively
			ActionKVWrite,
		},
	},
	ActionGroupKVAll: {
		Name:     "kv.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionKVRead,
			ActionKVWrite,
			ActionKVWatchBucket,
			ActionKVReadBucket,
			ActionKVWriteBucket,
			ActionKVDeleteBucket,
		},
	},
}

// Def returns the action definition, or nil if the action is not valid.
func (a Action) Def() *ActionDef {
	return actionRegistry[a]
}

// IsGroup returns true if the action is an action group.
func (a Action) IsGroup() bool {
	def := a.Def()
	return def != nil && def.ExpandsTo != nil
}

// IsAtomic returns true if the action is an atomic (non-group) action.
func (a Action) IsAtomic() bool {
	def := a.Def()
	return def != nil && def.IsAtomic
}

// IsValid returns true if the action is a valid action or group.
func (a Action) IsValid() bool {
	return a.Def() != nil
}

// RequiresInbox returns true if the action requires implicit SUB _INBOX.> permission.
// This is true for all js.* and kv.* actions which use request/reply.
func (a Action) RequiresInbox() bool {
	def := a.Def()
	return def != nil && def.RequiresInbox
}

// ResolveActions expands action groups into their atomic actions.
// The result is deduplicated and contains only atomic actions.
func ResolveActions(actions []Action) []Action {
	seen := make(map[Action]bool)
	var result []Action

	var expand func(a Action)
	expand = func(a Action) {
		def := a.Def()
		if def == nil {
			return // Invalid action, skip
		}

		// If it's an atomic action, add it
		if def.IsAtomic {
			if !seen[a] {
				seen[a] = true
				result = append(result, a)
			}
			return
		}

		// If it's a group, expand it
		if def.ExpandsTo != nil {
			for _, action := range def.ExpandsTo {
				expand(action)
			}
		}
	}

	for _, action := range actions {
		expand(action)
	}

	return result
}

// AnyRequiresInbox returns true if any of the actions requires inbox subscription.
func AnyRequiresInbox(actions []Action) bool {
	for _, a := range actions {
		if a.RequiresInbox() {
			return true
		}
	}
	return false
}
