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
	case ActionNATSReq:
		return mapNATSReq(n)

	// JetStream actions
	case ActionJSReadStream:
		return mapJSReadStream(n)
	case ActionJSWriteStream:
		return mapJSWriteStream(n)
	case ActionJSDeleteStream:
		return mapJSDeleteStream(n)
	case ActionJSReadConsumer:
		return mapJSReadConsumer(n)
	case ActionJSWriteConsumer:
		return mapJSWriteConsumer(n)
	case ActionJSDeleteConsumer:
		return mapJSDeleteConsumer(n)
	case ActionJSConsume:
		return mapJSConsume(n)

	// KV actions
	case ActionKVRead:
		return mapKVRead(n)
	case ActionKVWrite:
		return mapKVWrite(n)
	case ActionKVWatchBucket:
		return mapKVWatchBucket(n)
	case ActionKVReadBucket:
		return mapKVReadBucket(n)
	case ActionKVWriteBucket:
		return mapKVWriteBucket(n)
	case ActionKVDeleteBucket:
		return mapKVDeleteBucket(n)

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

// mapNATSReq: nats.req → PUB <subject>, SUB _INBOX.>
func mapNATSReq(n *Resource) []Permission {
	if n.Type != ResourceTypeNATS {
		// return empty list of permissions
		return []Permission{}
	}
	return []Permission{
		{Type: PermPub, Subject: n.Identifier},
		{Type: PermSub, Subject: "_INBOX.>"},
	}
}

// === JetStream ===

// mapJSReadStream: js.readStream → $JS.API.STREAM.INFO.<stream>
// If stream is *, also grant $JS.API.STREAM.LIST and $JS.API.STREAM.NAMES
func mapJSReadStream(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	perms := []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.INFO." + stream},
	}

	// For wildcard, also grant list/names
	if stream == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.LIST"},
			Permission{Type: PermPub, Subject: "$JS.API.STREAM.NAMES"},
		)
	}

	return perms
}

// mapJSWriteStream: js.writeStream → $JS.API.STREAM.CREATE/UPDATE/PURGE.<stream>
func mapJSWriteStream(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.CREATE." + stream},
		{Type: PermPub, Subject: "$JS.API.STREAM.UPDATE." + stream},
		{Type: PermPub, Subject: "$JS.API.STREAM.PURGE." + stream},
	}
}

// mapJSDeleteStream: js.deleteStream → $JS.API.STREAM.DELETE.<stream>
func mapJSDeleteStream(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.DELETE." + stream},
	}
}

// mapJSReadConsumer: js.readConsumer → $JS.API.CONSUMER.INFO.<stream>.<consumer>
// If consumer is *, also grant $JS.API.CONSUMER.LIST/NAMES.<stream>
func mapJSReadConsumer(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	consumer := n.SubIdentifier
	if consumer == "" {
		consumer = "*"
	}

	perms := []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.INFO." + stream + "." + consumer},
	}

	// For wildcard consumer, also grant list/names
	if consumer == "*" {
		perms = append(perms,
			Permission{Type: PermPub, Subject: "$JS.API.CONSUMER.LIST." + stream},
			Permission{Type: PermPub, Subject: "$JS.API.CONSUMER.NAMES." + stream},
		)
	}

	return perms
}

// mapJSWriteConsumer: js.writeConsumer → $JS.API.CONSUMER.CREATE/DURABLE.CREATE
func mapJSWriteConsumer(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	consumer := n.SubIdentifier
	if consumer == "" {
		consumer = "*"
	}

	return []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE." + stream + "." + consumer + ".>"},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.DURABLE.CREATE." + stream + "." + consumer},
	}
}

// mapJSDeleteConsumer: js.deleteConsumer → $JS.API.CONSUMER.DELETE.<stream>.<consumer>
func mapJSDeleteConsumer(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	consumer := n.SubIdentifier
	if consumer == "" {
		consumer = "*"
	}

	return []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.DELETE." + stream + "." + consumer},
	}
}

// mapJSConsume: js.consume → $JS.API.CONSUMER.MSG.NEXT.<stream>.<consumer>, $JS.ACK.<stream>.<consumer>.>
func mapJSConsume(n *Resource) []Permission {
	if n.Type != ResourceTypeJS {
		// return empty list of permissions
		return []Permission{}
	}

	stream := n.Identifier
	consumer := n.SubIdentifier
	if consumer == "" {
		consumer = "*"
	}

	return []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.MSG.NEXT." + stream + "." + consumer},
		{Type: PermPub, Subject: "$JS.ACK." + stream + "." + consumer + ".>"},
	}
}

// === KV ===

// mapKVRead: kv.read → PUB $JS.API.DIRECT.GET.KV_<bucket>.$KV.<bucket>.<key>
func mapKVRead(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier
	key := n.SubIdentifier
	if key == "" {
		key = ">"
	}

	return []Permission{
		{Type: PermPub, Subject: "$JS.API.DIRECT.GET.KV_" + bucket + ".$KV." + bucket + "." + key},
	}
}

// mapKVWrite: kv.write → PUB $KV.<bucket>.<key>
func mapKVWrite(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier
	key := n.SubIdentifier
	if key == "" {
		key = ">"
	}

	return []Permission{
		{Type: PermPub, Subject: "$KV." + bucket + "." + key},
	}
}

// mapKVWatchBucket: kv.watchBucket → Consumer create/delete, SUB delivery, flow control
func mapKVWatchBucket(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier

	return []Permission{
		{Type: PermPub, Subject: "$JS.API.CONSUMER.CREATE.KV_" + bucket + ".*.$KV." + bucket + ".>"},
		{Type: PermPub, Subject: "$JS.API.CONSUMER.DELETE.KV_" + bucket + ".>"},
		{Type: PermSub, Subject: "_INBOX.>"},                   // For delivery
		{Type: PermPub, Subject: "$JS.FC.KV_" + bucket + ".>"}, // Flow control
	}
}

// mapKVReadBucket: kv.readBucket → PUB $JS.API.STREAM.INFO.KV_<bucket>
func mapKVReadBucket(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.INFO.KV_" + bucket},
	}
}

// mapKVWriteBucket: kv.writeBucket → PUB $JS.API.STREAM.CREATE.KV_<bucket>
func mapKVWriteBucket(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.CREATE.KV_" + bucket},
	}
}

// mapKVDeleteBucket: kv.deleteBucket → PUB $JS.API.STREAM.DELETE.KV_<bucket>
func mapKVDeleteBucket(n *Resource) []Permission {
	if n.Type != ResourceTypeKV {
		// return empty list of permissions
		return []Permission{}
	}

	bucket := n.Identifier
	return []Permission{
		{Type: PermPub, Subject: "$JS.API.STREAM.DELETE.KV_" + bucket},
	}
}
