package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/policy"
)

const (
	// globalAccountPrefix is the KV key prefix used for global policies (account="*").
	globalAccountPrefix = "_global"

	// defaultCacheTTL is the default cache time-to-live.
	defaultCacheTTL = 30 * time.Second
)

// NatsPolicyProviderConfig holds configuration for NatsPolicyProvider.
type NatsPolicyProviderConfig struct {
	// Bucket is the name of the NATS KV bucket.
	Bucket string `json:"bucket"`

	// NatsURL is the NATS server URL (e.g., "nats://localhost:4222").
	NatsURL string `json:"natsUrl"`

	// NatsCredentials is the path to NATS credentials file.
	// Mutually exclusive with NatsNkey.
	NatsCredentials string `json:"natsCredentials,omitempty"`

	// NatsNkey is the path to the nkey seed file for NATS authentication.
	// Mutually exclusive with NatsCredentials.
	NatsNkey string `json:"natsNkey,omitempty"`

	// CacheTTL is how long cached entries remain valid, as a duration string (e.g., "30s", "1m").
	// Default: "30s".
	CacheTTL string `json:"cacheTtl,omitempty"`
}

// GetCacheTTL returns the cache TTL as a time.Duration, defaulting to 30s.
func (c *NatsPolicyProviderConfig) GetCacheTTL() time.Duration {
	if c.CacheTTL == "" {
		return defaultCacheTTL
	}
	d, err := time.ParseDuration(c.CacheTTL)
	if err != nil || d <= 0 {
		return defaultCacheTTL
	}
	return d
}

// NatsPolicyProvider implements PolicyProvider using a NATS KV bucket.
type NatsPolicyProvider struct {
	nc      *nats.Conn
	kv      jetstream.KeyValue
	cache   *cache
	config  NatsPolicyProviderConfig
	watcher jetstream.KeyWatcher
	done    chan struct{}
}

// NewNatsPolicyProvider creates a new NatsPolicyProvider from the given configuration.
// The KV bucket must already exist.
func NewNatsPolicyProvider(cfg NatsPolicyProviderConfig) (*NatsPolicyProvider, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("nats policy provider: bucket is required")
	}
	if cfg.NatsURL == "" {
		cfg.NatsURL = nats.DefaultURL
	}
	if url := os.Getenv("NATS_URL"); url != "" {
		cfg.NatsURL = url
	}
	if cfg.NatsCredentials != "" && cfg.NatsNkey != "" {
		return nil, fmt.Errorf("nats policy provider: natsCredentials and natsNkey are mutually exclusive")
	}

	// Build NATS options
	opts := []nats.Option{
		nats.Name("nauts-policy-provider"),
	}
	if cfg.NatsCredentials != "" {
		opts = append(opts, nats.UserCredentials(cfg.NatsCredentials))
	} else if cfg.NatsNkey != "" {
		opt, err := nats.NkeyOptionFromSeed(cfg.NatsNkey)
		if err != nil {
			return nil, fmt.Errorf("nats policy provider: loading nkey from %s: %w", cfg.NatsNkey, err)
		}
		opts = append(opts, opt)
	}

	// Connect to NATS
	nc, err := nats.Connect(cfg.NatsURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("nats policy provider: connecting to NATS: %w", err)
	}

	// Obtain JetStream context and open KV bucket
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats policy provider: creating jetstream context: %w", err)
	}

	kv, err := js.KeyValue(context.Background(), cfg.Bucket)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats policy provider: opening bucket %q: %w", cfg.Bucket, err)
	}

	p := &NatsPolicyProvider{
		nc:     nc,
		kv:     kv,
		cache:  newCache(cfg.GetCacheTTL()),
		config: cfg,
		done:   make(chan struct{}),
	}

	// Start watcher
	if err := p.startWatcher(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("nats policy provider: starting watcher: %w", err)
	}

	return p, nil
}

// Stop stops the KV watcher, closes the NATS connection, and clears the cache.
func (p *NatsPolicyProvider) Stop() error {
	close(p.done)
	if p.watcher != nil {
		_ = p.watcher.Stop()
	}
	p.nc.Close()
	p.cache.clear()
	return nil
}

// GetPolicy retrieves a policy by account and ID from the KV bucket.
func (p *NatsPolicyProvider) GetPolicy(ctx context.Context, account string, id string) (*policy.Policy, error) {
	key := kvPolicyKey(account, id)

	// Check cache
	if cached := p.cache.get(key); cached != nil {
		return cached.(*policy.Policy), nil
	}

	// Fetch from KV
	entry, err := p.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("fetching policy %s: %w", key, err)
	}

	var pol policy.Policy
	if err := json.Unmarshal(entry.Value(), &pol); err != nil {
		return nil, fmt.Errorf("decoding policy %s: %w", key, err)
	}
	if err := pol.Validate(); err != nil {
		return nil, fmt.Errorf("validating policy %s: %w", key, err)
	}

	p.cache.put(key, &pol)
	return &pol, nil
}

