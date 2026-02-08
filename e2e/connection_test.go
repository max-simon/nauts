package e2e

import (
	"flag"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	skipConnectionOperator = flag.Bool("skipConnectionOperator", false, "Skip operator mode tests")
	skipConnectionStatic   = flag.Bool("skipConnectionStatic", false, "Skip static mode tests")
)

func RunNatsConnectionTestProcedure(t *testing.T, dir string, mode string, port int) {

	WithTestEnv(t, dir, mode, port, func(t *testing.T, env *TestEnv) {

		t.Run("alice can authenticate", func(t *testing.T) {
			// Alice uses File provider "intro-file"
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("Alice failed to authenticate: %v", err)
			}
			nc.Close()
		})

		t.Run("alice can not authenticate with wrong password", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "wrong", "APP", "intro-file")
			if err == nil {
				nc.Close()
				t.Fatalf("Alice authenticated with wrong password")
			}
		})

		t.Run("alice can not authenticate via JWT provider", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-jwt")
			if err == nil {
				nc.Close()
				t.Fatalf("Alice authenticated to wrong provider")
			}
		})

		t.Run("bob can authenticate", func(t *testing.T) {
			token := env.GenerateJWT(t, []string{"APP.worker", "APP2.consumer"}, "bob")
			// Bob uses JWT provider "intro-jwt"
			nc, err := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err != nil {
				t.Fatalf("Bob failed to authenticate with JWT: %v", err)
			}
			nc.Close()
		})

		t.Run("alice can not publish to e2e.mytest", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("Alice connection failed: %v", err)
			}
			defer nc.Close()

			// Expect publish error (async)
			errCh := make(chan error, 1)
			nc.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
				errCh <- err
			})

			if err := nc.Publish("e2e.mytest", []byte("data")); err != nil {
				// If we get immediate error, that's good too
			}
			nc.Flush()

			select {
			case err := <-errCh:
				if err == nil {
					t.Fatalf("Expected permission error, got nil")
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("Alice published successfully (timeout waiting for error)")
			}
		})

		t.Run("bob pub alice sub", func(t *testing.T) {
			// Alice connects
			ncAlice, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("Alice connect failed: %v", err)
			}
			defer ncAlice.Close()

			sub, err := ncAlice.SubscribeSync("e2e.mytest")
			if err != nil {
				t.Fatalf("Alice subscribe failed: %v", err)
			}
			ncAlice.Flush()

			// Bob connects
			token := env.GenerateJWT(t, []string{"APP.worker"}, "bob")
			ncBob, err := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err != nil {
				t.Fatalf("Bob connect failed: %v", err)
			}
			defer ncBob.Close()

			if err := ncBob.Publish("e2e.mytest", []byte("hello")); err != nil {
				t.Fatalf("Bob publish failed: %v", err)
			}
			ncBob.Flush()

			msg, err := sub.NextMsg(2 * time.Second)
			if err != nil {
				t.Fatalf("Alice failed to receive message: %v", err)
			}
			if string(msg.Data) != "hello" {
				t.Errorf("Unexpected message data: %s", string(msg.Data))
			}
		})

		t.Run("chris auth accounts", func(t *testing.T) {
			token := env.GenerateJWT(t, []string{"APP2.producer"}, "chris")

			// Connect to APP2 -> Success
			nc, err := env.ConnectWithJwt(token, "APP2", "intro-jwt")
			if err != nil {
				t.Fatalf("Chris failed to connect to APP2: %v", err)
			}
			if err := nc.Publish("e2e.mytest", []byte("data")); err != nil {
				t.Fatalf("Chris failed to publish to APP2: %v", err)
			}
			nc.Flush()
			nc.Close()

			// Connect to APP -> Fail (or Connect but no permissions)
			nc2, err2 := env.ConnectWithJwt(token, "APP", "intro-jwt")
			if err2 != nil {
				t.Fatalf("Chris failed to connect to APP: %v", err2)
			}
			defer nc2.Close()

			// Allow time for async error
			errCh := make(chan error, 1)
			nc2.SetErrorHandler(func(_ *nats.Conn, _ *nats.Subscription, err error) {
				errCh <- err
			})

			if err := nc2.Publish("e2e.mytest", []byte("data")); err != nil {
				// Immediate error is also fine
				return
			}
			nc2.Flush()

			select {
			case err := <-errCh:
				// Expected error
				t.Logf("Chris correctly received error: %v", err)
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("Chris published to APP without error")
			}
		})
	})
}

func TestNatsConnectionStatic(t *testing.T) {
	if !*skipConnectionStatic {
		t.Log("Running tests in static mode")
		RunNatsConnectionTestProcedure(t, "connection-static", "static", 4222)
	} else {
		t.Log("Do not run tests in static mode")
	}
}

func TestNatsConnectionOperator(t *testing.T) {
	if !*skipConnectionOperator {
		t.Log("Running tests in operator mode")
		RunNatsConnectionTestProcedure(t, "connection-operator", "operator", 4223)
	} else {
		t.Log("Do not run tests in operator mode")
	}
}
