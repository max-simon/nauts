// Package policy provides policy-related types and functions for nauts.
// This file contains NATS permission types.
package policy

import (
	"sort"
	"strings"
)

// PermissionType represents the type of NATS permission.
type PermissionType string

const (
	PermPub PermissionType = "pub" // Publish permission
	PermSub PermissionType = "sub" // Subscribe permission
)

// Permission represents a single NATS permission.
type Permission struct {
	Type    PermissionType `json:"type"`
	Subject string         `json:"subject"`
	Queue   string         `json:"queue,omitempty"` // Only for SUB permissions
}

// PermissionSet holds a set of allowed subjects with wildcard-aware deduplication.
type PermissionSet struct {
	allow map[string]struct{}
	deny  map[string]struct{} // For future deny support
}

// NewPermissionSet creates a new empty PermissionSet.
func NewPermissionSet() *PermissionSet {
	return &PermissionSet{
		allow: make(map[string]struct{}),
		deny:  make(map[string]struct{}),
	}
}

// Add adds a subject to the allow set.
func (ps *PermissionSet) Add(subject string) {
	if ps.allow == nil {
		ps.allow = make(map[string]struct{})
	}
	ps.allow[subject] = struct{}{}
}

// AddDeny adds a subject to the deny set (placeholder for future use).
func (ps *PermissionSet) AddDeny(subject string) {
	if ps.deny == nil {
		ps.deny = make(map[string]struct{})
	}
	ps.deny[subject] = struct{}{}
}

