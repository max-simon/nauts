package jwt

import (
	"testing"

	"github.com/nats-io/nkeys"
)

func TestNewLocalSigner(t *testing.T) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating test keypair: %v", err)
	}

	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("getting seed: %v", err)
	}

	expectedPub, err := kp.PublicKey()
	if err != nil {
		t.Fatalf("getting public key: %v", err)
	}

	signer, err := NewLocalSigner(string(seed))
	if err != nil {
		t.Fatalf("creating local signer: %v", err)
	}

	if signer.PublicKey() != expectedPub {
		t.Errorf("public key mismatch: got %s, want %s", signer.PublicKey(), expectedPub)
	}
}

func TestNewLocalSigner_InvalidSeed(t *testing.T) {
	_, err := NewLocalSigner("invalid-seed")
	if err == nil {
		t.Error("expected error for invalid seed, got nil")
	}
}

func TestLocalSigner_Sign(t *testing.T) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating test keypair: %v", err)
	}

	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("getting seed: %v", err)
	}

	signer, err := NewLocalSigner(string(seed))
	if err != nil {
		t.Fatalf("creating local signer: %v", err)
	}

	data := []byte("test data to sign")
	sig, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("signing data: %v", err)
	}

	if len(sig) == 0 {
		t.Error("expected non-empty signature")
	}

	err = kp.Verify(data, sig)
	if err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestLocalSigner_SignVerifyWithPublicKey(t *testing.T) {
	kp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating test keypair: %v", err)
	}

	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("getting seed: %v", err)
	}

	signer, err := NewLocalSigner(string(seed))
	if err != nil {
		t.Fatalf("creating local signer: %v", err)
	}

	data := []byte("test data to sign")
	sig, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("signing data: %v", err)
	}

	pubKey, err := nkeys.FromPublicKey(signer.PublicKey())
	if err != nil {
		t.Fatalf("creating public key: %v", err)
	}

	err = pubKey.Verify(data, sig)
	if err != nil {
		t.Errorf("signature verification with public key failed: %v", err)
	}
}
