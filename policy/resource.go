// Package policy provides policy-related types and functions for nauts.
// This file contains Resource (NATS Resource Name) parsing and validation.
package policy

import (
	"strings"
)

// ResourceType represents the type of a NATS resource.
// Basic types are "nats", "js", "kv". Full types include subidentifier variants.
type ResourceType string

// Basic resource types (without subidentifier)
const (
	ResourceTypeNATS ResourceType = "nats"
	ResourceTypeJS   ResourceType = "js"
	ResourceTypeKV   ResourceType = "kv"
)

// Full resource types (including subidentifier variants)
const (
	// NATS resources
	ResourceTypeNATSSubject      ResourceType = "nats:subject"       // nats:<subject>
	ResourceTypeNATSSubjectQueue ResourceType = "nats:subject:queue" // nats:<subject>:<queue>

	// JetStream resources
	ResourceTypeJSStream         ResourceType = "js:stream"          // js:<stream>
	ResourceTypeJSStreamConsumer ResourceType = "js:stream:consumer" // js:<stream>:<consumer>

	// KV resources
	ResourceTypeKVBucket      ResourceType = "kv:bucket"       // kv:<bucket>
	ResourceTypeKVBucketEntry ResourceType = "kv:bucket:entry" // kv:<bucket>:<key>
)

// IsValid checks if the type is a valid resource type (nats, js, kv).
func (t ResourceType) IsValid() bool {
	switch t {
	case ResourceTypeNATS, ResourceTypeJS, ResourceTypeKV:
		return true
	default:
		return false
	}
}

// Resource represents a parsed NATS Resource Name.
type Resource struct {
	Type          ResourceType // The basic resource type (nats, js, kv)
	Identifier    string       // Primary identifier (subject, stream, bucket)
	SubIdentifier string       // Optional sub-identifier (queue, consumer, key)
	Raw           string       // Original raw resource string
}

// HasSubIdentifier returns true if the NRN has a sub-identifier.
func (n *Resource) HasSubIdentifier() bool {
	return n.SubIdentifier != ""
}

// String returns the NRN as a string.
func (n *Resource) String() string {
	if n.SubIdentifier != "" {
		return string(n.Type) + ":" + n.Identifier + ":" + n.SubIdentifier
	}
	return string(n.Type) + ":" + n.Identifier
}

// FullType returns the full resource type including subidentifier variant.
// For example, "nats:orders:workers" returns ResourceTypeNATSSubjectQueue.
func (n *Resource) FullType() ResourceType {
	switch n.Type {
	case ResourceTypeNATS:
		if n.SubIdentifier != "" {
			return ResourceTypeNATSSubjectQueue
		}
		return ResourceTypeNATSSubject
	case ResourceTypeJS:
		if n.SubIdentifier != "" {
			return ResourceTypeJSStreamConsumer
		}
		return ResourceTypeJSStream
	case ResourceTypeKV:
		if n.SubIdentifier != "" {
			return ResourceTypeKVBucketEntry
		}
		return ResourceTypeKVBucket
	default:
		return n.Type
	}
}

// IsSubject returns true if this is a NATS subject resource.
func (n *Resource) IsSubject() bool {
	return n.Type == ResourceTypeNATS
}

// IsStream returns true if this is a JetStream stream resource.
func (n *Resource) IsStream() bool {
	return n.Type == ResourceTypeJS
}

// IsBucket returns true if this is a KV bucket resource.
func (n *Resource) IsBucket() bool {
	return n.Type == ResourceTypeKV
}

// ParseResource parses a string into an NRN.
// It validates the format but does not validate wildcards.
// Use ParseAndValidateResource for full validation.
func ParseResource(s string) (*Resource, error) {
	if s == "" {
		return nil, NewResourceError(s, "empty resource", ErrInvalidResource)
	}

	// Split by colon - we expect 2 or 3 parts
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil, NewResourceError(s, "missing type or identifier", ErrInvalidResource)
	}

	nrnType := ResourceType(parts[0])
	if !nrnType.IsValid() {
		return nil, NewResourceError(s, "unknown type: "+parts[0], ErrUnknownResourceType)
	}

	identifier := parts[1]
	if identifier == "" {
		return nil, NewResourceError(s, "empty identifier", ErrInvalidResource)
	}

	var subIdentifier string
	if len(parts) == 3 {
		subIdentifier = parts[2]
		// Sub-identifier can be empty for patterns like "kv:bucket:"
		// but we'll treat this as invalid
		if subIdentifier == "" {
			return nil, NewResourceError(s, "empty sub-identifier", ErrInvalidResource)
		}
	}

	return &Resource{
		Type:          nrnType,
		Identifier:    identifier,
		SubIdentifier: subIdentifier,
		Raw:           s,
	}, nil
}

