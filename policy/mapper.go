// Package policy provides policy-related types and functions for nauts.
// This file contains action-to-permission mapping logic.
package policy

// MapActionToPermissions converts an action + NRN to NATS permissions.
// Returns a list of permissions that should be granted.
func MapActionToPermissions(action Action, n *Resource) []Permission {
	switch action {
	// Core NATS actions
	case ActionNATSPub:
		return mapNATSPub(n)
	case ActionNATSSub:
		return mapNATSSub(n)
	case ActionNATSService:
		return mapNATSService(n)

	// JetStream actions
	case ActionJSManage:
		return mapJSManage(n)
	case ActionJSView:
		return mapJSView(n)
	case ActionJSConsume:
		return mapJSConsume(n)

	// KV actions
	case ActionKVRead:
		return mapKVRead(n)
	case ActionKVEdit:
		return mapKVEdit(n)
	case ActionKVView:
		return mapKVView(n)
	case ActionKVManage:
		return mapKVManage(n)

	default:
		return []Permission{}
	}
}

// === Core NATS ===

// mapNATSPub: nats.pub → PUB <subject>
func mapNATSPub(n *Resource) []Permission {
	if n.Type != ResourceTypeNATS {
		// return empty list of permissions
		return []Permission{}
	}
	return []Permission{
		{Type: PermPub, Subject: n.Identifier},
	}
}

// mapNATSSub: nats.sub → SUB <subject> [queue=<queue>]
func mapNATSSub(n *Resource) []Permission {
	if n.Type != ResourceTypeNATS {
		// return empty list of permissions
		return []Permission{}
	}
	return []Permission{
		{Type: PermSub, Subject: n.Identifier, Queue: n.SubIdentifier},
	}
}

// mapNATSService: nats.service → SUB <subject> + allow responses
func mapNATSService(n *Resource) []Permission {
	if n.Type != ResourceTypeNATS {
		// return empty list of permissions
		return []Permission{}
	}
	return []Permission{
		{Type: PermSub, Subject: n.Identifier},
		{Type: PermResp},
	}
}

// === JetStream ===

// mapJSManage: js.manage
func mapJSManage(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		return []Permission{}
	}

	// js.manage includes all js.consume permissions
	perms := mapJSConsume(n)

	stream := n.Identifier
	if stream == "" {
		stream = "*"
	}

	perms = append(perms,
		Permission{Type: PermPub, Subject: "$JS.API.STREAM.*." + stream},
		Permission{Type: PermPub, Subject: "$JS.API.STREAM.MSG.*." + stream},
	)

	if stream == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.NAMES"},
		)
	}

	return perms
}

// mapJSView: js.view
func mapJSView(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		return []Permission{}
	}

	stream := n.Identifier
	if stream == "" {
		stream = "*"
	}

	consumerInfo := "$JS.API.CONSUMER.INFO." + stream + ".*"
	if stream == "*" {
		consumerInfo = "$JS.API.CONSUMER.INFO.*.*"
	}

	perms := []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.INFO." + stream},
		{Type: PermPub, Subject: consumerInfo},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.LIST." + stream},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.NAMES." + stream},
	}

	if stream == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.NAMES"},
		)
	}

	return perms
}

// mapJSConsume: js.consume
func mapJSConsume(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		return []Permission{}
	}

	stream := n.Identifier
	if stream == "" {
		stream = "*"
	}
	consumer := n.SubIdentifier

	// Specific consumer
	if consumer != "" && consumer != "*" {
		return []Permission{
			{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO." + stream + "." + consumer},
			{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE." + stream + "." + consumer},
			{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT." + stream + "." + consumer},
			{Type: PermPub, Subject: "$JS.ACK." + stream + "." + consumer + ".>"},
			{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE." + stream + ".*"},
			{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK." + stream + ".*"},
			{Type: PermPub, Subject: "$JS.FC." + stream + ".>"},
			{Type: PermPub, Subject: "$JS.API.DIRECT.GET." + stream},
			{Type: PermPub, Subject: "$JS.API.DIRECT.GET." + stream + ".>"},
		}
	}

	// Any consumer (including wildcard)
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.*." + stream},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.*." + stream + ".>"},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE." + stream + ".>"},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT." + stream + ".*"},
		{Type: PermPub, Subject: "$JS.ACK." + stream + ".>"},
		{Type: PermPub, Subject: "$JS.SNAPSHOT.RESTORE." + stream + ".*"},
		{Type: PermPub, Subject: "$JS.SNAPSHOT.ACK." + stream + ".*"},
		{Type: PermPub, Subject: "$JS.FC." + stream + ".>"},
		{Type: PermPub, Subject: "$JS.API.DIRECT.GET." + stream},
		{Type: PermPub, Subject: "$JS.API.DIRECT.GET." + stream + ".>"},
	}
}

// === KV ===

// mapKVRead: kv.read
func mapKVRead(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		return []Permission{}
	}

	bucket := n.Identifier
	key := n.SubIdentifier

	// Specific key
	if key != "" && key != ">" {
		return []Permission{
			{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_" + bucket},
			{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_" + bucket + ".$KV." + bucket + "." + key},
			{Type: PermSub, Subject: "$KV." + bucket + "." + key},
		}
	}

	// Any key
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_" + bucket},
		{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_" + bucket + ".$KV." + bucket + ".>"},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_" + bucket},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_" + bucket + ".>"},
		{Type: PermPub, Subject: "$JS.FC.KV_" + bucket + ".>"},
		{Type: PermSub, Subject: "$KV." + bucket + ".>"},
	}
}

// mapKVEdit: kv.edit
func mapKVEdit(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		return []Permission{}
	}

	perms := mapKVRead(n)
	bucket := n.Identifier
	key := n.SubIdentifier
	if key == "" {
		key = ">"
	}

	perms = append(perms, Permission{Type: PermPub, Subject: "$KV." + bucket + "." + key})
	return perms
}

// mapKVView: kv.view
func mapKVView(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		return []Permission{}
	}

	bucket := n.Identifier
	if bucket == "" {
		bucket = "*"
	}

	perms := []Permission{}

	if bucket != "*" {
		perms = append(perms, Permission{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_" + bucket})
	}

	if bucket == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.INFO.*"},
		)
	}

	return perms
}

// mapKVManage: kv.manage
func mapKVManage(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		return []Permission{}
	}

	// kv.manage includes all kv.read permissions
	perms := mapKVRead(n)
	bucket := n.Identifier
	if bucket == "" {
		bucket = "*"
	}

	if bucket != "*" {
		perms = append(perms, Permission{Type: PermPub, Subject: "$JS.API.STREAM.*.KV_" + bucket})
	}

	if bucket == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.INFO.*"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.*.*"},
		)
	}

	return perms
}
