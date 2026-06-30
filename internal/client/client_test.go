package client

import (
	"errors"
	"testing"

	"github.com/ArfaMujahid/invoice-generator/internal/apperr"
)

// TestClientValidate checks the required-field and email-format rules (FR-1.6)
// across valid and invalid inputs, asserting which fields are reported.
func TestClientValidate(t *testing.T) {
	tests := []struct {
		name       string
		client     Client
		wantValid  bool
		wantFields []string // field keys expected in the ValidationError
	}{
		{
			name:      "valid minimal",
			client:    Client{Name: "Acme Co", Email: "billing@acme.example"},
			wantValid: true,
		},
		{
			name:       "missing name",
			client:     Client{Email: "billing@acme.example"},
			wantValid:  false,
			wantFields: []string{"name"},
		},
		{
			name:       "missing email",
			client:     Client{Name: "Acme Co"},
			wantValid:  false,
			wantFields: []string{"email"},
		},
		{
			name:       "malformed email",
			client:     Client{Name: "Acme Co", Email: "not-an-email"},
			wantValid:  false,
			wantFields: []string{"email"},
		},
		{
			name:       "all missing",
			client:     Client{},
			wantValid:  false,
			wantFields: []string{"name", "email"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.Validate()
			if tt.wantValid {
				if err != nil {
					t.Fatalf("Validate() = %v; want nil", err)
				}
				return
			}

			var verr *apperr.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("Validate() = %v; want *apperr.ValidationError", err)
			}
			for _, f := range tt.wantFields {
				if _, ok := verr.Fields[f]; !ok {
					t.Errorf("missing validation error for field %q; got fields %v", f, verr.Fields)
				}
			}
			if len(verr.Fields) != len(tt.wantFields) {
				t.Errorf("got %d field errors %v; want %d (%v)",
					len(verr.Fields), verr.Fields, len(tt.wantFields), tt.wantFields)
			}
		})
	}
}
