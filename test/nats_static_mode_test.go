package test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nats-io/nats.go"
)

func TestNatsStaticMode(t *testing.T) {
	// This functionality relies on the "nats-static-mode" directory existing in test/
	env := newTestEnv(t, "nats-static-mode")
	env.start()
	defer env.stop()

	// Wait for services to be ready
	time.Sleep(2 * time.Second)

	// Helper to generate JWT signed by the test key
	generateJWT := func(t *testing.T, roles []string, sub string) string {
		keyPath := filepath.Join(env.baseDir, "rsa.key")
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			t.Fatalf("failed to read private key: %v", err)
		}

		// Parse PEM
		block, _ := pem.Decode(keyBytes)
		if block == nil {
			t.Fatalf("failed to parse PEM block")
		}

		// Parse Key (PKCS8 in the python script)
		privKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			t.Fatalf("failed to parse private key: %v", err)
		}

		now := time.Now()
		claims := jwt.MapClaims{
			"iss": "e2e",
			"sub": sub,
			"iat": now.Unix(),
			"exp": now.Add(time.Hour).Unix(),
			"nauts": map[string]interface{}{
				"roles": roles,
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		signed, err := token.SignedString(privKey)
		if err != nil {
			t.Fatalf("failed to sign token: %v", err)
		}
		return signed
	}

	// Helper to connect with username/password and specific provider
	connectUser := func(t *testing.T, username, password, account, providerID string) (*nats.Conn, error) {
		innerToken := username
		if password != "" {
			innerToken += ":" + password
		}
		// only add ap to json if given
		apOpt := ""
		if providerID != "" {
			apOpt = fmt.Sprintf(`,"ap":%q`, providerID)
		}
		tokenJson := fmt.Sprintf(`{"account":%q,"token":%q%s}`, account, innerToken, apOpt)

		opts := []nats.Option{
			nats.Name(fmt.Sprintf("nauts-e2e-%s", username)),
			nats.Token(tokenJson),
			nats.Timeout(2 * time.Second),
		}
		return nats.Connect(nats.DefaultURL, opts...)
	}

	// Helper to connect with JWT token
	connectJwt := func(t *testing.T, account, token, providerID string) (*nats.Conn, error) {
		apOpt := ""
		if providerID != "" {
			apOpt = fmt.Sprintf(`,"ap":%q`, providerID)
		}
		tokenJson := fmt.Sprintf(`{"account":%q,"token":%q%s}`, account, token, apOpt)

		opts := []nats.Option{
			nats.Name(fmt.Sprintf("nauts-e2e-test-jwt-%s", account)),
			nats.Token(tokenJson),
			nats.Timeout(2 * time.Second),
		}
		return nats.Connect(nats.DefaultURL, opts...)
	}

	t.Run("alice can authenticate", func(t *testing.T) {
		// Alice uses File provider "intro-file"
		nc, err := connectUser(t, "alice", "secret", "APP", "intro-file")
		if err != nil {
			t.Fatalf("Alice failed to authenticate: %v", err)
		}
		nc.Close()
	})

	t.Run("alice can not authenticate with wrong password", func(t *testing.T) {
		nc, err := connectUser(t, "alice", "wrong", "APP", "intro-file")
		if err == nil {
			nc.Close()
			t.Fatalf("Alice authenticated with wrong password")
		}
	})

	t.Run("alice can not authenticate via JWT provider", func(t *testing.T) {
		nc, err := connectUser(t, "alice", "secret", "APP", "intro-jwt")
		if err == nil {
			nc.Close()
			t.Fatalf("Alice authenticated to wrong provider")
		}
	})

	t.Run("bob can authenticate", func(t *testing.T) {
		token := generateJWT(t, []string{"APP.worker", "APP2.consumer"}, "bob")
		// Bob uses JWT provider "intro-jwt"
		nc, err := connectJwt(t, "APP", token, "intro-jwt")
		if err != nil {
			t.Fatalf("Bob failed to authenticate with JWT: %v", err)
		}
		nc.Close()
	})

	t.Run("alice can not publish to e2e.mytest", func(t *testing.T) {
		nc, err := connectUser(t, "alice", "secret", "APP", "intro-file")
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
		ncAlice, err := connectUser(t, "alice", "secret", "APP", "intro-file")
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
		token := generateJWT(t, []string{"APP.worker"}, "bob")
		ncBob, err := connectJwt(t, "APP", token, "intro-jwt")
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
		token := generateJWT(t, []string{"APP2.producer"}, "chris")

		// Connect to APP2 -> Success
		nc, err := connectJwt(t, "APP2", token, "intro-jwt")
		if err != nil {
			t.Fatalf("Chris failed to connect to APP2: %v", err)
		}
		if err := nc.Publish("e2e.mytest", []byte("data")); err != nil {
			t.Fatalf("Chris failed to publish to APP2: %v", err)
		}
		nc.Flush()
		nc.Close()

		// Connect to APP -> Fail (or Connect but no permissions)
		nc2, err2 := connectJwt(t, "APP", token, "intro-jwt")
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
}
