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

// TestPermissionsToNats_EmptyDenyAll verifies that empty permission lists
// result in explicit deny all. This is critical because NATS JWT defaults
// to allowing everything when no permissions are specified.
func TestPermissionsToNats_EmptyDenyAll(t *testing.T) {
	tests := []struct {
		name         string
		perms        *policy.NatsPermissions
		wantPubAllow []string
		wantPubDeny  []string
		wantSubAllow []string
		wantSubDeny  []string
	}{
		{
			name:         "empty permissions should deny all pub and sub",
			perms:        policy.NewNatsPermissions(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: nil,
			wantSubDeny:  []string{">"},
		},
		{
			name: "pub only should deny all sub",
			perms: func() *policy.NatsPermissions {
				p := policy.NewNatsPermissions()
				p.Allow(policy.Permission{Type: policy.PermPub, Subject: "foo.>"})
				return p
			}(),
			wantPubAllow: []string{"foo.>"},
			wantPubDeny:  nil,
			wantSubAllow: nil,
			wantSubDeny:  []string{">"},
		},
		{
			name: "sub only should deny all pub",
			perms: func() *policy.NatsPermissions {
				p := policy.NewNatsPermissions()
				p.Allow(policy.Permission{Type: policy.PermSub, Subject: "foo.>"})
				return p
			}(),
			wantPubAllow: nil,
			wantPubDeny:  []string{">"},
			wantSubAllow: []string{"foo.>"},
			wantSubDeny:  nil,
		},
		{
			name: "both pub and sub should not have any denies",
			perms: func() *policy.NatsPermissions {
				p := policy.NewNatsPermissions()
				p.Allow(policy.Permission{Type: policy.PermPub, Subject: "foo.>"})
				p.Allow(policy.Permission{Type: policy.PermSub, Subject: "bar.>"})
				return p
			}(),
			wantPubAllow: []string{"foo.>"},
			wantPubDeny:  nil,
			wantSubAllow: []string{"bar.>"},
			wantSubDeny:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := permissionsToNats(tt.perms)

			if !stringSliceEqual(got.Pub.Allow, tt.wantPubAllow) {
				t.Errorf("Pub.Allow = %v, want %v", got.Pub.Allow, tt.wantPubAllow)
			}
			if !stringSliceEqual(got.Pub.Deny, tt.wantPubDeny) {
				t.Errorf("Pub.Deny = %v, want %v", got.Pub.Deny, tt.wantPubDeny)
			}
			if !stringSliceEqual(got.Sub.Allow, tt.wantSubAllow) {
				t.Errorf("Sub.Allow = %v, want %v", got.Sub.Allow, tt.wantSubAllow)
			}
			if !stringSliceEqual(got.Sub.Deny, tt.wantSubDeny) {
				t.Errorf("Sub.Deny = %v, want %v", got.Sub.Deny, tt.wantSubDeny)
			}
		})
	}
}

func stringSliceEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
