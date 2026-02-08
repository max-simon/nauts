package policy

import (
	"errors"
	"testing"
)

func TestParseResource(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		wantResourceType ResourceType
		wantID           string
		wantSubID        string
		wantErr          bool
		wantErrSentinel  error
	}{
		// Valid NATS NRNs
		{
			name:             "nats simple subject",
			input:            "nats:orders",
			wantResourceType: ResourceTypeNATS,
			wantID:           "orders",
		},
		{
			name:             "nats dotted subject",
			input:            "nats:orders.created",
			wantResourceType: ResourceTypeNATS,
			wantID:           "orders.created",
		},
		{
			name:             "nats with queue",
			input:            "nats:orders:workers",
			wantResourceType: ResourceTypeNATS,
			wantID:           "orders",
			wantSubID:        "workers",
		},
		{
			name:             "nats with star wildcard",
			input:            "nats:orders.*",
			wantResourceType: ResourceTypeNATS,
			wantID:           "orders.*",
		},
		{
			name:             "nats with gt wildcard",
			input:            "nats:orders.>",
			wantResourceType: ResourceTypeNATS,
			wantID:           "orders.>",
		},

		// Valid JS NRNs
		{
			name:             "js stream only",
			input:            "js:ORDERS",
			wantResourceType: ResourceTypeJS,
			wantID:           "ORDERS",
		},
		{
			name:             "js stream with consumer",
			input:            "js:ORDERS:processor",
			wantResourceType: ResourceTypeJS,
			wantID:           "ORDERS",
			wantSubID:        "processor",
		},
		{
			name:             "js star stream and consumer",
			input:            "js:*:*",
			wantResourceType: ResourceTypeJS,
			wantID:           "*",
			wantSubID:        "*",
		},

		// Valid KV NRNs
		{
			name:             "kv bucket only",
			input:            "kv:config",
			wantResourceType: ResourceTypeKV,
			wantID:           "config",
		},
		{
			name:             "kv bucket with key",
			input:            "kv:config:app.settings",
			wantResourceType: ResourceTypeKV,
			wantID:           "config",
			wantSubID:        "app.settings",
		},
		{
			name:             "kv bucket with gt key",
			input:            "kv:config:app.>",
			wantResourceType: ResourceTypeKV,
			wantID:           "config",
			wantSubID:        "app.>",
		},

		// Template variables
		{
			name:             "nats with template variable",
			input:            "nats:user.{{ user.id }}",
			wantResourceType: ResourceTypeNATS,
			wantID:           "user.{{ user.id }}",
		},
		{
			name:             "kv with template in key",
			input:            "kv:users:{{ user.id }}.settings",
			wantResourceType: ResourceTypeKV,
			wantID:           "users",
			wantSubID:        "{{ user.id }}.settings",
		},

		// Invalid NRNs
		{
			name:            "empty string",
			input:           "",
			wantErr:         true,
			wantErrSentinel: ErrInvalidResource,
		},
		{
			name:            "no colon",
			input:           "nats",
			wantErr:         true,
			wantErrSentinel: ErrInvalidResource,
		},
		{
			name:            "unknown type",
			input:           "unknown:foo",
			wantErr:         true,
			wantErrSentinel: ErrUnknownResourceType,
		},
		{
			name:            "empty identifier",
			input:           "nats:",
			wantErr:         true,
			wantErrSentinel: ErrInvalidResource,
		},
		{
			name:            "empty sub-identifier",
			input:           "nats:foo:",
			wantErr:         true,
			wantErrSentinel: ErrInvalidResource,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseResource(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseResource(%q) expected error, got nil", tt.input)
					return
				}
				if tt.wantErrSentinel != nil {
					var nrnErr *PolicyError
					if !errors.As(err, &nrnErr) {
						t.Errorf("ParseResource(%q) error not ResourceError: %v", tt.input, err)
						return
					}
					if !errors.Is(nrnErr.Err, tt.wantErrSentinel) {
						t.Errorf("ParseResource(%q) error = %v, want sentinel %v", tt.input, err, tt.wantErrSentinel)
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ParseResource(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got.Type != tt.wantResourceType {
				t.Errorf("ParseResource(%q).ResourceType = %v, want %v", tt.input, got.Type, tt.wantResourceType)
			}
			if got.Identifier != tt.wantID {
				t.Errorf("ParseResource(%q).Identifier = %v, want %v", tt.input, got.Identifier, tt.wantID)
			}
			if got.SubIdentifier != tt.wantSubID {
				t.Errorf("ParseResource(%q).SubIdentifier = %v, want %v", tt.input, got.SubIdentifier, tt.wantSubID)
			}
			if got.Raw != tt.input {
				t.Errorf("ParseResource(%q).Raw = %v, want %v", tt.input, got.Raw, tt.input)
			}
		})
	}
}

func TestResource_String(t *testing.T) {
	tests := []struct {
		name string
		nrn  *Resource
		want string
	}{
		{
			name: "without sub-identifier",
			nrn:  &Resource{Type: ResourceTypeNATS, Identifier: "orders"},
			want: "nats:orders",
		},
		{
			name: "with sub-identifier",
			nrn:  &Resource{Type: ResourceTypeNATS, Identifier: "orders", SubIdentifier: "workers"},
			want: "nats:orders:workers",
		},
		{
			name: "js type",
			nrn:  &Resource{Type: ResourceTypeJS, Identifier: "ORDERS", SubIdentifier: "processor"},
			want: "js:ORDERS:processor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nrn.String(); got != tt.want {
				t.Errorf("Resource.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_HasSubIdentifier(t *testing.T) {
	tests := []struct {
		name string
		nrn  *Resource
		want bool
	}{
		{
			name: "without sub-identifier",
			nrn:  &Resource{Type: ResourceTypeNATS, Identifier: "orders"},
			want: false,
		},
		{
			name: "with sub-identifier",
			nrn:  &Resource{Type: ResourceTypeNATS, Identifier: "orders", SubIdentifier: "workers"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.nrn.HasSubIdentifier(); got != tt.want {
				t.Errorf("Resource.HasSubIdentifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceType_IsValid(t *testing.T) {
	tests := []struct {
		t    ResourceType
		want bool
	}{
		{ResourceTypeNATS, true},
		{ResourceTypeJS, true},
		{ResourceTypeKV, true},
		{ResourceType("unknown"), false},
		{ResourceType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.t), func(t *testing.T) {
			if got := tt.t.IsValid(); got != tt.want {
				t.Errorf("ResourceType(%q).IsValid() = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

func TestMustParseResource(t *testing.T) {
	// Test valid NRN
	nrn := MustParseResource("nats:orders")
	if nrn.Type != ResourceTypeNATS || nrn.Identifier != "orders" {
		t.Errorf("MustParse failed for valid NRN")
	}

	// Test panic on invalid NRN
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("MustParse did not panic on invalid NRN")
		}
	}()
	MustParseResource("invalid")
}

func TestResource_FullType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  ResourceType
	}{
		// NATS
		{"nats subject only", "nats:orders", ResourceTypeNATSSubject},
		{"nats with queue", "nats:orders:workers", ResourceTypeNATSSubjectQueue},

		// JetStream
		{"js stream only", "js:ORDERS", ResourceTypeJSStream},
		{"js stream with consumer", "js:ORDERS:processor", ResourceTypeJSStreamConsumer},

		// KV
		{"kv bucket only", "kv:config", ResourceTypeKVBucket},
		{"kv bucket with key", "kv:config:app.settings", ResourceTypeKVBucketEntry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nrn, err := ParseResource(tt.input)
			if err != nil {
				t.Fatalf("ParseResource(%q) failed: %v", tt.input, err)
			}

			if got := nrn.FullType(); got != tt.want {
				t.Errorf("Resource.FullType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResource_TypeHelpers(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		isSubject bool
		isStream  bool
		isBucket  bool
	}{
		{"nats", "nats:orders", true, false, false},
		{"js", "js:ORDERS", false, true, false},
		{"kv", "kv:config", false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nrn, _ := ParseResource(tt.input)

			if got := nrn.IsSubject(); got != tt.isSubject {
				t.Errorf("IsSubject() = %v, want %v", got, tt.isSubject)
			}
			if got := nrn.IsStream(); got != tt.isStream {
				t.Errorf("IsStream() = %v, want %v", got, tt.isStream)
			}
			if got := nrn.IsBucket(); got != tt.isBucket {
				t.Errorf("IsBucket() = %v, want %v", got, tt.isBucket)
			}
		})
	}
}
