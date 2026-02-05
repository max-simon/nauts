package provider

import (
	"encoding/json"
	"testing"
)

func TestBinding_Validate(t *testing.T) {
	tests := []struct {
		name    string
		binding binding
		wantErr bool
	}{
		{
			name: "valid binding",
			binding: binding{
				Role:     "test-role",
				Account:  "APP",
				Policies: []string{"policy-1"},
			},
			wantErr: false,
		},
		{
			name: "valid binding without policies",
			binding: binding{
				Role:    "test-role",
				Account: "APP",
			},
			wantErr: false,
		},
		{
			name: "missing role",
			binding: binding{
				Account:  "APP",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
		{
			name: "missing account",
			binding: binding{
				Role:     "test-role",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.binding.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("binding.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBindingKey(t *testing.T) {
	if got := bindingKey("APP", "admin"); got != "APP.admin" {
		t.Errorf("bindingKey() = %v, want %v", got, "APP.admin")
	}
}

func TestBinding_JSON(t *testing.T) {
	b := binding{
		Role:     "test-role",
		Account:  "APP",
		Policies: []string{"policy-1", "policy-2"},
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed binding
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Role != b.Role {
		t.Errorf("Role mismatch: got %v, want %v", parsed.Role, b.Role)
	}
	if parsed.Account != b.Account {
		t.Errorf("Account mismatch: got %v, want %v", parsed.Account, b.Account)
	}
	if len(parsed.Policies) != 2 {
		t.Errorf("Policies length mismatch: got %d, want 2", len(parsed.Policies))
	}
}

func TestDefaultRoleName(t *testing.T) {
	if DefaultRoleName != "default" {
		t.Errorf("DefaultRoleName = %v, want %v", DefaultRoleName, "default")
	}
}
