package policy_kv_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/msimon/nauts/e2e"
	"github.com/nats-io/nats.go"
)

func TestPoliciesKV(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4230, nil, func(t *testing.T, env *e2e.TestEnv) {
		adminNc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
		if err != nil {
			t.Fatalf("setup failed to authenticate as admin: %v", err)
		}
		defer adminNc.Close()

		adminJs, err := adminNc.JetStream()
		if err != nil {
			t.Fatalf("setup failed to get JetStream context: %v", err)
		}

		if _, err := adminJs.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_VIEW"}); err != nil {
			t.Fatalf("setup failed to create BUCKET_VIEW: %v", err)
		}
		kvData, err := adminJs.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_DATA"})
		if err != nil {
			t.Fatalf("setup failed to create BUCKET_DATA: %v", err)
		}
		for i := 0; i < 3; i++ {
			_, _ = kvData.PutString(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
		}
		kvEdit, err := adminJs.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_EDIT"})
		if err != nil {
			t.Fatalf("setup failed to create BUCKET_EDIT: %v", err)
		}
		_, _ = kvEdit.PutString("seed", "seed")

		kvScoped, err := adminJs.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_SCOPED"})
		if err != nil {
			t.Fatalf("setup failed to create BUCKET_SCOPED: %v", err)
		}
		_, _ = kvScoped.PutString("scoper.ok", "yes")
		_, _ = kvScoped.PutString("nope.ok", "no")

		t.Run("kv.manage: manager can create and delete BUCKET_MGR (but cannot delete BUCKET_VIEW)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("manager", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("manager failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			if _, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "BUCKET_MGR"}); err != nil {
				t.Fatalf("manager failed to create BUCKET_MGR: %v", err)
			}
			if err := js.DeleteKeyValue("BUCKET_MGR"); err != nil {
				t.Fatalf("manager failed to delete BUCKET_MGR: %v", err)
			}

			if err := js.DeleteKeyValue("BUCKET_VIEW"); err == nil {
				t.Fatalf("manager succeeded to delete BUCKET_VIEW (expected error)")
			}
		})

		t.Run("kv.view: viewer can inspect BUCKET_VIEW (but cannot read or write keys)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("viewer", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("viewer failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_VIEW")
			if err != nil {
				t.Fatalf("viewer failed to bind to BUCKET_VIEW: %v", err)
			}
			if _, err := kv.Status(); err != nil {
				t.Fatalf("viewer failed to get BUCKET_VIEW status: %v", err)
			}

			if _, err := kv.Get("missing"); err == nil {
				t.Fatalf("viewer succeeded to read key from BUCKET_VIEW (expected error)")
			}
			if _, err := kv.PutString("k", "v"); err == nil {
				t.Fatalf("viewer succeeded to write key to BUCKET_VIEW (expected error)")
			}
		})

		t.Run("kv.read with key '>' wildcard: reader can read any key in BUCKET_DATA but cannot write", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("reader", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("reader failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_DATA")
			if err != nil {
				t.Fatalf("reader failed to bind to BUCKET_DATA: %v", err)
			}
			for i := 0; i < 3; i++ {
				entry, err := kv.Get(fmt.Sprintf("k%d", i))
				if err != nil {
					t.Fatalf("reader failed to get k%d: %v", i, err)
				}
				if got, want := string(entry.Value()), fmt.Sprintf("v%d", i); got != want {
					t.Fatalf("reader got %q, want %q", got, want)
				}
			}
			if _, err := kv.PutString("new", "value"); err == nil {
				t.Fatalf("reader succeeded to write to BUCKET_DATA (expected error)")
			}

			watcher, err := kv.WatchAll(nats.IgnoreDeletes())
			if err != nil {
				t.Fatalf("reader failed to start watch: %v", err)
			}
			defer watcher.Stop()
			select {
			case <-watcher.Updates():
				// ok (initial values or subsequent updates)
			case <-time.After(250 * time.Millisecond):
				// also ok: the key point is that the watch subscription was allowed
			}
		})

		t.Run("kv.edit with key '>' wildcard: editor can put/get/delete in BUCKET_EDIT", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("editor", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("editor failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_EDIT")
			if err != nil {
				t.Fatalf("editor failed to bind to BUCKET_EDIT: %v", err)
			}

			if _, err := kv.PutString("key", "value"); err != nil {
				t.Fatalf("editor failed to put key: %v", err)
			}
			entry, err := kv.Get("key")
			if err != nil {
				t.Fatalf("editor failed to get key: %v", err)
			}
			if got, want := string(entry.Value()), "value"; got != want {
				t.Fatalf("editor got %q, want %q", got, want)
			}
			if err := kv.Delete("key"); err != nil {
				t.Fatalf("editor failed to delete key: %v", err)
			}
		})

		t.Run("kv.read with key '{{ user.id }}.>' scope: scoper can read own prefix but not nope", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("scoper", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("scoper failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			kv, err := js.KeyValue("BUCKET_SCOPED")
			if err != nil {
				t.Fatalf("scoper failed to bind to BUCKET_SCOPED: %v", err)
			}

			entry, err := kv.Get("scoper.ok")
			if err != nil {
				t.Fatalf("scoper failed to get scoper.ok: %v", err)
			}
			if got, want := string(entry.Value()), "yes"; got != want {
				t.Fatalf("scoper got %q, want %q", got, want)
			}

			if _, err := kv.Get("nope.ok"); err == nil {
				t.Fatalf("scoper succeeded to get nope.ok (expected error)")
			}
			if _, err := kv.PutString("scoper.write", "nope"); err == nil {
				t.Fatalf("scoper succeeded to write (expected error)")
			}
		})
	})
}
