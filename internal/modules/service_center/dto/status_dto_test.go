package dto

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseChangeStatusRequest(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		want       ChangeStatusRequest
		wantErrors []StatusFieldError
		wantErr    error
	}{
		{
			name: "valid false status",
			body: `{"isActive":false,"version":7}`,
			want: ChangeStatusRequest{IsActive: false, Version: 7},
		},
		{
			name: "missing fields",
			body: "{}",
			wantErrors: []StatusFieldError{
				{Field: "isActive", Message: "isActive is required"},
				{Field: "version", Message: "Version is required"},
			},
		},
		{
			name: "type and minimum validation",
			body: `{"isActive":"yes","version":0}`,
			wantErrors: []StatusFieldError{
				{Field: "isActive", Message: "must be a boolean"},
				{Field: "version", Message: "must be at least 1"},
			},
		},
		{
			name: "decimal version is not an integer",
			body: `{"isActive":true,"version":1.0}`,
			want: ChangeStatusRequest{IsActive: true},
			wantErrors: []StatusFieldError{
				{Field: "version", Message: "must be an integer"},
			},
		},
		{
			name:       "unknown field",
			body:       `{"isActive":false,"name":"x","version":7}`,
			want:       ChangeStatusRequest{IsActive: false, Version: 7},
			wantErrors: []StatusFieldError{{Field: "name", Message: "unknown field"}},
		},
		{
			name:    "malformed JSON",
			body:    "{isActive",
			wantErr: ErrMalformedStatusRequest,
		},
		{
			name:       "non-object JSON",
			body:       `[1]`,
			wantErrors: []StatusFieldError{{Field: "body", Message: "must be a JSON object"}},
		},
		{
			name: "null values are invalid",
			body: `{"isActive":null,"version":null}`,
			wantErrors: []StatusFieldError{
				{Field: "isActive", Message: "must be a boolean"},
				{Field: "version", Message: "must be an integer"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErrors, err := ParseChangeStatusRequest([]byte(tt.body))
			if !errors.Is(err, tt.wantErr) && (err != nil || tt.wantErr != nil) {
				t.Fatalf("ParseChangeStatusRequest() error = %v, want %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ParseChangeStatusRequest() = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(gotErrors, tt.wantErrors) {
				t.Fatalf("ParseChangeStatusRequest() errors = %#v, want %#v", gotErrors, tt.wantErrors)
			}
		})
	}
}
