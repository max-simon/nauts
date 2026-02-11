package policy

import (
	"encoding/json"
	"testing"
)

func TestPolicy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{
			name: "valid policy",
			policy: Policy{
				ID:      "test-policy",
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{ActionNATSPub},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing account",
			policy: Policy{
				ID:   "test-policy",
				Name: "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{ActionNATSPub},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing ID",
			policy: Policy{
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{ActionNATSPub},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing statements",
			policy: Policy{
				ID:         "test-policy",
				Account:    "APP",
				Name:       "Test Policy",
				Statements: []Statement{},
			},
			wantErr: true,
		},
		{
			name: "invalid statement effect",
			policy: Policy{
				ID:      "test-policy",
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    Effect("deny"),
						Actions:   []Action{ActionNATSPub},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid action",
			policy: Policy{
				ID:      "test-policy",
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{Action("invalid.action")},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing actions",
			policy: Policy{
				ID:      "test-policy",
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{},
						Resources: []string{"nats:orders"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing resources",
			policy: Policy{
				ID:      "test-policy",
				Account: "APP",
				Name:    "Test Policy",
				Statements: []Statement{
					{
						Effect:    EffectAllow,
						Actions:   []Action{ActionNATSPub},
						Resources: []string{},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.policy.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Policy.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEffect_IsValid(t *testing.T) {
	tests := []struct {
		effect Effect
		want   bool
	}{
		{EffectAllow, true},
		{Effect("deny"), false},
		{Effect(""), false},
		{Effect("allow"), true},
		{Effect("ALLOW"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.effect), func(t *testing.T) {
			if got := tt.effect.IsValid(); got != tt.want {
				t.Errorf("Effect(%q).IsValid() = %v, want %v", tt.effect, got, tt.want)
			}
		})
	}
}

func TestPolicy_JSON(t *testing.T) {
	policy := Policy{
		ID:      "test-policy",
		Account: "APP",
		Name:    "Test Policy",
		Statements: []Statement{
			{
				Effect:    EffectAllow,
				Actions:   []Action{ActionNATSPub, ActionNATSSub},
				Resources: []string{"nats:orders.>"},
			},
		},
	}

	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Policy
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != policy.ID {
		t.Errorf("ID mismatch: got %v, want %v", parsed.ID, policy.ID)
	}
	if parsed.Account != policy.Account {
		t.Errorf("Account mismatch: got %v, want %v", parsed.Account, policy.Account)
	}
	if parsed.Name != policy.Name {
		t.Errorf("Name mismatch: got %v, want %v", parsed.Name, policy.Name)
	}
	if len(parsed.Statements) != 1 {
		t.Fatalf("Expected 1 statement, got %d", len(parsed.Statements))
	}
	if parsed.Statements[0].Effect != EffectAllow {
		t.Errorf("Effect mismatch: got %v, want %v", parsed.Statements[0].Effect, EffectAllow)
	}
	if len(parsed.Statements[0].Actions) != 2 {
		t.Errorf("Actions length mismatch: got %d, want 2", len(parsed.Statements[0].Actions))
	}
}
