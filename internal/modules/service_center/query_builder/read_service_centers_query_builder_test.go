package query_builder

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

func TestBuildListServiceCentersQueryAppliesDirectScopeAndPagination(t *testing.T) {
	request := dto.ServiceCenterReadFilters{SortBy: "name", SortOrder: 1}
	query, args, err := BuildListServiceCentersQuery(request, []string{"name", "serviceTypes"}, []int64{11, 12}, false, false, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"service_center_groups", "ANY", "LIMIT", "OFFSET", "totalRecords"} {
		if !strings.Contains(query, part) {
			t.Errorf("query missing %q: %s", part, query)
		}
	}
	if len(args) == 0 {
		t.Fatal("expected bound arguments for scope and pagination")
	}
}

func TestBuildListServiceCentersQueryAppliesServiceTypeOverlapFilter(t *testing.T) {
	request := dto.ServiceCenterReadFilters{ServiceType: []string{"Repair"}}
	query, args, err := BuildListServiceCentersQuery(request, []string{"name"}, nil, true, false, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "s.service_types && $1::text[]") {
		t.Fatalf("query missing bound service type overlap predicate: %s", query)
	}
	if len(args) != 1 {
		t.Fatalf("argument count = %d, want 1 (%#v)", len(args), args)
	}
	serviceTypes, ok := args[0].([]string)
	if !ok || len(serviceTypes) != 1 || serviceTypes[0] != "Repair" {
		t.Fatalf("service type argument = %#v, want []string{\"Repair\"}", args[0])
	}
}

func TestBuildCountServiceCentersQueryUsesVisibleSearchFields(t *testing.T) {
	request := dto.ServiceCenterReadFilters{SortBy: "name", SortOrder: 1, Search: "  hp_50% "}
	query, args, err := BuildCountServiceCentersQuery(request, []string{"name", "city"}, []int64{22}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "ILIKE") || !strings.Contains(query, "service_center_groups") {
		t.Fatalf("expected search and scope in query: %s", query)
	}
	if len(args) < 2 {
		t.Fatalf("expected scope and search arguments, got %v", args)
	}
}

func TestBuildDetailServiceCenterQueryFailsClosedForEmptyScope(t *testing.T) {
	query, _, err := BuildDetailServiceCenterQuery(123, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "FALSE") {
		t.Fatalf("empty scope must be denied: %s", query)
	}
}

func TestUsageBackedListFilterReturnsProviderError(t *testing.T) {
	min := 1
	_, _, err := BuildCountServiceCentersQuery(
		dto.ServiceCenterReadFilters{SortBy: "name", SortOrder: 1, UsageMin: &min},
		[]string{"name"}, []int64{1}, false, false,
	)
	if !errors.Is(err, ErrUsageSourceUnavailable) {
		t.Fatalf("expected explicit missing usage source error, got %v", err)
	}
}

func TestReferenceNameDependentReadRequestsBuildQueries(t *testing.T) {
	search := dto.ServiceCenterReadFilters{SortBy: "name", SortOrder: 1, Search: "Panasonic"}
	if _, _, err := BuildListServiceCentersQuery(search, []string{"brands"}, []int64{1}, false, false, 1, 10); err != nil {
		t.Fatalf("reference-column list search should still build: %v", err)
	}
	sort := dto.ServiceCenterReadFilters{SortBy: "specializations", SortOrder: 1}
	if _, _, err := BuildListServiceCentersQuery(sort, []string{"name"}, []int64{1}, false, false, 1, 10); err != nil {
		t.Fatalf("reference-column sort should still build: %v", err)
	}
}

func TestBuildServiceCenterOptionsUsesScopedRows(t *testing.T) {
	query, args, err := BuildOptionsValueQuery("statusOptions", []int64{7}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "FROM service_centers s") || strings.Contains(query, "FROM service_centers s s") {
		t.Fatalf("expected one service-center alias in facet query: %s", query)
	}
	if !strings.Contains(query, "GROUP BY") || !strings.Contains(query, "service_center_groups") {
		t.Fatalf("expected grouped, scoped facet query: %s", query)
	}
	if len(args) == 0 {
		t.Fatal("expected group scope argument")
	}
}

func TestBuildServiceCenterReferenceOptionsUseLinkedIDs(t *testing.T) {
	cases := []struct {
		facet string
		link  string
	}{
		{"availableToOptions", "group_id::text AS value FROM service_center_groups"},
		{"specializationOptions", "category_id::text AS value FROM service_center_categories"},
		{"brandOptions", "brand_id::text AS value FROM service_center_brands"},
	}
	for _, test := range cases {
		t.Run(test.facet, func(t *testing.T) {
			query, _, err := BuildOptionsValueQuery(test.facet, []int64{7}, true, false)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(query, "CROSS JOIN LATERAL") || !strings.Contains(query, test.link) {
				t.Fatalf("expected linked reference IDs from local rows, got: %s", query)
			}
		})
	}
}

func TestReadProjectionUsesDatabaseColumnNamesAndNumericIDs(t *testing.T) {
	const rowID int64 = 2125148327514214401
	const createdByID int64 = 2125148327514214402
	const groupID int64 = 2125148327514214403
	const categoryID int64 = 2125148327514214404
	const brandID int64 = 2125148327514214405
	var row ReadCenterRow
	if err := json.Unmarshal([]byte(`{"id":"2125148327514214401","service_types":["Repair"],"is_active":true,"created_by_id":"2125148327514214402","group_ids":["2125148327514214403"],"category_ids":["2125148327514214404"],"brand_ids":["2125148327514214405"]}`), &row); err != nil {
		t.Fatal(err)
	}
	if row.ID != rowID || row.CreatedByID != createdByID || !row.IsActive || len(row.ServiceTypes) != 1 || row.ServiceTypes[0] != "Repair" {
		t.Fatalf("read projection lost numeric values: %#v", row)
	}
	if len(row.GroupIDs) != 1 || row.GroupIDs[0] != groupID || len(row.CategoryIDs) != 1 || row.CategoryIDs[0] != categoryID || len(row.BrandIDs) != 1 || row.BrandIDs[0] != brandID {
		t.Fatalf("read projection lost linked numeric IDs: %#v", row)
	}
	query, _, err := BuildListServiceCentersQuery(dto.ServiceCenterReadFilters{SortBy: "name", SortOrder: 1}, []string{"name"}, nil, true, false, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"s.id::text AS id",
		"s.created_by_id::text AS created_by_id",
		"array_agg(scg.group_id::text",
		"array_agg(scc.category_id::text",
		"array_agg(scb.brand_id::text",
	} {
		if !strings.Contains(query, fragment) {
			t.Errorf("read query does not preserve snowflake IDs as text with %q: %s", fragment, query)
		}
	}
	var contact ContactPersonRow
	if err := json.Unmarshal([]byte(`{"name":"A","phone_dial_code":"+62","phone_number":"1234","is_primary":true}`), &contact); err != nil {
		t.Fatal(err)
	}
	if contact.PhoneDialCode == nil || *contact.PhoneDialCode != "+62" || !contact.IsPrimary {
		t.Fatalf("contact projection was not decoded from PostgreSQL JSON keys: %#v", contact)
	}
}
