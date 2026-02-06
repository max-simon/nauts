package jwt

import (
	"testing"
	"time"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"

	"github.com/msimon/nauts/policy"
)

func TestIssueUserJWT(t *testing.T) {
	// Create account keypair (issuer)
	accountKp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating account keypair: %v", err)
	}
	accountSeed, err := accountKp.Seed()
	if err != nil {
		t.Fatalf("getting account seed: %v", err)
	}
	accountSigner, err := NewLocalSigner(string(accountSeed))
	if err != nil {
		t.Fatalf("creating account signer: %v", err)
	}

	// Create user keypair
	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	// Create permissions
	perms := policy.NewNatsPermissions()
	perms.Allow(policy.Permission{Type: policy.PermPub, Subject: "orders.>"})
	perms.Allow(policy.Permission{Type: policy.PermSub, Subject: "events.>"})

	accountPub := accountSigner.PublicKey()

	// Issue JWT
	token, err := IssueUserJWT("alice", userPub, time.Hour, perms, accountSigner, accountPub, "my-issuer-account")
	if err != nil {
		t.Fatalf("IssueUserJWT error: %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}

	// Decode and verify claims
	claims, err := natsjwt.DecodeUserClaims(token)
	if err != nil {
		t.Fatalf("decoding user claims: %v", err)
	}

	if claims.Name != "alice" {
		t.Errorf("name = %q, want %q", claims.Name, "alice")
	}

	if claims.Subject != userPub {
		t.Errorf("subject = %q, want %q", claims.Subject, userPub)
	}

	if len(claims.Permissions.Pub.Allow) != 1 || claims.Permissions.Pub.Allow[0] != "orders.>" {
		t.Errorf("pub allow = %v, want [orders.>]", claims.Permissions.Pub.Allow)
	}

	if len(claims.Permissions.Sub.Allow) != 1 || claims.Permissions.Sub.Allow[0] != "events.>" {
		t.Errorf("sub allow = %v, want [events.>]", claims.Permissions.Sub.Allow)
	}

	// Check audience is set to account public key
	if claims.Audience != accountPub {
		t.Errorf("audience = %q, want %q", claims.Audience, accountPub)
	}

	// Check issuer account
	if claims.IssuerAccount != "my-issuer-account" {
		t.Errorf("issuer account = %q, want %q", claims.IssuerAccount, "my-issuer-account")
	}

	// Check expiry is set
	if claims.Expires == 0 {
		t.Error("expected non-zero expiry")
	}
}

func TestIssueUserJWT_NoPermissions(t *testing.T) {
	accountKp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating account keypair: %v", err)
	}
	accountSeed, err := accountKp.Seed()
	if err != nil {
		t.Fatalf("getting account seed: %v", err)
	}
	accountSigner, err := NewLocalSigner(string(accountSeed))
	if err != nil {
		t.Fatalf("creating account signer: %v", err)
	}

	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	accountPub := accountSigner.PublicKey()

	token, err := IssueUserJWT("bob", userPub, 0, nil, accountSigner, accountPub, "")
	if err != nil {
		t.Fatalf("IssueUserJWT error: %v", err)
	}

	claims, err := natsjwt.DecodeUserClaims(token)
	if err != nil {
		t.Fatalf("decoding user claims: %v", err)
	}

	if claims.Name != "bob" {
		t.Errorf("name = %q, want %q", claims.Name, "bob")
	}

	// No expiry when ttl is 0
	if claims.Expires != 0 {
		t.Errorf("expected zero expiry for ttl=0, got %d", claims.Expires)
	}
}

func TestIssueUserJWT_ZeroTTL(t *testing.T) {
	accountKp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating account keypair: %v", err)
	}
	accountSeed, err := accountKp.Seed()
	if err != nil {
		t.Fatalf("getting account seed: %v", err)
	}
	accountSigner, err := NewLocalSigner(string(accountSeed))
	if err != nil {
		t.Fatalf("creating account signer: %v", err)
	}

	userKp, err := nkeys.CreateUser()
	if err != nil {
		t.Fatalf("creating user keypair: %v", err)
	}
	userPub, err := userKp.PublicKey()
	if err != nil {
		t.Fatalf("getting user public key: %v", err)
	}

	accountPub := accountSigner.PublicKey()

	token, err := IssueUserJWT("user", userPub, 0, nil, accountSigner, accountPub, "")
	if err != nil {
		t.Fatalf("IssueUserJWT error: %v", err)
	}

	claims, err := natsjwt.DecodeUserClaims(token)
	if err != nil {
		t.Fatalf("decoding user claims: %v", err)
	}

	// Zero TTL means no expiry
	if claims.Expires != 0 {
		t.Errorf("expires = %d, want 0", claims.Expires)
	}
}
