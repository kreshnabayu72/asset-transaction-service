package dto

import (
	"encoding/json"
	"testing"
)

func TestListRequestValidationAndVisibleColumns(t *testing.T) {
	request := ListServiceCentersRequest{
		SortBy:    "name",
		SortOrder: 1,
		Page:      1,
		Limit:     25,
		Columns:   []string{"name", "usage", "futureColumn"},
	}
	if got := request.Validate(); len(got) != 0 {
		t.Fatalf("valid request errors = %#v", got)
	}
	if got := request.VisibleColumns(); len(got) != 2 || got[0] != "name" || got[1] != "usage" {
		t.Fatalf("visible columns = %#v", got)
	}
}

func TestReadFilterValidationRejectsInvalidEnumsRangesAndIDs(t *testing.T) {
	min, max := 4, 2
	request := ServiceCenterReadFilters{
		SortBy:         "bogus",
		SortOrder:      0,
		ServiceType:    []string{"Repair-ish"},
		Status:         []string{"Paused"},
		Specialization: []string{"-5"},
		UpdatedAt:      []int64{20, 10},
		UsageMin:       &min,
		UsageMax:       &max,
	}
	if errors := request.ValidateFilters(); len(errors) < 7 {
		t.Fatalf("expected validation errors, got %#v", errors)
	}
}

func TestOptionsRequestedFacetsValidatesBooleanValues(t *testing.T) {
	request := OptionsServiceCentersRequest{
		"cityOptions":  json.RawMessage("true"),
		"brandOptions": json.RawMessage([]byte{34, 121, 101, 115, 34}),
	}
	facets, errors := request.RequestedFacets()
	if len(errors) != 1 || errors[0].Field != "brandOptions" {
		t.Fatalf("validation errors = %#v", errors)
	}
	if !facets["cityOptions"] || facets["brandOptions"] {
		t.Fatalf("requested facets = %#v", facets)
	}
}

func TestUnknownReadRequestFieldIsRejected(t *testing.T) {
	errors := UnknownReadRequestFields([]byte(`{"search":"x","unexpected":true}`), "search")
	if len(errors) != 1 || errors[0].Field != "unexpected" {
		t.Fatalf("unknown field errors = %#v", errors)
	}
}
