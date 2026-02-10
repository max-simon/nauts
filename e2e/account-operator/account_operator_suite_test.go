package account_operator_test

import (
	"testing"

	"github.com/msimon/nauts/e2e"
)

func TestAccountOperatorSuite(t *testing.T) {
	e2e.WithTestEnv(t, ".", "operator", 4223, func(t *testing.T, env *e2e.TestEnv) {
		t.Run("file auth: alice can authenticate", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "APP", "intro-file")
			if err != nil {
				t.Fatalf("alice failed to authenticate: %v", err)
			}
			nc.Close()
		})

		t.Run("unknown account is rejected", func(t *testing.T) {
			nc, err := env.ConnectWithUsernameAndPassword("alice", "secret", "NOPE", "intro-file")
			if err == nil {
				nc.Close()
				t.Fatalf("unexpectedly authenticated to unknown account")
			}
		})
	})
}
