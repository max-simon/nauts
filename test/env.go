package test

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/nats-io/nats.go"
)

type TestEnv struct {
	t         *testing.T
	mode      string
	baseDir   string
	natsCmd   *exec.Cmd
	nautsCmd  *exec.Cmd
	port      int
	credsFile string // sentinel creds for operator mode
}

func newTestEnv(t *testing.T, dir string, mode string, port int) *TestEnv {
	t.Helper()

	baseDir := filepath.Join(".", dir)
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		t.Fatalf("test directory %s does not exist", baseDir)
	}

	env := &TestEnv{
		t:       t,
		mode:    mode,
		baseDir: baseDir,
		port:    port,
	}

	if mode == "operator" {
		env.credsFile = filepath.Join(baseDir, "sentinel.creds")
	}

	return env
}

func (e *TestEnv) start() {
	e.t.Helper()

	// Start NATS server
	e.t.Log("Starting NATS server...")
	e.natsCmd = exec.Command("nats-server", "-c", "nats-server.conf", "-p", fmt.Sprintf("%d", e.port))
	e.natsCmd.Dir = e.baseDir
	if err := e.natsCmd.Start(); err != nil {
		e.t.Fatalf("Failed to start NATS server: %v", err)
	}

	// Wait for NATS to be ready
	time.Sleep(time.Second)

	// Start nauts auth service
	e.t.Log("Starting nauts auth service...")
	// environment variable NATS_URL
	e.nautsCmd = exec.Command("../../bin/nauts", "serve", "-c", "nauts.json")
	e.nautsCmd.Env = []string{fmt.Sprintf("NATS_URL=nats://localhost:%d", e.port)}
	e.nautsCmd.Dir = e.baseDir
	e.nautsCmd.Stdout = os.Stdout
	e.nautsCmd.Stderr = os.Stderr
	if err := e.nautsCmd.Start(); err != nil {
		e.stopNats()
		e.t.Fatalf("Failed to start nauts: %v", err)
	}

	// Wait for nauts to be ready
	time.Sleep(time.Second)
}

func (e *TestEnv) stop() {
	e.t.Helper()

	if e.nautsCmd != nil && e.nautsCmd.Process != nil {
		e.nautsCmd.Process.Kill()
		e.nautsCmd.Wait()
	}
	e.stopNats()

	// Wait for cleanup
	time.Sleep(500 * time.Millisecond)
}

func (e *TestEnv) stopNats() {
	if e.natsCmd != nil && e.natsCmd.Process != nil {
		e.natsCmd.Process.Kill()
		e.natsCmd.Wait()
	}
}

func (e *TestEnv) ConnectWithUsernameAndPassword(username string, password string, account string, providerID string) (*nats.Conn, error) {
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
	return nats.Connect(fmt.Sprintf("nats://localhost:%d", e.port), opts...)
}

func (e *TestEnv) ConnectWithJwt(token string, account string, providerID string) (*nats.Conn, error) {
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
	return nats.Connect(fmt.Sprintf("nats://localhost:%d", e.port), opts...)
}

func (e *TestEnv) GenerateJWT(t *testing.T, roles []string, sub string) string {
	keyPath := filepath.Join("common", "rsa.key")
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
