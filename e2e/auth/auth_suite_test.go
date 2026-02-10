package auth_test

import (
	"testing"
	"time"

	"github.com/msimon/nauts/e2e"
	"github.com/nats-io/nats.go"
)

func TestAuthSuite(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4224, func(t *testing.T, env *e2e.TestEnv) {
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

		t.Run("file auth: alice cannot use jwt provider", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-jwt")
			if err == nil {
				nc.Close()
				t.Fatalf("alice authenticated to wrong provider")
			}
		})

		t.Run("jwt auth: bob can authenticate", func(t *testing.T) {
			token := env.GenerateJWT(t, []string{"APP.worker"}, "bob")
			nc, err := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err != nil {
				t.Fatalf("bob failed to authenticate with JWT: %v", err)
			}
			nc.Close()
		})

		// t.Run("aws sigv4: can authenticate (optional)", func(t *testing.T) {
		// 	profile := os.Getenv("NAUTS_E2E_AWS_PROFILE")
		// 	if profile == "" {
		// 		profile = "nauts-role-APP-consumer"
		// 	}

		// 	token, err := env.GenerateAwsSigV4Token(profile)
		// 	if err != nil {
		// 		t.Skipf("AWS SigV4 not configured/available: %v", err)
		// 	}

		// 	nc, err := env.ConnectWithAwsSigV4(token, "APP", "intro-aws")
		// 	if err != nil {
		// 		t.Fatalf("AWS SigV4 authentication failed: %v", err)
		// 	}
		// 	nc.Close()
		// })

		t.Run("bob pub / alice sub", func(t *testing.T) {
			ncAlice, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice connect failed: %v", err)
			}
			defer ncAlice.Close()

			sub, err := ncAlice.SubscribeSync("e2e.mytest")
			if err != nil {
				t.Fatalf("alice subscribe failed: %v", err)
			}
			if err := ncAlice.Flush(); err != nil {
				t.Fatalf("alice flush failed: %v", err)
			}

			token := env.GenerateJWT(t, []string{"APP.worker"}, "bob")
			ncBob, err := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err != nil {
				t.Fatalf("bob connect failed: %v", err)
			}
			defer ncBob.Close()

			if err := ncBob.Publish("e2e.mytest", []byte("hello")); err != nil {
				t.Fatalf("bob publish failed: %v", err)
			}
			if err := ncBob.Flush(); err != nil {
				t.Fatalf("bob flush failed: %v", err)
			}

			msg, err := sub.NextMsg(2 * time.Second)
			if err != nil {
				t.Fatalf("alice failed to receive message: %v", err)
			}
			if string(msg.Data) != "hello" {
				t.Fatalf("unexpected message data: %s", string(msg.Data))
			}
		})

		t.Run("policy: alice cannot publish", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice connection failed: %v", err)
			}
			defer nc.Close()

			errCh := make(chan error, 1)
			nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
				select {
				case errCh <- err:
				default:
				}
			})

			_ = nc.Publish("e2e.mytest", []byte("data"))
			_ = nc.Flush()

			select {
			case <-errCh:
				// expected
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("alice published successfully (timeout waiting for error)")
			}
		})

		t.Run("auth accounts: APP2 role cannot publish in APP", func(t *testing.T) {
			token := env.GenerateJWT(t, []string{"APP2.producer"}, "chris")

			ncApp2, err := env.ConnectWithJwt(token, "APP2", "intro-jwt")
			if err != nil {
				t.Fatalf("chris failed to connect to APP2: %v", err)
			}
			if err := e2e.PublishSync(ncApp2, "e2e.mytest", []byte("data")); err != nil {
				ncApp2.Close()
				t.Fatalf("chris failed to publish in APP2: %v", err)
			}
			ncApp2.Close()

			ncApp, err := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err != nil {
				// Either connect is rejected, or it connects with zero permissions.
				return
			}
			defer ncApp.Close()

			if err := e2e.PublishSync(ncApp, "e2e.mytest", []byte("data")); err == nil {
				t.Fatalf("chris published to APP without error")
			}
		})
	})
}
