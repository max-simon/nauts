package auth

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nkeys"
)

func TestNewCalloutService_Validation(t *testing.T) {
	tests := []struct {
		name       string
		controller *AuthController
		config     CalloutConfig
		wantErr    string
	}{
		{
			name:       "nil controller",
			controller: nil,
			config:     CalloutConfig{NatsCredentials: "/path/to/creds"},
			wantErr:    "controller is required",
		},
		{
			name:       "missing authentication",
			controller: &AuthController{},
			config:     CalloutConfig{},
			wantErr:    "NATS authentication required",
		},
		{
			name:       "mutually exclusive authentication",
			controller: &AuthController{},
			config: CalloutConfig{
				NatsCredentials: "/path/to/creds",
				NatsNkey:        "/path/to/nkey",
			},
			wantErr: "mutually exclusive",
		},
		{
			name:       "invalid xkey seed",
			controller: &AuthController{},
			config: CalloutConfig{
				NatsCredentials: "/path/to/creds",
				XKeySeed:        "invalid-seed",
			},
			wantErr: "parsing xkey seed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCalloutService(tt.controller, tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewCalloutService_Defaults(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	// Check defaults
	if svc.config.NatsURL == "" {
		t.Error("NatsURL should be defaulted, got empty")
	}
	if svc.config.DefaultTTL != time.Hour {
		t.Errorf("DefaultTTL = %v, want 1h", svc.config.DefaultTTL)
	}
}

func TestNewCalloutService_EnvForNATSURL(t *testing.T) {
	ctrl := &AuthController{}

	// mock environment variable NATS_URL
	os.Setenv("NATS_URL", "nats://localhost:4000")
	defer os.Unsetenv("NATS_URL")

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	// Check defaults
	if svc.config.NatsURL != "nats://localhost:4000" {
		t.Errorf("NatsURL = %q, want %q", svc.config.NatsURL, "nats://localhost:4000")
	}
}

func TestNewCalloutService_WithNkey(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsNkey: "/path/to/auth-service.nk",
	})
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	if svc.config.NatsNkey != "/path/to/auth-service.nk" {
		t.Errorf("NatsNkey = %q, want %q", svc.config.NatsNkey, "/path/to/auth-service.nk")
	}
}

func TestNewCalloutService_WithXKey(t *testing.T) {
	ctrl := &AuthController{}

	// Generate a valid curve keypair
	kp, err := nkeys.CreateCurveKeys()
	if err != nil {
		t.Fatalf("creating curve keypair: %v", err)
	}
	seed, err := kp.Seed()
	if err != nil {
		t.Fatalf("getting seed: %v", err)
	}

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsCredentials: "/path/to/creds",
		XKeySeed:        string(seed),
	})
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	if svc.curveKeyPair == nil {
		t.Error("curveKeyPair should be set")
	}
}

func TestNewCalloutService_WithLogger(t *testing.T) {
	ctrl := &AuthController{}
	logger := &testLogger{}

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsCredentials: "/path/to/creds",
	}, WithCalloutLogger(logger))
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	if svc.logger != logger {
		t.Error("logger was not set correctly")
	}
}

func TestCalloutService_Stop(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewCalloutService(ctrl, CalloutConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	// First stop should succeed
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Second stop should be idempotent
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() second call error = %v", err)
	}
}

func TestCalloutConfig_Validation(t *testing.T) {
	// Test that empty NatsURL gets defaulted
	config := CalloutConfig{
		NatsCredentials: "/path/to/creds",
	}

	ctrl := &AuthController{}
	svc, err := NewCalloutService(ctrl, config)
	if err != nil {
		t.Fatalf("NewCalloutService() error = %v", err)
	}

	if svc.config.NatsURL == "" {
		t.Error("NatsURL should be defaulted")
	}
}

func TestXKeyEncryptDecrypt(t *testing.T) {
	// Test that xkey encryption/decryption works
	// Generate two keypairs (service and "server")
	serviceKp, err := nkeys.CreateCurveKeys()
	if err != nil {
		t.Fatalf("creating service keypair: %v", err)
	}
	serverKp, err := nkeys.CreateCurveKeys()
	if err != nil {
		t.Fatalf("creating server keypair: %v", err)
	}

	servicePub, _ := serviceKp.PublicKey()
	serverPub, _ := serverKp.PublicKey()

	// Encrypt with server keypair, decrypt with service keypair
	plaintext := []byte("test message")
	encrypted, err := serverKp.Seal(plaintext, servicePub)
	if err != nil {
		t.Fatalf("encrypting: %v", err)
	}

	decrypted, err := serviceKp.Open(encrypted, serverPub)
	if err != nil {
		t.Fatalf("decrypting: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}
