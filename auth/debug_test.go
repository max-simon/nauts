package auth

import (
	"os"
	"strings"
	"testing"
)

func TestNewDebugService_Validation(t *testing.T) {
	tests := []struct {
		name       string
		controller *AuthController
		config     ServerConfig
		wantErr    string
	}{
		{
			name:       "nil controller",
			controller: nil,
			config:     ServerConfig{NatsCredentials: "/path/to/creds"},
			wantErr:    "controller is required",
		},
		{
			name:       "missing authentication",
			controller: &AuthController{},
			config:     ServerConfig{},
			wantErr:    "NATS authentication required",
		},
		{
			name:       "mutually exclusive authentication",
			controller: &AuthController{},
			config: ServerConfig{
				NatsCredentials: "/path/to/creds",
				NatsNkey:        "/path/to/nkey",
			},
			wantErr: "mutually exclusive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewDebugService(tt.controller, tt.config)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestNewDebugService_Defaults(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewDebugService(ctrl, ServerConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewDebugService() error = %v", err)
	}

	if svc.config.NatsURL == "" {
		t.Error("NatsURL should be defaulted, got empty")
	}
}

func TestNewDebugService_EnvForNATSURL(t *testing.T) {
	ctrl := &AuthController{}

	os.Setenv("NATS_URL", "nats://localhost:4000")
	defer os.Unsetenv("NATS_URL")

	svc, err := NewDebugService(ctrl, ServerConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewDebugService() error = %v", err)
	}

	if svc.config.NatsURL != "nats://localhost:4000" {
		t.Errorf("NatsURL = %q, want %q", svc.config.NatsURL, "nats://localhost:4000")
	}
}

func TestNewDebugService_WithNkey(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewDebugService(ctrl, ServerConfig{
		NatsNkey: "/path/to/auth-service.nk",
	})
	if err != nil {
		t.Fatalf("NewDebugService() error = %v", err)
	}

	if svc.config.NatsNkey != "/path/to/auth-service.nk" {
		t.Errorf("NatsNkey = %q, want %q", svc.config.NatsNkey, "/path/to/auth-service.nk")
	}
}

func TestNewDebugService_WithLogger(t *testing.T) {
	ctrl := &AuthController{}
	logger := &testLogger{}

	svc, err := NewDebugService(ctrl, ServerConfig{
		NatsCredentials: "/path/to/creds",
	}, WithDebugLogger(logger))
	if err != nil {
		t.Fatalf("NewDebugService() error = %v", err)
	}

	if svc.logger != logger {
		t.Error("logger was not set correctly")
	}
}

func TestDebugService_Stop(t *testing.T) {
	ctrl := &AuthController{}

	svc, err := NewDebugService(ctrl, ServerConfig{
		NatsCredentials: "/path/to/creds",
	})
	if err != nil {
		t.Fatalf("NewDebugService() error = %v", err)
	}

	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() error = %v", err)
	}
	if err := svc.Stop(); err != nil {
		t.Errorf("Stop() second call error = %v", err)
	}
}
