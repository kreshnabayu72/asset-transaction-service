package query_builder

import (
	"strings"
	"testing"
)

func TestBuildLockStatusCenterQuery(t *testing.T) {
	query, args, err := BuildLockStatusCenterQuery(42)
	if err != nil {
		t.Fatalf("BuildLockStatusCenterQuery() error = %v", err)
	}
	upper := strings.ToUpper(query)
	for _, fragment := range []string{"SERVICE_CENTERS", "IS_ACTIVE", "VERSION", "FOR UPDATE"} {
		if !strings.Contains(upper, strings.ToUpper(fragment)) {
			t.Errorf("query %q does not contain %q", query, fragment)
		}
	}
	if len(args) != 1 || args[0] != int64(42) {
		t.Fatalf("args = %#v, want [42]", args)
	}
}

func TestBuildStatusGroupIDsQueryUsesLocalGroupTable(t *testing.T) {
	query, args, err := BuildStatusGroupIDsQuery(84)
	if err != nil {
		t.Fatalf("BuildStatusGroupIDsQuery() error = %v", err)
	}
	upper := strings.ToUpper(query)
	for _, fragment := range []string{"SERVICE_CENTER_GROUPS", "ARRAY_AGG", "GROUP_ID"} {
		if !strings.Contains(upper, strings.ToUpper(fragment)) {
			t.Errorf("query %q does not contain %q", query, fragment)
		}
	}
	if strings.Contains(upper, "FIXED_ASSET_GROUPS") || strings.Contains(upper, "SCHEDULE") {
		t.Fatalf("query reaches outside local service-center tables: %s", query)
	}
	if len(args) != 1 || args[0] != int64(84) {
		t.Fatalf("args = %#v, want [84]", args)
	}
}

func TestBuildUpdateStatusQueryConditionallyIncrementsVersion(t *testing.T) {
	query, args, err := BuildUpdateStatusQuery(42, 7, false, 999, "Rina")
	if err != nil {
		t.Fatalf("BuildUpdateStatusQuery() error = %v", err)
	}
	upper := strings.ToUpper(query)
	for _, fragment := range []string{"UPDATE", "SERVICE_CENTERS", "IS_ACTIVE", "VERSION", "VERSION + 1", "MODIFIED_BY_ID", "MODIFIED_BY_NAME", "UPDATED_AT", "WHERE", "RETURNING"} {
		if !strings.Contains(upper, strings.ToUpper(fragment)) {
			t.Errorf("query %q does not contain %q", query, fragment)
		}
	}
	if len(args) != 5 {
		t.Fatalf("got %d args (%#v), expected update values plus id and version", len(args), args)
	}
}
