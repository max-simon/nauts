package e2e

import (
	"flag"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	skipPolicyNats = flag.Bool("skipPolicyNats", false, "Skip policy nats tests")
)

func TestPoliciesNats(t *testing.T) {

	if *skipPolicyNats {
		t.Log("Skipping policy nats tests")
		return
	}

	WithTestEnv(t, "policy-static", "static", 4232, func(t *testing.T, env *TestEnv) {

		t.Run("admin can pub/sub to all subjects", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("Admin failed to authenticate: %v", err)
			}
			defer nc.Close()
			if err := PublishSync(nc, "any.subject", []byte("data")); err != nil {
				t.Errorf("Admin failed to publish: %v", err)
			}
			_, err = SubscribeSyncWithCheck(nc, "any.subject")
			if err != nil {
				t.Errorf("Admin failed to subscribe: %v", err)
			}
		})

		t.Run("writer can publish to public.> but not private.>", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("writer", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("writer failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Positive test
			if err := PublishSync(nc, "public.news", []byte("news")); err != nil {
				t.Errorf("writer failed to publish to public.news: %v", err)
			}

			// Negative test (publish)
			if err := PublishSync(nc, "private.data", []byte("secret")); err == nil {
				t.Errorf("writer succeeded to publish to private.data (expected error)")
			}

			// Negative test (subscribe)
			if _, err := SubscribeSyncWithCheck(nc, "public.news"); err == nil {
				t.Errorf("writer succeeded to subscribe to public.news (expected error)")
			}
		})

		t.Run("reader can subscribe to public.> but not publish", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("reader", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("reader failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Positive test (subscribe)
			sub, err := SubscribeSyncWithCheck(nc, "public.news")
			if err != nil {
				t.Errorf("reader failed to subscribe to public.news: %v", err)
			}

			// Verify receive
			adminNc, _ := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			defer adminNc.Close()
			PublishSync(adminNc, "public.news", []byte("hello"))
			if _, err := sub.NextMsg(time.Second); err != nil {
				t.Errorf("reader failed to receive message: %v", err)
			}

			// Negative test (publish)
			if err := PublishSync(nc, "public.news", []byte("data")); err == nil {
				t.Errorf("reader succeeded to publish (expected error)")
			}
		})

		t.Run("service can respond to requests", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("service", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("service failed to authenticate: %v", err)
			}
			defer nc.Close()

			sub, err := nc.Subscribe("service.help", func(m *nats.Msg) {
				m.Respond([]byte("ok"))
			})
			if err != nil {
				t.Fatalf("service failed to subscribe for reply: %v", err)
			}
			defer sub.Unsubscribe()

			// Setup requester
			adminNc, _ := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			defer adminNc.Close()

			msg, err := adminNc.Request("service.help", []byte("?"), time.Second)
			if err != nil {
				t.Errorf("request failed: %v", err)
			} else if string(msg.Data) != "ok" {
				t.Errorf("got wrong response: %s", string(msg.Data))
			}

			// Negative test: service cannot subscribe to other subjects
			if _, err := SubscribeSyncWithCheck(nc, "other.subject"); err == nil {
				t.Errorf("service succeeded to subscribe to other.subject (expected error)")
			}
		})

		t.Run("alice can pub/sub to user.alice.>", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("alice failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Positive
			if err := PublishSync(nc, "user.alice.data", []byte("mydata")); err != nil {
				t.Errorf("alice failed to publish: %v", err)
			}
			if _, err := SubscribeSyncWithCheck(nc, "user.alice.data"); err != nil {
				t.Errorf("alice failed to subscribe: %v", err)
			}

			// Negative: alice accessing bob's subject
			if err := PublishSync(nc, "user.bob.data", []byte("hack")); err == nil {
				t.Errorf("alice succeeded to publish to bob's subject (expected error)")
			}
		})
	})
}
