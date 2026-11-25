package dto

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOptionalStringTracksOmittedNullAndValue(t *testing.T) {
	tests := []struct {
		name string
		body string
		set  bool
		want *string
	}{
		{name: "omitted", body: `{}`, set: false},
		{name: "null", body: `{"attachmentFileId":null}`, set: true},
		{name: "value", body: `{"attachmentFileId":"staged-file"}`, set: true, want: stringPointer("staged-file")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request ServiceCenterWriteRequest
			if err := json.Unmarshal([]byte(test.body), &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			if request.AttachmentFileID.Set != test.set {
				t.Fatalf("Set = %v, want %v", request.AttachmentFileID.Set, test.set)
			}
			if request.AttachmentFileID.Value == nil || test.want == nil {
				if request.AttachmentFileID.Value != nil || test.want != nil {
					t.Fatalf("Value = %v, want %v", request.AttachmentFileID.Value, test.want)
				}
				return
			}
			if *request.AttachmentFileID.Value != *test.want {
				t.Fatalf("Value = %q, want %q", *request.AttachmentFileID.Value, *test.want)
			}
		})
	}
}

func TestWriteRequestValidationAcceptsMinimalValidBody(t *testing.T) {
	request := ServiceCenterWriteRequest{
		Name:         "Local Center",
		ServiceTypes: []string{"Repair"},
		AvailableTo:  []string{"1924881519926425700"},
		Country:      "Indonesia",
		City:         "Jakarta",
		CompanyPhone: &ServiceCenterPhone{DialCode: "+62", Number: "81234567"},
	}
	if errs := request.Validate(); len(errs) != 0 {
		t.Fatalf("Validate() errors = %#v, want none", errs)
	}
}

func TestUpdateRequestValidationDistinguishesMissingAndZeroVersion(t *testing.T) {
	base := `{"name":"Local Center","serviceTypes":["Repair"],"availableTo":["1924881519926425700"],"country":"Indonesia","city":"Jakarta","companyPhone":{"dialCode":"+62","number":"81234567"}}`
	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{name: "missing", body: base, wantMessage: "version is required"},
		{name: "zero", body: base[:len(base)-1] + `,"version":0}`, wantMessage: "must be greater than or equal to 1"},
		{name: "valid", body: base[:len(base)-1] + `,"version":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var request UpdateServiceCenterRequest
			if err := json.Unmarshal([]byte(test.body), &request); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			errs := request.Validate()
			if test.wantMessage != "" && !hasCRUDFieldError(errs, "version", test.wantMessage) {
				t.Fatalf("Validate() errors = %#v, want version error %q", errs, test.wantMessage)
			}
			if test.wantMessage == "" && hasCRUDFieldError(errs, "version", "version is required") {
				t.Fatalf("Validate() errors = %#v, unexpected missing-version error", errs)
			}
		})
	}
}

func TestWriteRequestValidationUsesUnicodeCharacterCountAndRejectsEmoji(t *testing.T) {
	request := ServiceCenterWriteRequest{
		Name:         strings.Repeat("𠮷", 120),
		ServiceTypes: []string{"Routine"},
		AvailableTo:  []string{"1"},
		Country:      "Indonesia",
		City:         "Jakarta",
		CompanyEmail: stringPointer("help@example.com"),
	}
	if errs := request.Validate(); len(errs) != 0 {
		t.Fatalf("120 Unicode characters rejected: %#v", errs)
	}
	request.Name = strings.Repeat("𠮷", 121)
	errs := request.Validate()
	if !hasCRUDFieldError(errs, "name", "Max. 120 characters") {
		t.Fatalf("121 Unicode characters errors = %#v", errs)
	}
	request.Name = "Service Center 😀"
	errs = request.Validate()
	if !hasCRUDFieldError(errs, "name", "Name contains invalid characters") {
		t.Fatalf("emoji name errors = %#v", errs)
	}
}

func TestWriteRequestDoesNotInventCountryCatalog(t *testing.T) {
	request := ServiceCenterWriteRequest{
		Name:         "Local Center",
		ServiceTypes: []string{"Routine"},
		AvailableTo:  []string{"1"},
		Country:      "Atlantis",
		City:         "Jakarta",
		CompanyEmail: stringPointer("help@example.com"),
	}
	if errs := request.Validate(); len(errs) != 0 {
		t.Fatalf("nonempty country was rejected without a verified catalog: %#v", errs)
	}
}

func stringPointer(value string) *string {
	return &value
}

func hasCRUDFieldError(errs []CRUDFieldError, field, message string) bool {
	for _, err := range errs {
		if err.Field == field && err.Message == message {
			return true
		}
	}
	return false
}
