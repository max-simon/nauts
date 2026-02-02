package model

import (
	"encoding/json"
	"testing"
)

func TestGroup_Validate(t *testing.T) {
	tests := []struct {
		name    string
		group   Group
		wantErr bool
	}{
		{
			name: "valid group",
			group: Group{
				ID:       "test-group",
				Name:     "Test Group",
				Policies: []string{"policy-1"},
			},
			wantErr: false,
		},
		{
			name: "valid group without policies",
			group: Group{
				ID:   "test-group",
				Name: "Test Group",
			},
			wantErr: false,
		},
		{
			name: "missing ID",
			group: Group{
				Name:     "Test Group",
				Policies: []string{"policy-1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroup_JSON(t *testing.T) {
	group := Group{
		ID:       "test-group",
		Name:     "Test Group",
		Policies: []string{"policy-1", "policy-2"},
	}

	data, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var parsed Group
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if parsed.ID != group.ID {
		t.Errorf("ID mismatch: got %v, want %v", parsed.ID, group.ID)
	}
	if parsed.Name != group.Name {
		t.Errorf("Name mismatch: got %v, want %v", parsed.Name, group.Name)
	}
	if len(parsed.Policies) != 2 {
		t.Errorf("Policies length mismatch: got %d, want 2", len(parsed.Policies))
	}
}

func TestDefaultGroupID(t *testing.T) {
	if DefaultGroupID != "default" {
		t.Errorf("DefaultGroupID = %v, want %v", DefaultGroupID, "default")
	}
}

func TestUser_GetAttribute(t *testing.T) {
	tests := []struct {
		name string
		user *User
		key  string
		want string
	}{
		{
			name: "existing attribute",
			user: &User{Attributes: map[string]string{"dept": "eng"}},
			key:  "dept",
			want: "eng",
		},
		{
			name: "missing attribute",
			user: &User{Attributes: map[string]string{"dept": "eng"}},
			key:  "team",
			want: "",
		},
		{
			name: "nil attributes",
			user: &User{},
			key:  "dept",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.user.GetAttribute(tt.key); got != tt.want {
				t.Errorf("GetAttribute(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}
