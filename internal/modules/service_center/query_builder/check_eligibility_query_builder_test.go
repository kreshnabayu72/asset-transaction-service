package query_builder

import (
	"strings"
	"testing"
)

func TestBuildCheckEligibilityQueries(t *testing.T) {
	tests := []struct {
		name  string
		build func(int64) (string, []any, error)
		table string
		field string
	}{
		{name: "service center", build: BuildCheckEligibilityCenterQuery, table: "service_centers", field: "service_types"},
		{name: "groups", build: BuildCheckEligibilityGroupIDsQuery, table: "service_center_groups", field: "group_id"},
		{name: "categories", build: BuildCheckEligibilityCategoryIDsQuery, table: "service_center_categories", field: "category_id"},
		{name: "brands", build: BuildCheckEligibilityBrandIDsQuery, table: "service_center_brands", field: "brand_id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, args, err := tt.build(42)
			if err != nil {
				t.Fatalf("build() error = %v", err)
			}
			if !strings.Contains(query, tt.table) || !strings.Contains(query, tt.field) || !strings.Contains(query, "service_center_id") && tt.table != "service_centers" {
				t.Fatalf("query = %q, missing expected table/field filter", query)
			}
			if len(args) != 1 || args[0] != int64(42) {
				t.Fatalf("args = %#v, want [42]", args)
			}
		})
	}
}
