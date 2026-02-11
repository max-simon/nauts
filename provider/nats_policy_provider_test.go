package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/msimon/nauts/identity"
	"github.com/msimon/nauts/policy"
)

// --- Unit tests (no NATS required) ---

func TestKvPolicyKey(t *testing.T) {
	tests := []struct {
		account string
		id      string
		want    string
	}{
		{"APP", "read-access", "APP.policy.read-access"},
		{"*", "base-permissions", "_global.policy.base-permissions"},
		{"CORP", "admin", "CORP.policy.admin"},
	}
	for _, tt := range tests {
		got := kvPolicyKey(tt.account, tt.id)
		if got != tt.want {
			t.Errorf("kvPolicyKey(%q, %q) = %q, want %q", tt.account, tt.id, got, tt.want)
		}
	}
}

func TestKvBindingKey(t *testing.T) {
	tests := []struct {
		account string
		role    string
		want    string
	}{
		{"APP", "readonly", "APP.binding.readonly"},
		{"APP", "admin", "APP.binding.admin"},
		{"*", "default", "_global.binding.default"},
	}
	for _, tt := range tests {
		got := kvBindingKey(tt.account, tt.role)
		if got != tt.want {
			t.Errorf("kvBindingKey(%q, %q) = %q, want %q", tt.account, tt.role, got, tt.want)
		}
	}
}

func TestAccountToKVPrefix(t *testing.T) {
	tests := []struct {
		account string
		want    string
	}{
		{"*", "_global"},
		{"APP", "APP"},
		{"_global", "_global"},
	}
	for _, tt := range tests {
		got := accountToKVPrefix(tt.account)
		if got != tt.want {
			t.Errorf("accountToKVPrefix(%q) = %q, want %q", tt.account, got, tt.want)
		}
	}
}

func TestAccountFromKVPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		want   string
	}{
		{"_global", "*"},
		{"APP", "APP"},
		{"CORP", "CORP"},
	}
	for _, tt := range tests {
		got := accountFromKVPrefix(tt.prefix)
		if got != tt.want {
			t.Errorf("accountFromKVPrefix(%q) = %q, want %q", tt.prefix, got, tt.want)
		}
	}
}

func TestParsePolicyKey(t *testing.T) {
	tests := []struct {
		key         string
		wantAccount string
		wantID      string
		wantOK      bool
	}{
		{"APP.policy.read-access", "APP", "read-access", true},
		{"_global.policy.base", "*", "base", true},
		{"APP.binding.admin", "", "", false},
		{"invalid", "", "", false},
		{"APP.policy.", "", "", false},
	}
	for _, tt := range tests {
		account, id, ok := parsePolicyKey(tt.key)
		if ok != tt.wantOK {
			t.Errorf("parsePolicyKey(%q) ok = %v, want %v", tt.key, ok, tt.wantOK)
			continue
		}
		if account != tt.wantAccount {
			t.Errorf("parsePolicyKey(%q) account = %q, want %q", tt.key, account, tt.wantAccount)
		}
		if id != tt.wantID {
			t.Errorf("parsePolicyKey(%q) id = %q, want %q", tt.key, id, tt.wantID)
		}
	}
}

