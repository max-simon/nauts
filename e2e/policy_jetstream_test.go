package e2e

import (
	"flag"
	"testing"

	"github.com/nats-io/nats.go"
)

var (
	skipPolicyJetstream = flag.Bool("skipPolicyJetstream", false, "Skip policy jetstream tests")
)

func TestPoliciesJetstream(t *testing.T) {

	if *skipPolicyJetstream {
		t.Log("Skipping policy jetstream tests")
		return
	}

	WithTestEnv(t, "policy-static", "static", 4231, func(t *testing.T, env *TestEnv) {

		// Setup streams as admin
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
			if _, err = js.AddStream(&nats.StreamConfig{Name: "STREAM_WRITE", Subjects: []string{"write.>"}}); err != nil {
				t.Fatalf("Setup failed to create STREAM_WRITE stream: %v", err)
			}
			if _, err = js.AddStream(&nats.StreamConfig{Name: "STREAM_READ", Subjects: []string{"read.>"}}); err != nil {
				t.Fatalf("Setup failed to create STREAM_READ stream: %v", err)
			}
			if _, err = js.AddConsumer("STREAM_READ", &nats.ConsumerConfig{
				Durable: "CONSUMER_READ",
			}); err != nil {
				t.Fatalf("Setup failed to create CONSUMER_READ: %v", err)
			}
		}()

		t.Run("writer can manage STREAM_WRITE", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("writer", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("writer failed to authenticate: %v", err)
			}
			defer nc.Close()
			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			// Update stream
			_, err = js.UpdateStream(&nats.StreamConfig{Name: "STREAM_WRITE", Subjects: []string{"write.>", "new.>"}})
			if err != nil {
				t.Errorf("writer failed to update STREAM_WRITE: %v", err)
			}

			// Purge stream
			if err := js.PurgeStream("STREAM_WRITE"); err != nil {
				t.Errorf("writer failed to purge STREAM_WRITE: %v", err)
			}

			// Negative: manage other stream
			if _, err := js.UpdateStream(&nats.StreamConfig{Name: "STREAM_READ", Subjects: []string{"read.>"}}); err == nil {
				t.Errorf("writer succeeded to update STREAM_READ (expected error)")
			}
		})

		t.Run("reader can view STREAM_WRITE but not consume", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("reader", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("reader failed to authenticate: %v", err)
			}
			defer nc.Close()
			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			// Positive: View info
			if _, err := js.StreamInfo("STREAM_WRITE"); err != nil {
				t.Errorf("reader failed to get STREAM_WRITE info: %v", err)
			}

			// Negative: Create consumer (needs consume or manage)
			// Wait, is 'js.view' allowing consumer creation? No.
			if _, err := js.AddConsumer("STREAM_WRITE", &nats.ConsumerConfig{Durable: "reader"}); err == nil {
				t.Errorf("reader succeeded to create consumer on STREAM_WRITE (expected error)")
			}
		})

		t.Run("reader can consume from STREAM_READ:CONSUMER_READ", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("reader", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("reader failed to authenticate: %v", err)
			}
			defer nc.Close()
			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			// Positive: Consume
			sub, err := js.PullSubscribe("", "CONSUMER_READ", nats.BindStream("STREAM_READ"))
			if err != nil {
				t.Errorf("reader failed to bind to CONSUMER_READ: %v", err)
			} else {
				sub.Unsubscribe()
			}

			// Negative: Create new consumer on STREAM_READ
			// 'js.consume' on specific consumer excludes creating others unless *
			// The policy is "js:STREAM_READ:CONSUMER_READ".
			if _, err := js.AddConsumer("STREAM_READ", &nats.ConsumerConfig{Durable: "new-consumer"}); err == nil {
				t.Errorf("reader succeeded to create new consumer on STREAM_READ (expected error)")
			}
		})

		t.Run("alice can consume from STREAM_VARS:alice", func(t *testing.T) {
			// Setup required stream as admin
			adminNc, _ := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			defer adminNc.Close()
			adminJs, _ := adminNc.JetStream()
			adminJs.AddStream(&nats.StreamConfig{Name: "STREAM_VARS", Subjects: []string{"vars.>"}})
			adminJs.AddConsumer("STREAM_VARS", &nats.ConsumerConfig{Durable: "alice"})
			adminJs.AddConsumer("STREAM_VARS", &nats.ConsumerConfig{Durable: "bob"})

			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("alice failed to authenticate: %v", err)
			}
			defer nc.Close()
			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			// Positive
			sub, err := js.PullSubscribe("", "alice", nats.BindStream("STREAM_VARS"))
			if err != nil {
				t.Errorf("alice failed to bind to consumer alice: %v", err)
			} else {
				sub.Unsubscribe()
			}

			// Negative: access bob's consumer
			if _, err := js.PullSubscribe("", "bob", nats.BindStream("STREAM_VARS")); err == nil {
				t.Errorf("alice succeeded to bind to consumer bob (expected error)")
			}
		})
	})
}
