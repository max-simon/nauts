package e2e

import (
	"flag"
	"testing"

	"github.com/nats-io/nats.go"
)

var (
	skipPolicyKv = flag.Bool("skipPolicyKv", false, "Skip policy kv tests")
)

func TestPoliciesKV(t *testing.T) {

	if *skipPolicyKv {
		t.Log("Skipping policy kv tests")
		return
	}

	WithTestEnv(t, "policy-static", "static", 4230, func(t *testing.T, env *TestEnv) {

		// Setup Buckets as admin
		func() {
			nc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("Setup failed to authenticate as admin: %v", err)
			}
			defer nc.Close()
			js, err := nc.JetStream()
			if err != nil {
				t.Fatalf("Setup failed to get JetStream context: %v", err)
			}

			// Create BUCKET_WRITE bucket
			if _, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_WRITE"}); err != nil {
				t.Fatalf("Setup failed to create BUCKET_WRITE bucket: %v", err)
			}
			// Create BUCKET_READ bucket
			kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_READ"})
			if err != nil {
				t.Fatalf("Setup failed to create BUCKET_READ bucket: %v", err)
			}
			kv.PutString("foo", "bar")
		}()

		t.Run("writer can edit BUCKET_WRITE", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("writer", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("writer failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_WRITE")
			if err != nil {
				t.Fatalf("writer failed to bind to BUCKET_WRITE: %v", err)
			}

			// Write (should succeed)
			_, err = kv.PutString("key", "value")
			if err != nil {
				t.Errorf("writer failed to write to BUCKET_WRITE: %v", err)
			}

			// Read (should succeed)
			entry, err := kv.Get("key")
			if err != nil {
				t.Errorf("writer failed to get key from BUCKET_WRITE: %v", err)
			} else if string(entry.Value()) != "value" {
				t.Errorf("writer got wrong value: %s", string(entry.Value()))
			}

			// Delete (should succeed)
			if err := kv.Delete("key"); err != nil {
				t.Errorf("writer failed to delete key from BUCKET_WRITE: %v", err)
			}

			// Negative: Write to BUCKET_READ
			// kv.view/read allows getting value but not putting
			// But writer doesn't have permissions on BUCKET_READ
			_, err = js.KeyValue("BUCKET_READ")
			if err != nil {
				// Binding might fail if no stream info permission?
				// Actually writer has no permissions on BUCKET_READ, so binding might fail or succeed depending on implementation details of client library
				// NATS Go client usually does StreamInfo/Consumer info
			} else {
				// If binding succeeds (it might), try operation
				// But we expect binding to fail or operation to fail.
			}
		})

		t.Run("reader can read BUCKET_READ but not write", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("reader", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("reader failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_READ")
			if err != nil {
				t.Fatalf("reader failed to bind to BUCKET_READ: %v", err)
			}

			// Read (should succeed)
			entry, err := kv.Get("foo")
			if err != nil {
				t.Errorf("reader failed to get foo from BUCKET_READ: %v", err)
			} else if string(entry.Value()) != "bar" {
				t.Errorf("reader got wrong value: %s", string(entry.Value()))
			}

			// Write (should fail)
			_, err = kv.PutString("newkey", "value")
			if err == nil {
				t.Errorf("reader succeeded to write to BUCKET_READ (expected error)")
			}
		})

		t.Run("alice can edit USER_alice", func(t *testing.T) {
			// Create user bucket first as admin
			adminNc, _ := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			defer adminNc.Close()
			adminJs, _ := adminNc.JetStream()
			adminJs.CreateKeyValue(&nats.KeyValueConfig{Bucket: "USER_alice"})

			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("alice failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("USER_alice")
			if err != nil {
				t.Fatalf("alice failed to bind to USER_alice: %v", err)
			}

			// Write
			_, err = kv.PutString("profile", "json")
			if err != nil {
				t.Errorf("alice failed to write to USER_alice: %v", err)
			}
		})
	})
}
