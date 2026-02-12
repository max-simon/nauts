// Package policy provides policy-related types and functions for nauts.
// This file contains NATS permission types.
package policy

import (
	"fmt"
	"sort"
	"strings"

	natsjwt "github.com/nats-io/jwt/v2"
)

// PermissionType represents the type of NATS permission.
type PermissionType string

const (
	PermPub  PermissionType = "pub"  // Publish permission
	PermSub  PermissionType = "sub"  // Subscribe permission
	PermResp PermissionType = "resp" // Allow responses
)

// Permission represents a single NATS permission.
type Permission struct {
	Type    PermissionType `json:"type"`
	Subject string         `json:"subject"`
	Queue   string         `json:"queue,omitempty"` // Only for SUB permissions
}

func (p Permission) String() string {
	if p.Queue != "" {
		return p.Subject + " " + p.Queue
	}
	return p.Subject
}

// PermissionSet holds a set of allowed subjects with wildcard-aware deduplication.
type PermissionSet struct {
	allow map[Permission]struct{}
}

// NewPermissionSet creates a new empty PermissionSet.
func NewPermissionSet() *PermissionSet {
	return &PermissionSet{
		allow: make(map[Permission]struct{}),
	}
}

// Add adds a subject to the allow set.
func (ps *PermissionSet) Add(p Permission) {
	if ps.allow == nil {
		ps.allow = make(map[Permission]struct{})
	}
	ps.allow[p] = struct{}{}
}

// AllowList returns the allow set as a sorted slice.
func (ps *PermissionSet) AllowList() []Permission {
	if ps.allow == nil {
		return []Permission{}
	}
	result := make([]Permission, 0, len(ps.allow))
	for s := range ps.allow {
		result = append(result, s)
	}
	// Sort by Subject, then Queue
	sort.Slice(result, func(i, j int) bool {
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		return result[i].Queue < result[j].Queue
	})
	return result
}

// Deduplicate removes subjects that are covered by wildcards.
func (ps *PermissionSet) Deduplicate() {
	ps.allow = deduplicateWithWildcards(ps.allow)
}

// IsEmpty returns true if the permission set has no allowed subjects.
func (ps *PermissionSet) IsEmpty() bool {
	return len(ps.allow) == 0
}

func (ps *PermissionSet) String() string {
	allowList := ps.AllowList()
	strs := make([]string, len(allowList))
	for i, perm := range allowList {
		strs[i] = perm.String()
	}
	return strings.Join(strs, ", ")
}

// NatsPermissions holds compiled NATS permissions.
type NatsPermissions struct {
	pub            *PermissionSet
	sub            *PermissionSet
	allowResponses bool // If true, sets Resp permissions
}

// NewNatsPermissions creates an empty NatsPermissions struct.
func NewNatsPermissions() *NatsPermissions {
	return &NatsPermissions{
		pub:            NewPermissionSet(),
		sub:            NewPermissionSet(),
		allowResponses: false,
	}
}

// Clone returns a deep copy of the permissions.
func (p *NatsPermissions) Clone() *NatsPermissions {
	if p == nil {
		return nil
	}
	clone := NewNatsPermissions()
	clone.allowResponses = p.allowResponses
	if p.pub != nil {
		for perm := range p.pub.allow {
			clone.pub.Add(perm)
		}
	}
	if p.sub != nil {
		for perm := range p.sub.allow {
			clone.sub.Add(perm)
		}
	}
	return clone
}

// Allow adds a permission to the appropriate allow set.
func (p *NatsPermissions) Allow(perm Permission) {
	switch perm.Type {
	case PermPub:
		p.pub.Add(perm)
	case PermSub:
		p.sub.Add(perm)
	case PermResp:
		p.allowResponses = true
	}
}

// Merge combines another NatsPermissions into this one.
func (p *NatsPermissions) Merge(other *NatsPermissions) {
	if other == nil {
		return
	}

	if other.pub != nil {
		for s := range other.pub.allow {
			p.pub.Add(s)
		}
	}
	if other.sub != nil {
		for s := range other.sub.allow {
			p.sub.Add(s)
		}
	}
	if other.allowResponses {
		p.allowResponses = true
	}
}

// Deduplicate removes duplicate permissions using wildcard-aware deduplication.
func (p *NatsPermissions) Deduplicate() {
	p.pub.Deduplicate()
	p.sub.Deduplicate()
}

// IsEmpty returns true if there are no permissions.
func (p *NatsPermissions) IsEmpty() bool {
	return p.pub.IsEmpty() && p.sub.IsEmpty()
}

// PubList returns the list of publish subjects.
func (p *NatsPermissions) PubList() []Permission {
	return p.pub.AllowList()
}

// SubList returns the list of subscribe subjects (without queue).
func (p *NatsPermissions) SubList() []Permission {
	return p.sub.AllowList()
}

