package provider_nats_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/msimon/nauts/e2e"
)

func seedKV(t *testing.T, natsURL string) {
	t.Helper()

	nkeyPath, err := filepath.Abs("./user-auth.nk")
	if err != nil {
		t.Fatalf("resolving nkey path: %v", err)
	}

	opt, err := nats.NkeyOptionFromSeed(nkeyPath)
	if err != nil {
		t.Fatalf("loading nkey: %v", err)
	}

	nc, err := nats.Connect(natsURL, opt)
	if err != nil {
		t.Fatalf("connecting to NATS for KV seeding: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("creating jetstream context: %v", err)
	}

	kv, err := js.CreateKeyValue(context.Background(), jetstream.KeyValueConfig{
		Bucket: "nauts-policies",
	})
	if err != nil {
		t.Fatalf("creating KV bucket: %v", err)
	}

	// Seed policy: same as account-static's policies.json
	policyJSON := `{
		"id": "app-consumer",
		"account": "APP",
		"name": "APP consumer",
		"statements": [
			{
				"effect": "allow",
				"actions": ["nats.sub"],
				"resources": ["nats:e2e.mytest"]
			}
		]
	}`
	if _, err := kv.PutString(context.Background(), "APP.policy.app-consumer", policyJSON); err != nil {
		t.Fatalf("seeding policy: %v", err)
	}

	// Seed binding: same as account-static's bindings.json
	bindingJSON := `{"role": "consumer", "account": "APP", "policies": ["app-consumer"]}`
	if _, err := kv.PutString(context.Background(), "APP.binding.consumer", bindingJSON); err != nil {
		t.Fatalf("seeding binding: %v", err)
	}
}

func TestProviderNatsSuite(t *testing.T) {
	hook := func(t *testing.T, natsURL string) {
		seedKV(t, natsURL)
	}

	e2e.WithTestEnv(t, ".", "static", 4225, hook, func(t *testing.T, env *e2e.TestEnv) {
		t.Run("file auth: alice can authenticate", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice failed to authenticate: %v", err)
			}
			nc.Close()
		})

		t.Run("file auth: alice wrong password rejected", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "wrong", "APP", "intro-file")
			if err == nil {
				nc.Close()
				t.Fatalf("alice authenticated with wrong password")
			}
		})

		t.Run("unknown account is rejected", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "NOPE", "intro-file")
			if err == nil {
				nc.Close()
				t.Fatalf("unexpectedly authenticated to unknown account")
			}
		})

		t.Run("policy: alice cannot publish", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice connection failed: %v", err)
			}
			defer nc.Close()

			if err := e2e.PublishSync(nc, "e2e.mytest", []byte("data")); err == nil {
				t.Fatalf("alice published successfully")
			}
		})

		t.Run("policy: alice can subscribe", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice connection failed: %v", err)
			}
			defer nc.Close()

			_, err = e2e.SubscribeSyncWithCheck(nc, "e2e.mytest")
			if err != nil {
				t.Fatalf("alice subscribe failed: %v", err)
			}
		})
	})
}