// GetPoliciesForRole returns all policies attached to a role.
// If the id starts with "_global:", the prefix is stripped and the policy
// is looked up as a global policy (account="_global").
func (p *NatsPolicyProvider) GetPoliciesForRole(ctx context.Context, role identity.Role) ([]*policy.Policy, error) {
	role.Name = strings.TrimSpace(role.Name)
	if role.Name == "" {
		return nil, ErrRoleNotFound
	}
	role.Account = strings.TrimSpace(role.Account)
	if role.Account == "" {
		return nil, ErrRoleNotFound
	}

	b, err := p.getBinding(ctx, role.Account, role.Name)
	if err != nil {
		return nil, err
	}

	// Deduplicate and sort policy IDs
	policyIDs := make([]string, 0, len(b.Policies))
	seen := make(map[string]struct{})
	for _, id := range b.Policies {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		policyIDs = append(policyIDs, id)
	}
	sort.Strings(policyIDs)

	result := make([]*policy.Policy, 0, len(policyIDs))
	for _, id := range policyIDs {
		policyAccount := role.Account
		// if policy id has prefix `_global:`, pull from global account
		if strings.HasPrefix(id, globalAccountPrefix+":") {
			id = strings.TrimPrefix(id, globalAccountPrefix+":")
			policyAccount = globalAccountPrefix
		}
		pol, err := p.GetPolicy(ctx, policyAccount, id)
		if err != nil {
			if errors.Is(err, ErrPolicyNotFound) {
				continue
			}
			return nil, err
		}
		result = append(result, pol)
	}

	return result, nil
}

// GetPolicies returns all policies for the given account plus global policies.
func (p *NatsPolicyProvider) GetPolicies(ctx context.Context, account string) ([]*policy.Policy, error) {
	account = strings.TrimSpace(account)

	// Build filters to find matching keys
	filters := []string{account + ".policy.>"}
	if account != globalAccountPrefix {
		filters = append(filters, globalAccountPrefix+".policy.>")
	}

	lister, err := p.kv.ListKeysFiltered(ctx, filters...)
	if err != nil {
		if errors.Is(err, jetstream.ErrNoKeysFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing policy keys: %w", err)
	}

	var result []*policy.Policy
	for key := range lister.Keys() {
		acc, id, ok := parsePolicyKey(key)
		if !ok {
			continue
		}
		pol, err := p.GetPolicy(ctx, acc, id)
		if err != nil {
			if errors.Is(err, ErrPolicyNotFound) {
				continue
			}
			return nil, err
		}
		result = append(result, pol)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result, nil
}

// getBinding fetches a binding from the cache or KV bucket.
func (p *NatsPolicyProvider) getBinding(ctx context.Context, account, role string) (*binding, error) {
	key := kvBindingKey(account, role)

	// Check cache
	if cached := p.cache.get(key); cached != nil {
		return cached.(*binding), nil
	}

	// Fetch from KV
	entry, err := p.kv.Get(ctx, key)
	if err != nil {
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			return nil, ErrRoleNotFound
		}
		return nil, fmt.Errorf("fetching binding %s: %w", key, err)
	}

	var b binding
	if err := json.Unmarshal(entry.Value(), &b); err != nil {
		return nil, fmt.Errorf("decoding binding %s: %w", key, err)
	}

	p.cache.put(key, &b)
	return &b, nil
}

// startWatcher creates a KV watcher on the entire bucket for cache invalidation.
func (p *NatsPolicyProvider) startWatcher() error {
	watcher, err := p.kv.WatchAll(context.Background(), jetstream.UpdatesOnly())
	if err != nil {
		return fmt.Errorf("creating watcher: %w", err)
	}
	p.watcher = watcher

	go p.watchLoop()
	return nil
}

// watchLoop processes watcher updates and invalidates cache entries.
func (p *NatsPolicyProvider) watchLoop() {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		updates := p.watcher.Updates()
		for {
			select {
			case <-p.done:
				return
			case entry, ok := <-updates:
				if !ok {
					// Channel closed — try to reconnect
					goto reconnect
				}
				if entry != nil {
					p.cache.invalidate(entry.Key())
				}
			}
		}

	reconnect:
		// Attempt to re-establish the watcher with exponential backoff
		for {
			select {
			case <-p.done:
				return
			case <-time.After(backoff):
			}

			watcher, err := p.kv.WatchAll(context.Background(), jetstream.UpdatesOnly())
			if err != nil {
				log.Printf("nats policy provider: watcher reconnect failed: %v", err)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			p.watcher = watcher
			backoff = time.Second
			break
		}
	}
}

// kvPolicyKey builds the KV key for a policy.
// Global policies (account="*") use "_global" as the account prefix.
func kvPolicyKey(account string, id string) string {
	return account + ".policy." + id
}

// kvBindingKey builds the KV key for a binding.
func kvBindingKey(account string, role string) string {
	return account + ".binding." + role
}

// parsePolicyKey extracts account and policy ID from a KV key.
// Returns ("", "", false) if the key does not match the expected pattern.
func parsePolicyKey(key string) (account, id string, ok bool) {
	// Expected format: <account>.policy.<id>
	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 || parts[1] != "policy" || parts[2] == "" {
		return "", "", false
	}
	return parts[0], parts[2], true
}
