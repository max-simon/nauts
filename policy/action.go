// Package policy defines types for nauts policies, statements, and actions.
package policy

// Action represents an action that can be performed on a NATS resource.
// It is a string type for JSON compatibility.
type Action string

// ActionDef defines the attributes of an action.
type ActionDef struct {
	Name      string   // Action name (e.g., "nats.pub")
	IsAtomic  bool     // true for atomic actions, false for groups
	ExpandsTo []Action // for groups: list of actions to expand to
}

// Core NATS actions
const (
	ActionNATSPub     Action = "nats.pub"     // Publish messages to subjects
	ActionNATSSub     Action = "nats.sub"     // Subscribe to subjects (including queues)
	ActionNATSService Action = "nats.service" // Subscribe subject and allow_responses
)

// JetStream actions
const (
	ActionJSManage  Action = "js.manage"  // Manage streams (create, update, delete, purge)
	ActionJSView    Action = "js.view"    // View stream and consumer info
	ActionJSConsume Action = "js.consume" // Fetch messages and acknowledge
)

// KV actions
const (
	ActionKVRead   Action = "kv.read"   // Get key values, watch keys
	ActionKVEdit   Action = "kv.edit"   // Write key values
	ActionKVView   Action = "kv.view"   // View bucket info
	ActionKVManage Action = "kv.manage" // Manage buckets
)

// Action groups
const (
	ActionGroupNATSAll Action = "nats.*" // All nats.* actions
	ActionGroupJSAll   Action = "js.*"   // js.manage
	ActionGroupKVAll   Action = "kv.*"   // kv.manage
)

// actionRegistry maps action names to their definitions.
var actionRegistry = map[Action]*ActionDef{
	// Core NATS actions
	ActionNATSPub: {
		Name:     "nats.pub",
		IsAtomic: true,
	},
	ActionNATSSub: {
		Name:     "nats.sub",
		IsAtomic: true,
	},
	ActionNATSService: {
		Name:     "nats.service",
		IsAtomic: true,
	},

	// JetStream actions (all require inbox for request/reply)
	ActionJSManage: {
		Name:     "js.manage",
		IsAtomic: true,
	},
	ActionJSView: {
		Name:     "js.view",
		IsAtomic: true,
	},
	ActionJSConsume: {
		Name:     "js.consume",
		IsAtomic: true,
	},

	// KV actions (all require inbox for request/reply)
	ActionKVRead: {
		Name:     "kv.read",
		IsAtomic: true,
	},
	ActionKVEdit: {
		Name:     "kv.edit",
		IsAtomic: true,
	},
	ActionKVView: {
		Name:     "kv.view",
		IsAtomic: true,
	},
	ActionKVManage: {
		Name:     "kv.manage",
		IsAtomic: true,
	},

	// Action groups
	ActionGroupNATSAll: {
		Name:     "nats.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionNATSPub,
			ActionNATSSub,
			ActionNATSService,
		},
	},
	ActionGroupJSAll: {
		Name:     "js.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionJSManage,
		},
	},
	ActionGroupKVAll: {
		Name:     "kv.*",
		IsAtomic: false,
		ExpandsTo: []Action{
			ActionKVManage,
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

// Check if an action requires Jetstream info
func (a Action) RequiresJetstream() bool {
	switch a {
	case ActionJSConsume, ActionJSManage, ActionJSView, ActionKVRead, ActionKVEdit, ActionKVView, ActionKVManage:
		return true
	default:
		return false
	}
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
