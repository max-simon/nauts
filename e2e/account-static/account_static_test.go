package account_static_test

import (
	"testing"

	"github.com/msimon/nauts/e2e"
)

func TestAccountStaticSuite(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4222, func(t *testing.T, env *e2e.TestEnv) {
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
