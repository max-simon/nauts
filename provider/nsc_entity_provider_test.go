package provider

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	natsjwt "github.com/nats-io/jwt/v2"
	"github.com/nats-io/nkeys"
)

func TestNewNscEntityProvider(t *testing.T) {
	nscDir := setupTestNscDir(t)

	provider, err := NewNscEntityProvider(NscConfig{
		Dir:          nscDir,
		OperatorName: "test-operator",
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	ctx := context.Background()

	op, err := provider.GetOperator(ctx)
	if err != nil {
		t.Fatalf("getting operator: %v", err)
	}

	if op.Name() != "test-operator" {
		t.Errorf("operator name mismatch: got %s, want test-operator", op.Name())
	}

	if op.PublicKey() == "" {
		t.Error("expected non-empty operator public key")
	}

	if op.Signer() == nil {
		t.Error("expected non-nil operator signer")
	}

	acc, err := provider.GetAccount(ctx, "test-account")
	if err != nil {
		t.Fatalf("getting account: %v", err)
	}

	if acc.Name() != "test-account" {
		t.Errorf("account name mismatch: got %s, want test-account", acc.Name())
	}

	if acc.PublicKey() == "" {
		t.Error("expected non-empty account public key")
	}

	if acc.Signer() == nil {
		t.Error("expected non-nil account signer")
	}
}

func TestNewNscEntityProvider_MissingDir(t *testing.T) {
	_, err := NewNscEntityProvider(NscConfig{
		Dir:          "",
		OperatorName: "test-operator",
	})
	if err == nil {
		t.Error("expected error for missing dir, got nil")
	}
}

func TestNewNscEntityProvider_MissingOperatorName(t *testing.T) {
	_, err := NewNscEntityProvider(NscConfig{
		Dir:          "/tmp/nsc",
		OperatorName: "",
	})
	if err == nil {
		t.Error("expected error for missing operator name, got nil")
	}
}

func TestNewNscEntityProvider_OperatorNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	_, err := NewNscEntityProvider(NscConfig{
		Dir:          tmpDir,
		OperatorName: "nonexistent",
	})
	if err == nil {
		t.Error("expected error for nonexistent operator, got nil")
	}
	if !errors.Is(err, ErrOperatorNotFound) {
		t.Errorf("expected ErrOperatorNotFound, got: %v", err)
	}
}

func TestNscEntityProvider_GetAccount_NotFound(t *testing.T) {
	nscDir := setupTestNscDir(t)

	provider, err := NewNscEntityProvider(NscConfig{
		Dir:          nscDir,
		OperatorName: "test-operator",
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	ctx := context.Background()

	_, err = provider.GetAccount(ctx, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent account, got nil")
	}
	if !errors.Is(err, ErrAccountNotFound) {
		t.Errorf("expected ErrAccountNotFound, got: %v", err)
	}
}

func TestNscEntityProvider_ListAccounts(t *testing.T) {
	nscDir := setupTestNscDir(t)

	provider, err := NewNscEntityProvider(NscConfig{
		Dir:          nscDir,
		OperatorName: "test-operator",
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	ctx := context.Background()

	accounts, err := provider.ListAccounts(ctx)
	if err != nil {
		t.Fatalf("listing accounts: %v", err)
	}

	if len(accounts) != 1 {
		t.Errorf("expected 1 account, got %d", len(accounts))
	}

	if accounts[0].Name() != "test-account" {
		t.Errorf("account name mismatch: got %s, want test-account", accounts[0].Name())
	}
}

func TestNscEntityProvider_SignAndVerify(t *testing.T) {
	nscDir := setupTestNscDir(t)

	provider, err := NewNscEntityProvider(NscConfig{
		Dir:          nscDir,
		OperatorName: "test-operator",
	})
	if err != nil {
		t.Fatalf("creating provider: %v", err)
	}

	ctx := context.Background()

	op, err := provider.GetOperator(ctx)
	if err != nil {
		t.Fatalf("getting operator: %v", err)
	}

	data := []byte("test data")
	sig, err := op.Signer().Sign(data)
	if err != nil {
		t.Fatalf("signing with operator: %v", err)
	}

	pubKey, err := nkeys.FromPublicKey(op.PublicKey())
	if err != nil {
		t.Fatalf("creating public key: %v", err)
	}

	if err := pubKey.Verify(data, sig); err != nil {
		t.Errorf("operator signature verification failed: %v", err)
	}

	acc, err := provider.GetAccount(ctx, "test-account")
	if err != nil {
		t.Fatalf("getting account: %v", err)
	}

	sig, err = acc.Signer().Sign(data)
	if err != nil {
		t.Fatalf("signing with account: %v", err)
	}

	pubKey, err = nkeys.FromPublicKey(acc.PublicKey())
	if err != nil {
		t.Fatalf("creating public key: %v", err)
	}

	if err := pubKey.Verify(data, sig); err != nil {
		t.Errorf("account signature verification failed: %v", err)
	}
}

func setupTestNscDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	opKp, err := nkeys.CreateOperator()
	if err != nil {
		t.Fatalf("creating operator keypair: %v", err)
	}

	opPub, err := opKp.PublicKey()
	if err != nil {
		t.Fatalf("getting operator public key: %v", err)
	}

	opSeed, err := opKp.Seed()
	if err != nil {
		t.Fatalf("getting operator seed: %v", err)
	}

	accKp, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatalf("creating account keypair: %v", err)
	}

	accPub, err := accKp.PublicKey()
	if err != nil {
		t.Fatalf("getting account public key: %v", err)
	}

	accSeed, err := accKp.Seed()
	if err != nil {
		t.Fatalf("getting account seed: %v", err)
	}

	operatorDir := filepath.Join(tmpDir, "nats", "test-operator")
	accountsDir := filepath.Join(operatorDir, "accounts", "test-account")
	opKeysDir := filepath.Join(tmpDir, "keys", "keys", "O", opPub[1:3])
	accKeysDir := filepath.Join(tmpDir, "keys", "keys", "A", accPub[1:3])

	for _, dir := range []string{operatorDir, accountsDir, opKeysDir, accKeysDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("creating directory %s: %v", dir, err)
		}
	}

	opClaims := natsjwt.NewOperatorClaims(opPub)
	opClaims.Name = "test-operator"
	opJWT, err := opClaims.Encode(opKp)
	if err != nil {
		t.Fatalf("encoding operator JWT: %v", err)
	}

	if err := os.WriteFile(filepath.Join(operatorDir, "test-operator.jwt"), []byte(opJWT), 0644); err != nil {
		t.Fatalf("writing operator JWT: %v", err)
	}

	accClaims := natsjwt.NewAccountClaims(accPub)
	accClaims.Name = "test-account"
	accJWT, err := accClaims.Encode(opKp)
	if err != nil {
		t.Fatalf("encoding account JWT: %v", err)
	}

	if err := os.WriteFile(filepath.Join(accountsDir, "test-account.jwt"), []byte(accJWT), 0644); err != nil {
		t.Fatalf("writing account JWT: %v", err)
	}

	if err := os.WriteFile(filepath.Join(opKeysDir, opPub+".nk"), opSeed, 0600); err != nil {
		t.Fatalf("writing operator seed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(accKeysDir, accPub+".nk"), accSeed, 0600); err != nil {
		t.Fatalf("writing account seed: %v", err)
	}

	return tmpDir
}