func TestNatsPolicyProviderConfig_GetCacheTTL(t *testing.T) {
	tests := []struct {
		name string
		ttl  string
		want time.Duration
	}{
		{"default when empty", "", 30 * time.Second},
		{"valid duration", "1m", time.Minute},
		{"valid short duration", "5s", 5 * time.Second},
		{"invalid falls back to default", "invalid", 30 * time.Second},
		{"negative falls back to default", "-5s", 30 * time.Second},
		{"zero falls back to default", "0s", 30 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &NatsPolicyProviderConfig{CacheTTL: tt.ttl}
			got := cfg.GetCacheTTL()
			if got != tt.want {
				t.Errorf("GetCacheTTL() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- Integration tests (require nats-server binary) ---

func natsServerAvailable() bool {
	_, err := exec.LookPath("nats-server")
	return err == nil
}

type testNatsServer struct {
	cmd  *exec.Cmd
	port int
	dir  string
}

func startTestNatsServer(t *testing.T) *testNatsServer {
	t.Helper()

	if !natsServerAvailable() {
		t.Skip("nats-server not found in PATH")
	}

	dir := t.TempDir()
	port := 14222 + os.Getpid()%1000

	cmd := exec.Command("nats-server",
		"-js",
		"-sd", dir,
		"-p", fmt.Sprintf("%d", port),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("starting nats-server: %v", err)
	}

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	return &testNatsServer{cmd: cmd, port: port, dir: dir}
}

func (s *testNatsServer) url() string {
	return fmt.Sprintf("nats://localhost:%d", s.port)
}

func createTestBucket(t *testing.T, url, bucket string) jetstream.KeyValue {
	t.Helper()

	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatalf("connecting for bucket creation: %v", err)
	}
	t.Cleanup(func() { nc.Close() })

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("creating jetstream context: %v", err)
	}

	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: bucket,
	})
	if err != nil {
		t.Fatalf("creating bucket %q: %v", bucket, err)
	}
	return kv
}

func seedPolicy(t *testing.T, kv jetstream.KeyValue, account, id string, pol *policy.Policy) {
	t.Helper()
	data, err := json.Marshal(pol)
	if err != nil {
		t.Fatalf("marshaling policy: %v", err)
	}
	key := kvPolicyKey(account, id)
	if _, err := kv.Put(context.Background(), key, data); err != nil {
		t.Fatalf("putting policy %s: %v", key, err)
	}
}

func seedBinding(t *testing.T, kv jetstream.KeyValue, account, role string, b *binding) {
	t.Helper()
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshaling binding: %v", err)
	}
	key := kvBindingKey(account, role)
	if _, err := kv.Put(context.Background(), key, data); err != nil {
		t.Fatalf("putting binding %s: %v", key, err)
	}
}

func TestNatsPolicyProvider_GetPolicy(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-get-policy"
	kv := createTestBucket(t, srv.url(), bucket)

	testPolicy := &policy.Policy{
		ID:      "read-access",
		Account: "APP",
		Name:    "Read Access",
		Statements: []policy.Statement{
			{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:public.>"}},
		},
	}
	seedPolicy(t, kv, "APP", "read-access", testPolicy)

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	ctx := context.Background()

	// Test successful fetch
	pol, err := provider.GetPolicy(ctx, "APP", "read-access")
	if err != nil {
		t.Fatalf("GetPolicy() error = %v", err)
	}
	if pol.ID != "read-access" {
		t.Errorf("GetPolicy() ID = %q, want %q", pol.ID, "read-access")
	}
	if pol.Account != "APP" {
		t.Errorf("GetPolicy() Account = %q, want %q", pol.Account, "APP")
	}

	// Test cache hit (second fetch should use cache)
	pol2, err := provider.GetPolicy(ctx, "APP", "read-access")
	if err != nil {
		t.Fatalf("GetPolicy() cache hit error = %v", err)
	}
	if pol2.ID != "read-access" {
		t.Errorf("GetPolicy() cache hit ID = %q, want %q", pol2.ID, "read-access")
	}

	// Test not found
	_, err = provider.GetPolicy(ctx, "APP", "nonexistent")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("GetPolicy(nonexistent) error = %v, want ErrPolicyNotFound", err)
	}
}

