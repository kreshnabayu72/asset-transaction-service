package query_builder

import (
	"encoding/json"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

func TestInsertChildQueriesMatchDeclaredMigrationColumns(t *testing.T) {
	tests := []struct {
		name        string
		build       func() (string, []any, error)
		wantColumns []string
		unwanted    []string
	}{
		{
			name: "service center",
			build: func() (string, []any, error) {
				return BuildInsertServiceCenterQuery(model.ServiceCenter{ID: 1, Name: "Center", NameKey: "center", ServiceTypes: []string{"Repair"}, CreatedByName: "Admin"})
			},
			wantColumns: []string{"id", "name", "name_key", "service_types", "is_active", "country", "province_state", "city", "postal_code", "address_line", "company_phone_dial_code", "company_phone_number", "company_email", "notes", "version", "created_by_id", "created_by_name", "created_at", "updated_at", "modified_by_id", "modified_by_name"},
			unwanted:    []string{"storage_path", "file_id"},
		},
		{
			name: "groups",
			build: func() (string, []any, error) {
				return BuildInsertServiceCenterGroupsQuery(model.ServiceCenterGroup{ServiceCenterID: 10, GroupID: 20})
			},
			wantColumns: []string{"service_center_id", "group_id"},
			unwanted:    []string{"created_at", "updated_at", "storage_path", "file_id"},
		},
		{
			name: "categories",
			build: func() (string, []any, error) {
				return BuildInsertServiceCenterCategoriesQuery(model.ServiceCenterCategory{ServiceCenterID: 10, CategoryID: 30})
			},
			wantColumns: []string{"service_center_id", "category_id"},
			unwanted:    []string{"created_at", "updated_at", "storage_path", "file_id"},
		},
		{
			name: "brands",
			build: func() (string, []any, error) {
				return BuildInsertServiceCenterBrandsQuery(model.ServiceCenterBrand{ServiceCenterID: 10, BrandID: 40})
			},
			wantColumns: []string{"service_center_id", "brand_id"},
			unwanted:    []string{"created_at", "updated_at", "storage_path", "file_id"},
		},
		{
			name: "contacts",
			build: func() (string, []any, error) {
				return BuildInsertServiceCenterContactsQuery(model.ServiceCenterContact{ID: 11, ServiceCenterID: 10, Position: 0, Name: "Primary", IsPrimary: true})
			},
			wantColumns: []string{"id", "service_center_id", "position", "name", "phone_dial_code", "phone_number", "email", "is_primary"},
			unwanted:    []string{"created_at", "updated_at", "storage_path", "file_id"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			query, args, err := test.build()
			if err != nil {
				t.Fatalf("build query: %v", err)
			}
			if len(args) != len(test.wantColumns) {
				t.Fatalf("argument count = %d, want %d; query: %s", len(args), len(test.wantColumns), query)
			}
			for _, column := range test.wantColumns {
				if !strings.Contains(query, column) {
					t.Errorf("query does not include declared column %q: %s", column, query)
				}
			}
			for _, column := range test.unwanted {
				if strings.Contains(query, column) {
					t.Errorf("query includes undeclared column %q: %s", column, query)
				}
			}
		})
	}
}

func TestDeleteServiceCenterChildRowsReturnExistingColumn(t *testing.T) {
	tables := []string{
		"service_center_groups",
		"service_center_categories",
		"service_center_brands",
		"service_center_contacts",
	}
	for _, table := range tables {
		t.Run(table, func(t *testing.T) {
			query, _, err := BuildDeleteServiceCenterChildRowsQuery(table, 42)
			if err != nil {
				t.Fatalf("build child delete: %v", err)
			}
			if !strings.Contains(query, "WHERE \"service_center_id\" = $1") {
				t.Errorf("child delete query %q lacks service_center_id predicate", query)
			}
			if !strings.Contains(query, "RETURNING service_center_id") {
				t.Errorf("child delete query %q lacks existing RETURNING column", query)
			}
			if strings.Contains(query, "RETURNING id") {
				t.Errorf("child delete query %q returns nonexistent id column", query)
			}
		})
	}
}

func TestServiceCenterCRUDRowDecodesNumericDatabaseIDs(t *testing.T) {
	const raw = `{"id":2125148327514214401,"created_by_id":17,"modified_by_id":23,"version":4}`
	var row model.ServiceCenterCRUDRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("decode numeric database IDs: %v", err)
	}
	center := row.ToServiceCenter()
	if center.ID != 2125148327514214401 || center.CreatedByID != 17 || center.ModifiedByID == nil || *center.ModifiedByID != 23 {
		t.Fatalf("mapped center IDs = (%d, %d, %v), want numeric database values", center.ID, center.CreatedByID, center.ModifiedByID)
	}
	encoded, err := json.Marshal(center)
	if err != nil {
		t.Fatalf("encode API-facing model: %v", err)
	}
	for _, expected := range []string{`"id":"2125148327514214401"`, `"created_by_id":"17"`, `"modified_by_id":"23"`} {
		if !strings.Contains(string(encoded), expected) {
			t.Errorf("API-facing model JSON %s does not contain %s", encoded, expected)
		}
	}
}

func TestServiceCenterCRUDGroupRowDecodesLargeNumericIDsWithoutChangingAPIWireIDs(t *testing.T) {
	const serviceCenterID int64 = 2125159143944753152
	const groupID int64 = 2125159143944753153
	const raw = `{"service_center_id":2125159143944753152,"group_id":2125159143944753153}`
	var row model.ServiceCenterCRUDGroupRow
	if err := json.Unmarshal([]byte(raw), &row); err != nil {
		t.Fatalf("decode numeric CRUD group row: %v", err)
	}
	if row.ServiceCenterID != serviceCenterID || row.GroupID != groupID {
		t.Fatalf("decoded IDs = (%d, %d), want (%d, %d)", row.ServiceCenterID, row.GroupID, serviceCenterID, groupID)
	}

	apiRow, err := json.Marshal(model.ServiceCenterGroup{ServiceCenterID: row.ServiceCenterID, GroupID: row.GroupID})
	if err != nil {
		t.Fatalf("encode API-facing group: %v", err)
	}
	for _, expected := range []string{`"service_center_id":"2125159143944753152"`, `"group_id":"2125159143944753153"`} {
		if !strings.Contains(string(apiRow), expected) {
			t.Errorf("API group JSON %s does not contain %s", apiRow, expected)
		}
	}

	query, _, err := BuildCRUDGroupsQuery(serviceCenterID)
	if err != nil {
		t.Fatalf("build CRUD group query: %v", err)
	}
	for _, column := range []string{"service_center_id", "group_id"} {
		if !strings.Contains(query, column) {
			t.Errorf("CRUD group query %q missing %q", query, column)
		}
	}
}
