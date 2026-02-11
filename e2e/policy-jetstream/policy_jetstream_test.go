package policy_jetstream_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/msimon/nauts/e2e"
	"github.com/nats-io/nats.go"
)

func TestPoliciesJetstream(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4231, nil, func(t *testing.T, env *e2e.TestEnv) {
		adminNc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
		if err != nil {
			t.Fatalf("setup failed to authenticate as admin: %v", err)
		}
		defer adminNc.Close()

		adminJs, err := adminNc.JetStream()
		if err != nil {
			t.Fatalf("setup failed to get JetStream context: %v", err)
		}

		// STREAM_CONS is the stream used by view/consume tests.
		if _, err := adminJs.AddStream(&nats.StreamConfig{Name: "STREAM_CONS", Subjects: []string{"events.>"}}); err != nil {
			t.Fatalf("setup failed to create STREAM_CONS: %v", err)
		}
		if _, err := adminJs.AddConsumer("STREAM_CONS", &nats.ConsumerConfig{Durable: "CONSUMER_A", AckPolicy: nats.AckExplicitPolicy}); err != nil {
			t.Fatalf("setup failed to create CONSUMER_A: %v", err)
		}
		if _, err := adminJs.AddConsumer("STREAM_CONS", &nats.ConsumerConfig{Durable: "CONSUMER_B", AckPolicy: nats.AckExplicitPolicy}); err != nil {
			t.Fatalf("setup failed to create CONSUMER_B: %v", err)
		}

		// Seed some messages for consume tests.
		for i := 0; i < 3; i++ {
			if _, err := adminJs.Publish("events.seed", []byte(fmt.Sprintf("msg-%d", i))); err != nil {
				t.Fatalf("setup failed to publish seed message: %v", err)
			}
		}

		t.Run("js.manage: manager can create/update/purge/delete STREAM_MGR (but cannot view STREAM_CONS)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("manager", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("manager failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			if _, err := js.AddStream(&nats.StreamConfig{Name: "STREAM_MGR", Subjects: []string{"mgr.>"}}); err != nil {
				t.Fatalf("manager failed to create STREAM_MGR: %v", err)
			}
			if _, err := js.UpdateStream(&nats.StreamConfig{Name: "STREAM_MGR", Subjects: []string{"mgr.>", "mgr2.>"}}); err != nil {
				t.Fatalf("manager failed to update STREAM_MGR: %v", err)
			}
			if err := js.PurgeStream("STREAM_MGR"); err != nil {
				t.Fatalf("manager failed to purge STREAM_MGR: %v", err)
			}
			if err := js.DeleteStream("STREAM_MGR"); err != nil {
				t.Fatalf("manager failed to delete STREAM_MGR: %v", err)
			}

			if _, err := js.StreamInfo("STREAM_CONS"); err == nil {
				t.Fatalf("manager succeeded to view STREAM_CONS (expected error)")
			}
		})

		t.Run("js.view: viewer can view stream and consumer info (but cannot manage or consume)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("viewer", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("viewer failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			if _, err := js.StreamInfo("STREAM_CONS"); err != nil {
				t.Fatalf("viewer failed to get STREAM_CONS info: %v", err)
			}
			if _, err := js.ConsumerInfo("STREAM_CONS", "CONSUMER_A"); err != nil {
				t.Fatalf("viewer failed to get CONSUMER_A info: %v", err)
			}

			if _, err := js.UpdateStream(&nats.StreamConfig{Name: "STREAM_CONS", Subjects: []string{"events.>", "events2.>"}}); err == nil {
				t.Fatalf("viewer succeeded to update STREAM_CONS (expected error)")
			}
			sub, err := js.PullSubscribe("", "CONSUMER_A", nats.BindStream("STREAM_CONS"))
			if err != nil {
				// Some client versions may attempt additional consume operations during setup.
				// Any error here is acceptable as long as consume is not granted.
				return
			}
			defer sub.Unsubscribe()
			if _, err := sub.Fetch(1, nats.MaxWait(1*time.Second)); err == nil {
				t.Fatalf("viewer succeeded to fetch messages from CONSUMER_A (expected error)")
			}
		})

		t.Run("js.consume: consumerA can consume only from STREAM_CONS:CONSUMER_A", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("consumerA", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("consumerA failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			sub, err := js.PullSubscribe("", "CONSUMER_A", nats.BindStream("STREAM_CONS"))
			if err != nil {
				t.Fatalf("consumerA failed to bind to CONSUMER_A: %v", err)
			}
			msgs, err := sub.Fetch(1, nats.MaxWait(2*time.Second))
			if err != nil {
				t.Fatalf("consumerA failed to fetch message: %v", err)
			}
			if len(msgs) != 1 {
				t.Fatalf("consumerA expected 1 message, got %d", len(msgs))
			}
			_ = msgs[0].Ack()
			_ = sub.Unsubscribe()

			if _, err := js.PullSubscribe("", "CONSUMER_B", nats.BindStream("STREAM_CONS")); err == nil {
				t.Fatalf("consumerA succeeded to bind to CONSUMER_B (expected error)")
			}
		})

		t.Run("js.consume: consumer wildcard '*' can consume from any consumer on STREAM_CONS", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("consumerStar", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("consumerStar failed to authenticate: %v", err)
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				t.Fatal(err)
			}

			subA, err := js.PullSubscribe("", "CONSUMER_A", nats.BindStream("STREAM_CONS"))
			if err != nil {
				t.Fatalf("consumerStar failed to bind to CONSUMER_A: %v", err)
			}
			_ = subA.Unsubscribe()

			subB, err := js.PullSubscribe("", "CONSUMER_B", nats.BindStream("STREAM_CONS"))
			if err != nil {
				t.Fatalf("consumerStar failed to bind to CONSUMER_B: %v", err)
			}
			_ = subB.Unsubscribe()
		})
	})
}
