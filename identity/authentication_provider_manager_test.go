package identity

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type recordingAuthProvider struct {
	patterns  []string
	userID    string
	verifyErr error

	called  int
	lastReq AuthRequest
}

func (p *recordingAuthProvider) ManageableAccounts() []string {
	return p.patterns
}

func (p *recordingAuthProvider) Verify(_ context.Context, req AuthRequest) (*User, error) {
	p.called++
	p.lastReq = req
	if p.verifyErr != nil {
		return nil, p.verifyErr
	}
	return &User{ID: p.userID}, nil
}

func TestNewAuthenticationProviderManager_Validation(t *testing.T) {
	t.Run("empty providers", func(t *testing.T) {
		_, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty id", func(t *testing.T) {
		_, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{" ": &recordingAuthProvider{}})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nil provider", func(t *testing.T) {
		_, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{"p1": nil})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAuthenticationProviderManager_SelectProvider_ExplicitAP(t *testing.T) {
	p1 := &recordingAuthProvider{patterns: []string{"ACME"}, userID: "p1"}
	p2 := &recordingAuthProvider{patterns: []string{"ACME"}, userID: "p2"}

	m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
		"p1": p1,
		"p2": p2,
	})
	if err != nil {
		t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
	}

	_, provider, err := m.SelectProvider(AuthRequest{Account: "ACME", Token: "t", AP: "p2"})
	if err != nil {
		t.Fatalf("SelectProvider() error = %v", err)
	}
	user, err := provider.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "t", AP: "p2"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if user.ID != "p2" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "p2")
	}
	if p2.called != 1 {
		t.Fatalf("p2 called = %d, want 1", p2.called)
	}
	if p1.called != 0 {
		t.Fatalf("p1 called = %d, want 0", p1.called)
	}
}

func TestAuthenticationProviderManager_SelectProvider_ExplicitAP_NotFound(t *testing.T) {
	m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
		"p1": &recordingAuthProvider{patterns: []string{"*"}, userID: "p1"},
	})
	if err != nil {
		t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
	}

	_, _, err = m.SelectProvider(AuthRequest{Account: "ACME", Token: "t", AP: "missing"})
	if !errors.Is(err, ErrAuthenticationProviderNotFound) {
		t.Fatalf("SelectProvider() error = %v, want %v", err, ErrAuthenticationProviderNotFound)
	}
}

func TestAuthenticationProviderManager_SelectProvider_ExplicitAP_NotManageable(t *testing.T) {
	m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
		"p1": &recordingAuthProvider{patterns: []string{"ACME"}, userID: "p1"},
	})
	if err != nil {
		t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
	}

	_, _, err = m.SelectProvider(AuthRequest{Account: "OTHER", Token: "t", AP: "p1"})
	if !errors.Is(err, ErrAuthenticationProviderNotManageable) {
		t.Fatalf("SelectProvider() error = %v, want %v", err, ErrAuthenticationProviderNotManageable)
	}
}

func TestAuthenticationProviderManager_SelectProvider_ImplicitSelection(t *testing.T) {
	t.Run("single match", func(t *testing.T) {
		p1 := &recordingAuthProvider{patterns: []string{"AC*"}, userID: "p1"}
		p2 := &recordingAuthProvider{patterns: []string{"ZZ*"}, userID: "p2"}

		m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{"p1": p1, "p2": p2})
		if err != nil {
			t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
		}

		_, provider, err := m.SelectProvider(AuthRequest{Account: "ACME", Token: "t"})
		if err != nil {
			t.Fatalf("SelectProvider() error = %v", err)
		}
		user, err := provider.Verify(context.Background(), AuthRequest{Account: "ACME", Token: "t"})
		if err != nil {
			t.Fatalf("Verify() error = %v", err)
		}
		if user.ID != "p1" {
			t.Fatalf("user.ID = %q, want %q", user.ID, "p1")
		}
		if p1.called != 1 || p2.called != 0 {
			t.Fatalf("called counts = (%d,%d), want (1,0)", p1.called, p2.called)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
			"p1": &recordingAuthProvider{patterns: []string{"ACME"}, userID: "p1"},
		})
		if err != nil {
			t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
		}

		_, _, err = m.SelectProvider(AuthRequest{Account: "OTHER", Token: "t"})
		if !errors.Is(err, ErrAuthenticationProviderNotManageable) {
			t.Fatalf("SelectProvider() error = %v, want %v", err, ErrAuthenticationProviderNotManageable)
		}
	})

	t.Run("ambiguous matches", func(t *testing.T) {
		m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
			"p1": &recordingAuthProvider{patterns: []string{"*"}, userID: "p1"},
			"p2": &recordingAuthProvider{patterns: []string{"A*"}, userID: "p2"},
		})
		if err != nil {
			t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
		}

		_, _, err = m.SelectProvider(AuthRequest{Account: "ACME", Token: "t"})
		if !errors.Is(err, ErrAuthenticationProviderAmbiguous) {
			t.Fatalf("SelectProvider() error = %v, want %v", err, ErrAuthenticationProviderAmbiguous)
		}
		if err != nil && !strings.Contains(err.Error(), "providers match") {
			t.Fatalf("SelectProvider() error = %q, expected ambiguity details", err.Error())
		}
	})
}

func TestAuthenticationProviderManager_ManageableAccountMatching_SYS_AUTH(t *testing.T) {
	m, err := NewAuthenticationProviderManager(map[string]AuthenticationProvider{
		"p1": &recordingAuthProvider{patterns: []string{"*"}, userID: "p1"},
		"p2": &recordingAuthProvider{patterns: []string{"SYS", "AUTH"}, userID: "p2"},
	})
	if err != nil {
		t.Fatalf("NewAuthenticationProviderManager() error = %v", err)
	}

	// SYS should NOT be matched by wildcard patterns.
	_, provider, err := m.SelectProvider(AuthRequest{Account: "SYS", Token: "t", AP: "p2"})
	if err != nil {
		t.Fatalf("SelectProvider() error = %v", err)
	}
	user, err := provider.Verify(context.Background(), AuthRequest{Account: "SYS", Token: "t", AP: "p2"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if user.ID != "p2" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "p2")
	}

	// If AP is not specified, SYS should still not match "*"; only explicit SYS pattern matches.
	_, provider, err = m.SelectProvider(AuthRequest{Account: "SYS", Token: "t"})
	if err != nil {
		t.Fatalf("SelectProvider() error = %v", err)
	}
	user, err = provider.Verify(context.Background(), AuthRequest{Account: "SYS", Token: "t"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if user.ID != "p2" {
		t.Fatalf("user.ID = %q, want %q", user.ID, "p2")
	}
}

func TestMatchAccountPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		account string
		want    bool
	}{
		{name: "exact", pattern: "ACME", account: "ACME", want: true},
		{name: "all wildcard", pattern: "*", account: "ACME", want: true},
		{name: "prefix wildcard matches", pattern: "AC*", account: "ACME", want: true},
		{name: "prefix wildcard no match", pattern: "AC*", account: "ZZZ", want: false},
		{name: "empty pattern", pattern: "", account: "ACME", want: false},
		{name: "empty account", pattern: "*", account: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchAccountPattern(tt.pattern, tt.account); got != tt.want {
				t.Fatalf("matchAccountPattern(%q,%q) = %v, want %v", tt.pattern, tt.account, got, tt.want)
			}
		})
	}
}
