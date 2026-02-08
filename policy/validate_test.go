package policy

import (
	"errors"
	"testing"
)

func TestValidateResource(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		// Valid NATS NRNs
		{"nats simple", "nats:orders", false},
		{"nats with star", "nats:orders.*", false},
		{"nats with gt", "nats:orders.>", false},
		{"nats star and gt", "nats:*.>", false},
		{"nats queue with star", "nats:orders:worker-*", false},
		{"nats complex subject", "nats:orders.*.created.>", false},

		// Invalid NATS NRNs
		{"nats queue with gt", "nats:orders:workers.>", true},
		{"nats gt in middle", "nats:orders.>.foo", true},

		// Valid JS NRNs
		{"js stream only", "js:ORDERS", false},
		{"js star stream", "js:*", false},
		{"js stream consumer", "js:ORDERS:processor", false},
		{"js star consumer", "js:ORDERS:*", false},
		{"js star both", "js:*:*", false},

		// Invalid JS NRNs
		{"js stream with gt", "js:ORDERS.>", true},
		{"js consumer with gt", "js:ORDERS:processor.>", true},

		// Valid KV NRNs
		{"kv bucket only", "kv:config", false},
		{"kv star bucket", "kv:*", false},
		{"kv bucket key", "kv:config:app.settings", false},
		{"kv key with gt", "kv:config:app.>", false},
		{"kv key with star", "kv:config:app.*", false},
		{"kv key star and gt", "kv:config:*.>", false},

		// Invalid KV NRNs
		{"kv bucket with gt", "kv:config.>", true},

		// Template variables - should pass validation (validated after interpolation)
		{"nats template", "nats:user.{{ user.id }}", false},
		{"js template", "js:{{ stream.name }}", false},
		{"kv template", "kv:{{ bucket }}:{{ key }}", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nrn, err := ParseResource(tt.input)
			if err != nil {
				t.Fatalf("ParseResource(%q) failed: %v", tt.input, err)
			}

			err = ValidateResource(nrn)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResource(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseAndValidateResource(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		wantErr         bool
		wantErrSentinel error
	}{
		// Valid
		{"nats simple", "nats:orders", false, nil},
		{"nats with wildcards", "nats:orders.>", false, nil},
		{"js with consumer", "js:ORDERS:processor", false, nil},
		{"kv with key", "kv:config:app.settings", false, nil},

		// Invalid parsing
		{"empty", "", true, ErrInvalidResource},
		{"unknown type", "foo:bar", true, ErrUnknownResourceType},

		// Invalid wildcards
		{"nats queue gt", "nats:orders:workers.>", true, ErrInvalidWildcard},
		{"js stream gt", "js:ORDERS.>", true, ErrInvalidWildcard},
		{"kv bucket gt", "kv:config.>", true, ErrInvalidWildcard},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseAndValidateResource(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseAndValidateResource(%q) expected error, got nil", tt.input)
					return
				}
				if tt.wantErrSentinel != nil {
					var nrnErr *PolicyError
					if errors.As(err, &nrnErr) {
						if !errors.Is(nrnErr.Err, tt.wantErrSentinel) {
							t.Errorf("ParseAndValidateResource(%q) error sentinel = %v, want %v", tt.input, nrnErr.Err, tt.wantErrSentinel)
						}
					}
				}
				return
			}

			if err != nil {
				t.Errorf("ParseAndValidateResource(%q) unexpected error: %v", tt.input, err)
			}
		})
	}
}

func TestValidateGTPlacement(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		// Valid
		{">", false},
		{"foo.>", false},
		{"foo.bar.>", false},
		{"*.>", false},

		// Invalid
		{">.foo", true},
		{"foo.>.bar", true},
		{"fo>o", true},
		{"foo>.bar", true},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateGTPlacement(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGTPlacement(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}

func TestHasWildcard(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"no wildcard", "nats:orders", false},
		{"star in identifier", "nats:orders.*", true},
		{"gt in identifier", "nats:orders.>", true},
		{"star in sub-identifier", "nats:orders:worker-*", true},
		{"wildcard in both", "nats:*.>:*", true},
		{"js star", "js:*", true},
		{"js no wildcard", "js:ORDERS:processor", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nrn, err := ParseResource(tt.input)
			if err != nil {
				t.Fatalf("ParseResource(%q) failed: %v", tt.input, err)
			}

			if got := HasWildcard(nrn); got != tt.want {
				t.Errorf("HasWildcard(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