// AllowList returns the allow set as a sorted slice.
func (ps *PermissionSet) AllowList() []string {
	if ps.allow == nil {
		return []string{}
	}
	result := make([]string, 0, len(ps.allow))
	for s := range ps.allow {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

// DenyList returns the deny set as a sorted slice.
func (ps *PermissionSet) DenyList() []string {
	if ps.deny == nil {
		return []string{}
	}
	result := make([]string, 0, len(ps.deny))
	for s := range ps.deny {
		result = append(result, s)
	}
	sort.Strings(result)
	return result
}

// Deduplicate removes subjects that are covered by wildcards.
func (ps *PermissionSet) Deduplicate() {
	ps.allow = deduplicateWithWildcards(ps.allow)
	ps.deny = deduplicateWithWildcards(ps.deny)
}

// IsEmpty returns true if the permission set has no allowed subjects.
func (ps *PermissionSet) IsEmpty() bool {
	return ps.allow == nil || len(ps.allow) == 0
}

// SubQueuePerm represents a subscription permission with a queue group.
type SubQueuePerm struct {
	Subject string `json:"subject"`
	Queue   string `json:"queue"`
}

// NatsPermissions holds compiled NATS permissions.
type NatsPermissions struct {
	pub          *PermissionSet
	sub          *PermissionSet
	subWithQueue map[string]map[string]struct{} // subject -> set of queues
}

// NewNatsPermissions creates an empty NatsPermissions struct.
func NewNatsPermissions() *NatsPermissions {
	return &NatsPermissions{
		pub:          NewPermissionSet(),
		sub:          NewPermissionSet(),
		subWithQueue: make(map[string]map[string]struct{}),
	}
}

// Allow adds a permission to the appropriate allow set.
func (p *NatsPermissions) Allow(perm Permission) {
	switch perm.Type {
	case PermPub:
		p.pub.Add(perm.Subject)
	case PermSub:
		if perm.Queue != "" {
			p.addSubWithQueue(perm.Subject, perm.Queue)
		} else {
			p.sub.Add(perm.Subject)
		}
	}
}

// Deny adds a permission to the deny set (placeholder for future use).
func (p *NatsPermissions) Deny(perm Permission) {
	switch perm.Type {
	case PermPub:
		p.pub.AddDeny(perm.Subject)
	case PermSub:
		if perm.Queue == "" {
			p.sub.AddDeny(perm.Subject)
		}
	}
}

// addSubWithQueue adds a subscribe permission with queue to the internal set.
func (p *NatsPermissions) addSubWithQueue(subject, queue string) {
	if p.subWithQueue == nil {
		p.subWithQueue = make(map[string]map[string]struct{})
	}
	if p.subWithQueue[subject] == nil {
		p.subWithQueue[subject] = make(map[string]struct{})
	}
	p.subWithQueue[subject][queue] = struct{}{}
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
	for subject, queues := range other.subWithQueue {
		for queue := range queues {
			p.addSubWithQueue(subject, queue)
		}
	}
}

// Deduplicate removes duplicate permissions using wildcard-aware deduplication.
func (p *NatsPermissions) Deduplicate() {
	p.pub.Deduplicate()
	p.sub.Deduplicate()
}

// IsEmpty returns true if there are no permissions.
func (p *NatsPermissions) IsEmpty() bool {
	return p.pub.IsEmpty() && p.sub.IsEmpty() && len(p.subWithQueue) == 0
}

// PubList returns the list of publish subjects.
func (p *NatsPermissions) PubList() []string {
	return p.pub.AllowList()
}

// SubList returns the list of subscribe subjects (without queue).
func (p *NatsPermissions) SubList() []string {
	return p.sub.AllowList()
}

// SubWithQueueList returns the list of subscribe permissions with queue groups.
func (p *NatsPermissions) SubWithQueueList() []SubQueuePerm {
	result := make([]SubQueuePerm, 0)
	for subject, queues := range p.subWithQueue {
		for queue := range queues {
			result = append(result, SubQueuePerm{Subject: subject, Queue: queue})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Subject != result[j].Subject {
			return result[i].Subject < result[j].Subject
		}
		return result[i].Queue < result[j].Queue
	})
	return result
}

// GetPermissions returns permissions in NATS-compatible format.
func (p *NatsPermissions) GetPermissions() map[string]interface{} {
	result := make(map[string]interface{})

	pubList := p.PubList()
	if len(pubList) > 0 {
		result["publish"] = map[string]interface{}{
			"allow": pubList,
		}
	}

	subList := p.SubList()
	subQueueList := p.SubWithQueueList()
	if len(subList) > 0 || len(subQueueList) > 0 {
		subPerms := make(map[string]interface{})
		if len(subList) > 0 {
			subPerms["allow"] = subList
		}
		result["subscribe"] = subPerms
	}

	return result
}

// deduplicateWithWildcards removes subjects that are covered by wildcard patterns.
// NATS wildcard rules:
//   - `*` matches a single token
//   - `>` matches one or more tokens (must be terminal)
func deduplicateWithWildcards(subjects map[string]struct{}) map[string]struct{} {
	if len(subjects) == 0 {
		return subjects
	}

	// Convert to slice for processing
	list := make([]string, 0, len(subjects))
	for s := range subjects {
		list = append(list, s)
	}

	// For each subject, check if it's covered by any other subject
	// Keep only subjects that are not covered by anything else
	result := make(map[string]struct{})
	for _, subject := range list {
		covered := false
		for _, other := range list {
			if subject == other {
				continue
			}
			if isCoveredBy(subject, other) {
				covered = true
				break
			}
		}
		if !covered {
			result[subject] = struct{}{}
		}
	}

	return result
}

// isCoveredBy returns true if subject is covered by pattern.
// This handles both concrete subjects and wildcard patterns.
// Examples:
//   - "foo.bar" is covered by "foo.*"
//   - "foo.bar" is covered by "foo.>"
//   - "foo.bar.baz" is covered by "foo.>"
//   - "foo.bar" is NOT covered by "foo.*.baz"
//   - "foo.*" is covered by "foo.>"
//   - "foo.>" is NOT covered by "foo.*"
func isCoveredBy(subject, pattern string) bool {
	if subject == pattern {
		return true
	}

	subjectTokens := strings.Split(subject, ".")
	patternTokens := strings.Split(pattern, ".")

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
