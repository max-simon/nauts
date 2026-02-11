package auth_test

import (
	"os"
	"testing"

	"github.com/msimon/nauts/e2e"
)

func TestAuthSuite(t *testing.T) {
	e2e.WithTestEnv(t, ".", "static", 4224, nil, func(t *testing.T, env *e2e.TestEnv) {
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

		t.Run("jwt auth: bob can not authenticate in unknown account", func(t *testing.T) {
			token := env.GenerateJWT(t, []string{"APP3.worker"}, "bob")
			nc, err := env.ConnectWithJwt(token, "APP3", "intro-jwt")
			if err == nil {
				t.Fatalf("bob authenticated with JWT to unknown account: %v", err)
			}
			nc.Close()
		})

		t.Run("aws sigv4: can authenticate (optional)", func(t *testing.T) {
			profile := os.Getenv("NAUTS_E2E_AWS_PROFILE")
			if profile == "" {
				profile = "nauts-role-APP-consumer"
			}

			token, err := env.GenerateAwsSigV4Token(profile)
			if err != nil {
				t.Skipf("AWS SigV4 not configured/available: %v", err)
			}

			nc, err := env.ConnectWithAwsSigV4(token, "APP", "intro-aws")
			if err != nil {
				t.Fatalf("AWS SigV4 authentication failed: %v", err)
			}
			nc.Close()
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