// ParseAndValidateResource parses and validates a resource string including wildcard rules.
func ParseAndValidateResource(s string) (*Resource, error) {
	nrn, err := ParseResource(s)
	if err != nil {
		return nil, err
	}

	if err := ValidateResource(nrn); err != nil {
		return nil, err
	}

	return nrn, nil
}

// MustParseResource parses a resource string and panics on error.
// Use only for compile-time constants.
func MustParseResource(s string) *Resource {
	nrn, err := ParseAndValidateResource(s)
	if err != nil {
		panic(err)
	}
	return nrn
}

// ValidateResource validates an NRN's wildcard usage according to type-specific rules.
//
// Wildcard rules:
//   - nats: * and > allowed in subject; * only in queue
//   - js: * only in stream and consumer; no >
//   - kv: * in bucket and key; > only in key
func ValidateResource(n *Resource) error {
	switch n.Type {
	case ResourceTypeNATS:
		return validateNATSResource(n)
	case ResourceTypeJS:
		return validateJSResource(n)
	case ResourceTypeKV:
		return validateKVResource(n)
	default:
		return NewResourceError(n.Raw, "unknown type", ErrUnknownResourceType)
	}
}

// validateNATSNRN validates NATS subject NRNs.
// Rules:
//   - Subject: both * and > wildcards allowed
//   - Queue: only * wildcard allowed (no >)
func validateNATSResource(n *Resource) error {
	// Subject can have * and >
	if err := validateWildcards(n.Identifier, true, true); err != nil {
		return NewResourceError(n.Raw, "invalid subject: "+err.Error(), ErrInvalidWildcard)
	}

	// Queue can only have *
	if n.SubIdentifier != "" {
		if err := validateWildcards(n.SubIdentifier, true, false); err != nil {
			return NewResourceError(n.Raw, "invalid queue: "+err.Error(), ErrInvalidWildcard)
		}
	}

	return nil
}

// validateJSNRN validates JetStream stream/consumer NRNs.
// Rules:
//   - Stream: only * wildcard allowed (no >)
//   - Consumer: only * wildcard allowed (no >)
func validateJSResource(n *Resource) error {
	// Stream can only have *
	if err := validateWildcards(n.Identifier, true, false); err != nil {
		return NewResourceError(n.Raw, "invalid stream: "+err.Error(), ErrInvalidWildcard)
	}

	// Consumer can only have *
	if n.SubIdentifier != "" {
		if err := validateWildcards(n.SubIdentifier, true, false); err != nil {
			return NewResourceError(n.Raw, "invalid consumer: "+err.Error(), ErrInvalidWildcard)
		}
	}

	return nil
}

// validateKVNRN validates KV bucket/key NRNs.
// Rules:
//   - Bucket: only * wildcard allowed (no >)
//   - Key: both * and > wildcards allowed
func validateKVResource(n *Resource) error {
	// Bucket can only have *
	if err := validateWildcards(n.Identifier, true, false); err != nil {
		return NewResourceError(n.Raw, "invalid bucket: "+err.Error(), ErrInvalidWildcard)
	}

	// Key can have * and >
	if n.SubIdentifier != "" {
		if err := validateWildcards(n.SubIdentifier, true, true); err != nil {
			return NewResourceError(n.Raw, "invalid key: "+err.Error(), ErrInvalidWildcard)
		}
	}

	return nil
}

// validateWildcards checks if a value contains valid wildcards.
func validateWildcards(value string, allowStar, allowGT bool) error {
	// Skip validation for template variables - they will be validated after interpolation
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		return nil
	}

	if !allowStar && strings.Contains(value, "*") {
		return ErrInvalidWildcard
	}

	if !allowGT && strings.Contains(value, ">") {
		return ErrInvalidWildcard
	}

	// Validate > placement - must be at the end of a token
	if strings.Contains(value, ">") {
		if err := validateGTPlacement(value); err != nil {
			return err
		}
	}

	return nil
}

// validateGTPlacement ensures > is only used as a terminal wildcard.
// Valid: "foo.>" or ">"
// Invalid: ">.foo" or "foo.>.bar"
func validateGTPlacement(value string) error {
	tokens := strings.Split(value, ".")
	for i, token := range tokens {
		if token == ">" {
			// > must be the last token
			if i != len(tokens)-1 {
				return ErrInvalidWildcard
			}
		} else if strings.Contains(token, ">") {
			// > must be the entire token, not part of it
			return ErrInvalidWildcard
		}
	}
	return nil
}

// HasWildcard returns true if the NRN contains any wildcards.
func HasWildcard(n *Resource) bool {
	return strings.ContainsAny(n.Identifier, "*>") ||
		strings.ContainsAny(n.SubIdentifier, "*>")
}