func TestNatsPolicyProvider_GetPolicy_Global(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-get-policy-global"
	kv := createTestBucket(t, srv.url(), bucket)

	globalPolicy := &policy.Policy{
		ID:      "base-permissions",
		Account: "*",
		Name:    "Base Permissions",
		Statements: []policy.Statement{
			{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:public.>"}},
		},
	}
	seedPolicy(t, kv, "*", "base-permissions", globalPolicy)

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	pol, err := provider.GetPolicy(context.Background(), "*", "base-permissions")
	if err != nil {
		t.Fatalf("GetPolicy() error = %v", err)
	}
	if pol.ID != "base-permissions" {
		t.Errorf("GetPolicy() ID = %q, want %q", pol.ID, "base-permissions")
	}
}

func TestNatsPolicyProvider_GetPoliciesForRole(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-get-policies-for-role"
	kv := createTestBucket(t, srv.url(), bucket)

	// Seed policies
	seedPolicy(t, kv, "APP", "read-access", &policy.Policy{
		ID:      "read-access",
		Account: "APP",
		Name:    "Read",
		Statements: []policy.Statement{
			{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:public.>"}},
		},
	})
	seedPolicy(t, kv, "APP", "write-access", &policy.Policy{
		ID:      "write-access",
		Account: "APP",
		Name:    "Write",
		Statements: []policy.Statement{
			{Effect: "allow", Actions: []policy.Action{"nats.pub"}, Resources: []string{"nats:data.>"}},
		},
	})

	// Seed binding
	seedBinding(t, kv, "APP", "admin", &binding{
		Role:     "admin",
		Account:  "APP",
		Policies: []string{"read-access", "write-access"},
	})

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	policies, err := provider.GetPoliciesForRole(context.Background(), identity.Role{
		Account: "APP",
		Name:    "admin",
	})
	if err != nil {
		t.Fatalf("GetPoliciesForRole() error = %v", err)
	}
	if len(policies) != 2 {
		t.Fatalf("GetPoliciesForRole() returned %d policies, want 2", len(policies))
	}
	// Should be sorted by ID
	if policies[0].ID != "read-access" {
		t.Errorf("policies[0].ID = %q, want %q", policies[0].ID, "read-access")
	}
	if policies[1].ID != "write-access" {
		t.Errorf("policies[1].ID = %q, want %q", policies[1].ID, "write-access")
	}
}

func TestNatsPolicyProvider_GetPoliciesForRole_NotFound(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-role-not-found"
	createTestBucket(t, srv.url(), bucket)

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	_, err = provider.GetPoliciesForRole(context.Background(), identity.Role{
		Account: "APP",
		Name:    "nonexistent",
	})
	if !errors.Is(err, ErrRoleNotFound) {
		t.Errorf("GetPoliciesForRole(nonexistent) error = %v, want ErrRoleNotFound", err)
	}
}

func TestNatsPolicyProvider_GetPoliciesForRole_MissingPolicy(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-missing-policy"
	kv := createTestBucket(t, srv.url(), bucket)

	// Seed binding referencing a policy that doesn't exist
	seedBinding(t, kv, "APP", "broken", &binding{
		Role:     "broken",
		Account:  "APP",
		Policies: []string{"nonexistent-policy"},
	})

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	// Should return empty list, not an error (consistent with FilePolicyProvider)
	policies, err := provider.GetPoliciesForRole(context.Background(), identity.Role{
		Account: "APP",
		Name:    "broken",
	})
	if err != nil {
		t.Fatalf("GetPoliciesForRole() error = %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("GetPoliciesForRole() returned %d policies, want 0", len(policies))
	}
}

func TestNatsPolicyProvider_GetPolicies(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-get-policies"
	kv := createTestBucket(t, srv.url(), bucket)

	// Seed APP policies
	seedPolicy(t, kv, "APP", "read", &policy.Policy{
		ID: "read", Account: "APP", Name: "Read",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:public.>"}}},
	})
	seedPolicy(t, kv, "APP", "write", &policy.Policy{
		ID: "write", Account: "APP", Name: "Write",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.pub"}, Resources: []string{"nats:data.>"}}},
	})
	// Seed global policy
	seedPolicy(t, kv, "*", "global-base", &policy.Policy{
		ID: "global-base", Account: "*", Name: "Global Base",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:status.>"}}},
	})
	// Seed OTHER account policy (should not appear)
	seedPolicy(t, kv, "OTHER", "other-policy", &policy.Policy{
		ID: "other-policy", Account: "OTHER", Name: "Other",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:other.>"}}},
	})

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	policies, err := provider.GetPolicies(context.Background(), "APP")
	if err != nil {
		t.Fatalf("GetPolicies() error = %v", err)
	}
	if len(policies) != 3 {
		t.Fatalf("GetPolicies() returned %d policies, want 3", len(policies))
	}
	// Should be sorted by ID
	if policies[0].ID != "global-base" {
		t.Errorf("policies[0].ID = %q, want %q", policies[0].ID, "global-base")
	}
	if policies[1].ID != "read" {
		t.Errorf("policies[1].ID = %q, want %q", policies[1].ID, "read")
	}
	if policies[2].ID != "write" {
		t.Errorf("policies[2].ID = %q, want %q", policies[2].ID, "write")
	}
}

func TestNatsPolicyProvider_GetPolicies_EmptyBucket(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-empty-bucket"
	createTestBucket(t, srv.url(), bucket)

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	policies, err := provider.GetPolicies(context.Background(), "APP")
	if err != nil {
		t.Fatalf("GetPolicies() error = %v", err)
	}
	if len(policies) != 0 {
		t.Errorf("GetPolicies() returned %d policies, want 0", len(policies))
	}
}

func TestNatsPolicyProvider_CacheInvalidation(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-cache-invalidation"
	kv := createTestBucket(t, srv.url(), bucket)

	seedPolicy(t, kv, "APP", "mutable", &policy.Policy{
		ID: "mutable", Account: "APP", Name: "Original",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.sub"}, Resources: []string{"nats:old.>"}}},
	})

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:   bucket,
		NatsURL:  srv.url(),
		CacheTTL: "1m",
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}
	defer provider.Stop()

	ctx := context.Background()

	// First fetch — populates cache
	pol, err := provider.GetPolicy(ctx, "APP", "mutable")
	if err != nil {
		t.Fatalf("GetPolicy() error = %v", err)
	}
	if pol.Name != "Original" {
		t.Fatalf("GetPolicy() Name = %q, want %q", pol.Name, "Original")
	}

	// Update the policy in KV
	updated := &policy.Policy{
		ID: "mutable", Account: "APP", Name: "Updated",
		Statements: []policy.Statement{{Effect: "allow", Actions: []policy.Action{"nats.pub"}, Resources: []string{"nats:new.>"}}},
	}
	data, _ := json.Marshal(updated)
	if _, err := kv.Put(ctx, kvPolicyKey("APP", "mutable"), data); err != nil {
		t.Fatalf("updating policy: %v", err)
	}

	// Wait for watcher to invalidate cache
	time.Sleep(500 * time.Millisecond)

	// Should get updated value
	pol, err = provider.GetPolicy(ctx, "APP", "mutable")
	if err != nil {
		t.Fatalf("GetPolicy() after update error = %v", err)
	}
	if pol.Name != "Updated" {
		t.Errorf("GetPolicy() after update Name = %q, want %q", pol.Name, "Updated")
	}
}

func TestNatsPolicyProvider_MissingBucket(t *testing.T) {
	srv := startTestNatsServer(t)

	_, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  "nonexistent-bucket",
		NatsURL: srv.url(),
	})
	if err == nil {
		t.Fatal("expected error for missing bucket")
	}
}

func TestNatsPolicyProvider_Stop(t *testing.T) {
	srv := startTestNatsServer(t)
	bucket := "test-stop"
	createTestBucket(t, srv.url(), bucket)

	provider, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
		Bucket:  bucket,
		NatsURL: srv.url(),
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	if err := provider.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestNatsPolicyProvider_ValidationErrors(t *testing.T) {
	t.Run("missing bucket", func(t *testing.T) {
		_, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
			NatsURL: "nats://localhost:4222",
		})
		if err == nil {
			t.Fatal("expected error for missing bucket")
		}
	})

	t.Run("mutually exclusive credentials", func(t *testing.T) {
		_, err := NewNatsPolicyProvider(NatsPolicyProviderConfig{
			Bucket:          "test",
			NatsURL:         "nats://localhost:4222",
			NatsCredentials: "/path/to/creds",
			NatsNkey:        "/path/to/nkey",
		})
		if err == nil {
			t.Fatal("expected error for mutually exclusive credentials")
		}
	})
}
