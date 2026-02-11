package policy_nats_test

import (
	"testing"
	"time"

	"github.com/msimon/nauts/e2e"
	"github.com/nats-io/nats.go"
)

func TestPoliciesNats(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4232, nil, func(t *testing.T, env *e2e.TestEnv) {
		t.Run("admin can pub/sub to all subjects", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("admin failed to authenticate: %v", err)
			}
			defer nc.Close()
			if err := e2e.PublishSync(nc, "any.subject", []byte("data")); err != nil {
				t.Fatalf("admin failed to publish: %v", err)
			}
			if _, err := e2e.SubscribeSyncWithCheck(nc, "any.subject"); err != nil {
				t.Fatalf("admin failed to subscribe: %v", err)
			}
		})

		t.Run("pub action: allowed to publish (uses user.id + role.id)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("pubber", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("pubber failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Policy resource is: nats:pub.{{ user.id }}.{{ role.id }}
			if err := e2e.PublishSync(nc, "pub.pubber.pub", []byte("news")); err != nil {
				t.Fatalf("pubber failed to publish to pub.pubber.pub: %v", err)
			}
			if err := e2e.PublishSync(nc, "pub.other.pub", []byte("secret")); err == nil {
				t.Fatalf("pubber succeeded to publish to pub.other.pub (expected error)")
			}
			if _, err := e2e.SubscribeSyncWithCheck(nc, "pub.pubber.pub"); err == nil {
				t.Fatalf("pubber succeeded to subscribe (expected error)")
			}
		})

		t.Run("sub action: allowed to subscribe", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("subber", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("subber failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Policy resource is: nats:sub.e2e
			sub, err := e2e.SubscribeSyncWithCheck(nc, "sub.e2e")
			if err != nil {
				t.Fatalf("subber failed to subscribe to sub.e2e: %v", err)
			}
			defer sub.Unsubscribe()
			if _, err := e2e.SubscribeSyncWithCheck(nc, "nope.e2e"); err == nil {
				t.Fatalf("subber succeeded to subscribe to nope.e2e (expected error)")
			}

			adminNc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("admin connect failed: %v", err)
			}
			defer adminNc.Close()
			_ = e2e.PublishSync(adminNc, "sub.e2e", []byte("hello"))
			if _, err := sub.NextMsg(time.Second); err != nil {
				t.Fatalf("subber failed to receive message: %v", err)
			}

			if err := e2e.PublishSync(nc, "sub.e2e", []byte("data")); err == nil {
				t.Fatalf("subber succeeded to publish (expected error)")
			}
		})

		t.Run("service action: allowed to listen and respond (uses role.id)", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("service", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("service failed to authenticate: %v", err)
			}
			defer nc.Close()

			// Policy resource is: nats:svc.{{ role.id }}
			sub, err := nc.Subscribe("svc.service", func(m *nats.Msg) {
				_ = m.Respond([]byte("ok"))
			})
			if err != nil {
				t.Fatalf("service failed to subscribe for reply: %v", err)
			}
			defer sub.Unsubscribe()

			adminNc, err := env.ConnectWithUsernameAndPassword("admin", "secret", "POLICY", "policy-file")
			if err != nil {
				t.Fatalf("admin connect failed: %v", err)
			}
			defer adminNc.Close()

			msg, err := adminNc.Request("svc.service", []byte("?"), time.Second)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if string(msg.Data) != "ok" {
				t.Fatalf("got wrong response: %s", string(msg.Data))
			}

			if _, err := e2e.SubscribeSyncWithCheck(nc, "other.subject"); err == nil {
				t.Fatalf("service succeeded to subscribe to other.subject (expected error)")
			}
		})
	})
}
