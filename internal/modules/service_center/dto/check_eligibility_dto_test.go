package dto

import (
	"strings"
	"testing"
)

func TestParseCheckEligibilityRequest(t *testing.T) {
	t.Run("parses a complete request and a null brand", func(t *testing.T) {
		request, validationErrors, err := ParseCheckEligibilityRequest([]byte(`{
			"serviceCenterId":"1924881519926425600",
			"serviceType":"Routine",
			"assets":[
				{"groupId":"1924881519926425702","categoryId":"1924881519926425703","brandId":null}
			]
		}`))
		if err != nil {
			t.Fatalf("ParseCheckEligibilityRequest() error = %v", err)
		}
		if len(validationErrors) != 0 {
			t.Fatalf("ParseCheckEligibilityRequest() validation errors = %#v", validationErrors)
		}
		if request.ServiceCenterID != 1924881519926425600 || request.ServiceType != "Routine" {
			t.Fatalf("ParseCheckEligibilityRequest() request = %#v", request)
		}
		if len(request.Assets) != 1 || request.Assets[0].GroupID != 1924881519926425702 || request.Assets[0].CategoryID != 1924881519926425703 {
			t.Fatalf("ParseCheckEligibilityRequest() assets = %#v", request.Assets)
		}
		if request.Assets[0].BrandID != nil {
			t.Fatalf("null brandId = %v, want nil", *request.Assets[0].BrandID)
		}
	})

	t.Run("parses a string brand id", func(t *testing.T) {
		request, validationErrors, err := ParseCheckEligibilityRequest([]byte(`{
			"serviceCenterId":"1",
			"serviceType":"Repair",
			"assets":[{"groupId":"2","categoryId":"3","brandId":"4"}]
		}`))
		if err != nil || len(validationErrors) != 0 {
			t.Fatalf("ParseCheckEligibilityRequest() = (%#v, %#v, %v)", request, validationErrors, err)
		}
		if request.Assets[0].BrandID == nil || *request.Assets[0].BrandID != 4 {
			t.Fatalf("brandId = %v, want 4", request.Assets[0].BrandID)
		}
	})

	t.Run("separates malformed JSON from validation errors", func(t *testing.T) {
		_, validationErrors, err := ParseCheckEligibilityRequest([]byte(`{"serviceCenterId":`))
		if err == nil || len(validationErrors) != 0 {
			t.Fatalf("malformed JSON returned (%#v, %v), want an error and no validation errors", validationErrors, err)
		}

		_, validationErrors, err = ParseCheckEligibilityRequest([]byte(`{"serviceCenterId":1}`))
		if err != nil {
			t.Fatalf("valid JSON returned parse error: %v", err)
		}
		if len(validationErrors) < 2 {
			t.Fatalf("validation errors = %#v, want service center ID and required fields", validationErrors)
		}
	})

	t.Run("reports invalid nested ids and requires the nullable brand field", func(t *testing.T) {
		_, validationErrors, err := ParseCheckEligibilityRequest([]byte(`{
			"serviceCenterId":"1",
			"serviceType":"Routine",
			"assets":[{"groupId":2,"categoryId":"3"}]
		}`))
		if err != nil {
			t.Fatalf("ParseCheckEligibilityRequest() error = %v", err)
		}
		fields := make([]string, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			fields = append(fields, validationError.Field)
		}
		joined := strings.Join(fields, ",")
		for _, want := range []string{"assets[0].groupId", "assets[0].brandId"} {
			if !strings.Contains(joined, want) {
				t.Errorf("validation fields = %q, missing %q", joined, want)
			}
		}
	})

	t.Run("rejects invalid service type and non-decimal IDs", func(t *testing.T) {
		_, validationErrors, err := ParseCheckEligibilityRequest([]byte(`{
			"serviceCenterId":"+1",
			"serviceType":"routine",
			"assets":[{"groupId":"2","categoryId":"3","brandId":null}]
		}`))
		if err != nil {
			t.Fatalf("ParseCheckEligibilityRequest() error = %v", err)
		}
		if len(validationErrors) != 2 {
			t.Fatalf("validation errors = %#v, want two errors", validationErrors)
		}
	})
}