// ToNatsJWT converts policy.NatsPermissions to natsjwt.Permissions.
// When no permissions are granted, we explicitly deny all to prevent
// NATS default behavior of allowing everything when permissions are unset.
// Note: NATS JWTs do not support queue group restrictions.
// Subscriptions allowed with a queue group will be allowed as regular subscriptions.
func (p *NatsPermissions) ToNatsJWT() natsjwt.Permissions {
	var natsPerms natsjwt.Permissions

	pubList := p.PubList()
	if len(pubList) > 0 {
		strList := make([]string, 0, len(pubList))
		for _, perm := range pubList {
			strList = append(strList, perm.String())
		}
		sort.Strings(strList)
		natsPerms.Pub.Allow = strList
	} else {
		// No publish permissions means deny all
		natsPerms.Pub.Deny = []string{">"}
	}

	subList := p.SubList()
	if len(subList) > 0 {
		// Collect unique subjects, ignoring queue groups
		strList := make([]string, 0, len(subList))
		for _, perm := range subList {
			strList = append(strList, perm.String())
		}

		sort.Strings(strList)
		natsPerms.Sub.Allow = strList
	} else {
		// No subscribe permissions means deny all
		natsPerms.Sub.Deny = []string{">"}
	}

	if p.allowResponses {
		// Set empty response permission to allow responses (MaxMsgs and Expires default to 0/nil which means unlimited/default)
		natsPerms.Resp = &natsjwt.ResponsePermission{}
	}

	return natsPerms
}

// deduplicateWithWildcards removes subjects that are covered by wildcard patterns.
// NATS wildcard rules:
//   - `*` matches a single token
//   - `>` matches one or more tokens (must be terminal)
func deduplicateWithWildcards(permissions map[Permission]struct{}) map[Permission]struct{} {
	if len(permissions) == 0 {
		return permissions
	}

	// Convert to slice for processing
	list := make([]Permission, 0, len(permissions))
	for p := range permissions {
		list = append(list, p)
	}

	// For each permission, check if it's covered by any other permission
	// Keep only permissions that are not covered by anything else
	result := make(map[Permission]struct{})
	for _, perm := range list {
		covered := false
		for _, other := range list {
			if perm == other {
				continue
			}
			if isCoveredBy(perm, other) {
				covered = true
				break
			}
		}
		if !covered {
			result[perm] = struct{}{}
		}
	}

	return result
}

// isCoveredBy returns true if subject is covered by pattern.
// This handles both concrete subjects and wildcard patterns, considering queues.
// TODO: this does not handle wildcards in queue names
func isCoveredBy(subject, pattern Permission) bool {
	// If only subject has queue but not pattern, return match result ignoring queue.
	// This covers the case where a general subscription (no queue) covers a queue subscription.
	if subject.Queue != "" && pattern.Queue == "" {
		// Proceed to check subject match
	} else if subject.Queue != "" && pattern.Queue != "" {
		// If both have queues, they must match
		if subject.Queue != pattern.Queue {
			return false
		}
	} else if subject.Queue == "" && pattern.Queue != "" {
		// If only pattern has a queue, it cannot cover a regular subscription
		return false
	}

	// Standard subject check
	if subject.Subject == pattern.Subject {
		return true
	}

	subjectTokens := strings.Split(subject.Subject, ".")
	patternTokens := strings.Split(pattern.Subject, ".")

	// Special case: if subject ends with ">" (multi-token wildcard),
	// it can only be covered by a pattern that also ends with ">"
	// with equal or shorter prefix
	if len(subjectTokens) > 0 && subjectTokens[len(subjectTokens)-1] == ">" {
		if len(patternTokens) == 0 || patternTokens[len(patternTokens)-1] != ">" {
			return false
		}
		// Both end with ">", compare prefixes
		// Pattern must have same or shorter prefix that matches
		subjectPrefix := subjectTokens[:len(subjectTokens)-1]
		patternPrefix := patternTokens[:len(patternTokens)-1]

		if len(patternPrefix) > len(subjectPrefix) {
			return false
		}

		// Pattern prefix must match subject prefix
		for i, pt := range patternPrefix {
			if pt == "*" {
				continue // * in pattern matches any single token
			}
			if pt != subjectPrefix[i] {
				return false
			}
		}
		return true
	}

	// Special case: if subject contains "*" but pattern ends with ">"
	// and pattern prefix matches, subject is covered
	// e.g., "foo.*" is covered by "foo.>"
	if len(patternTokens) > 0 && patternTokens[len(patternTokens)-1] == ">" {
		patternPrefix := patternTokens[:len(patternTokens)-1]
		if len(patternPrefix) < len(subjectTokens) {
			// Check if pattern prefix matches subject prefix
			for i, pt := range patternPrefix {
				if pt == "*" {
					continue
				}
				st := subjectTokens[i]
				if st == "*" {
					continue // * in subject can be anything
				}
				if pt != st {
					return false
				}
			}
			return true
		}
	}

	return matchTokens(subjectTokens, patternTokens)
}

// matchTokens checks if subject tokens match pattern tokens with wildcard support.
func matchTokens(subject, pattern []string) bool {
	si, pi := 0, 0

	for pi < len(pattern) {
		if pattern[pi] == ">" {
			// > matches one or more remaining tokens
			return si < len(subject)
		}

		if si >= len(subject) {
			// Subject exhausted but pattern continues
			return false
		}

		if pattern[pi] == "*" {
			// * matches exactly one token
			si++
			pi++
			continue
		}

		if pattern[pi] != subject[si] {
			// Literal mismatch
			return false
		}

		si++
		pi++
	}

	// Both exhausted = match
	return si == len(subject)
}

func (p *NatsPermissions) String() string {
	pub := p.pub.String()
	sub := p.sub.String()
	return fmt.Sprintf("pub: %s, sub: %s", pub, sub)
}
