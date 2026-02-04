package provider

import (
	"encoding/json"
	"testing"
)

func TestRole_Validate(t *testing.T) {
	tests := []struct {
		name    string
		role    Role
		wantErr bool
	}{
		{
			name: "valid global role",
			role: Role{
				Name:     "test-role",
				Account:  GlobalAccountID,
				Policies: []string{"policy-1"},
			},
			wantErr: false,
		},
		{
			name: "valid local role",
			role: Role{
				Name:     "test-role",
				Account:  "APP",
				Policies: []string{"policy-1"},
			},
			wantErr: false,
		},
		{
			name: "valid role without policies",
			role: Role{
				Name:    "test-role",
				Account: GlobalAccountID,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			role: Role{
				Account:  GlobalAccountID,
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
		{
			name: "missing account",
			role: Role{
				Name:     "test-role",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.role.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Role.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRole_IsGlobal(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want bool
	}{
		{
			name: "global role",
			role: Role{Name: "test", Account: GlobalAccountID},
			want: true,
		},
		{
			name: "local role",
			role: Role{Name: "test", Account: "APP"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.IsGlobal(); got != tt.want {
				t.Errorf("Role.IsGlobal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRole_Key(t *testing.T) {
	tests := []struct {
		name string
		role Role
		want string
	}{
		{
			name: "global role",
			role: Role{Name: "admin", Account: GlobalAccountID},
			want: "admin:*",
		},
		{
			name: "local role",
			role: Role{Name: "admin", Account: "APP"},
			want: "admin:APP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.Key(); got != tt.want {
				t.Errorf("Role.Key() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRole_JSON(t *testing.T) {
	role := Role{
		Name:     "test-role",
		Account:  GlobalAccountID,
		Policies: []string{"policy-1", "policy-2"},
	}

	data, err := json.Marshal(role)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Role
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.Name != role.Name {
		t.Errorf("Name mismatch: got %v, want %v", parsed.Name, role.Name)
	}
	if parsed.Account != role.Account {
		t.Errorf("Account mismatch: got %v, want %v", parsed.Account, role.Account)
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

func TestGlobalAccountID(t *testing.T) {
	if GlobalAccountID != "*" {
		t.Errorf("GlobalAccountID = %v, want %v", GlobalAccountID, "*")
	}
}
