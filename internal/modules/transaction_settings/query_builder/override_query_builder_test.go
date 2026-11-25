package query_builder

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

func i64(v int64) *int64 { return &v }

func TestBuildOverrideListQuery_Defaults(t *testing.T) {
	query, args, err := BuildOverrideListQuery(dto.OverrideQuery{Transaction: "borrow", SortBy: "createdAt", SortOrder: -1, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Contains(t, query, "transaction_rule_overrides")
	require.Contains(t, query, "o.transaction_type")
	require.Contains(t, query, "totalRecords")
	require.Contains(t, query, "o.created_at")
	require.NotContains(t, query, "COLLATE")
	require.Contains(t, args, "borrow")
}

func TestBuildOverrideListQuery_SortExpressions(t *testing.T) {
	for _, key := range []string{"scope", "target", "overridden", "createdBy", "category"} {
		query, _, err := BuildOverrideListQuery(dto.OverrideQuery{Transaction: "borrow", SortBy: key, SortOrder: 1, Page: 1, Limit: 10})
		require.NoError(t, err, key)
		require.Contains(t, query, `COLLATE "en-US-x-icu"`, key)
	}
	// An unknown key falls back to the default sort.
	query, _, err := BuildOverrideListQuery(dto.OverrideQuery{Transaction: "borrow", SortBy: "id", Page: 1, Limit: 10})
	require.NoError(t, err)
	require.NotContains(t, query, "COLLATE")
}

func TestBuildOverrideListQuery_Filters(t *testing.T) {
	groups := []int64{3, 4}
	q := dto.OverrideQuery{
		Transaction: "borrow", CategoryID: i64(42), Search: "karawang", SearchColumns: []string{"scope", "target", "overridden", "createdBy", "category"},
		SearchFieldKeys: []string{"max_duration"}, Scopes: []string{"group"}, OverriddenKeys: []string{"max_duration"},
		CreatedBy: []string{"Rina"}, CategoryIDs: []int64{7}, CreatedFrom: i64(1), CreatedTo: i64(2), VisibleGroupIDs: &groups,
		SortBy: "createdAt", SortOrder: -1, Page: 2, Limit: 25,
	}
	query, args, err := BuildOverrideListQuery(q)
	require.NoError(t, err)
	for _, want := range []string{
		"o.category_id = ", "o.category_id IS NULL", `"o"."category_id" IN`, `"o"."scope" IN`, `"o"."created_by_name" IN`, "jsonb_exists_any(o.fields",
		"to_timestamp", "o.target_group_id = ANY", "position(lower(", "o.target_name", "OFFSET 25",
	} {
		require.Contains(t, query, want)
	}
	require.Contains(t, args, "karawang")
	require.Contains(t, args, int64(42))
}

func TestBuildOverrideListQuery_SearchWithoutSearchableColumn(t *testing.T) {
	query, _, err := BuildOverrideListQuery(dto.OverrideQuery{Transaction: "borrow", Search: "x", SearchColumns: nil, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Contains(t, query, "FALSE")
	// overridden alone without matching field keys adds no clause, an empty visibility set matches nothing.
	empty := []int64(nil)
	query, _, err = BuildOverrideListQuery(dto.OverrideQuery{Transaction: "borrow", Search: "x", SearchColumns: []string{"overridden"}, VisibleGroupIDs: &empty, Page: 1, Limit: 10})
	require.NoError(t, err)
	require.Contains(t, query, "FALSE")
	require.Contains(t, query, "o.scope = 'group'")
}

func TestBuildOverrideOptionsQueries(t *testing.T) {
	q := dto.OverrideQuery{Transaction: "borrow", CategoryID: i64(42), Scopes: []string{"group"}, CreatedBy: []string{"Rina"}, Search: "x"}
	for name, build := range map[string]func(dto.OverrideQuery) (string, []any, error){
		"scope":     BuildOverrideScopeOptionsQuery,
		"createdBy": BuildOverrideCreatedByOptionsQuery,
		"fields":    BuildOverrideFieldOptionsQuery,
	} {
		query, _, err := build(q)
		require.NoError(t, err, name)
		require.Contains(t, query, "GROUP BY", name)
		// The options flags are not filters: scope, createdBy and search are not applied.
		require.NotContains(t, query, `"o"."scope" IN`, name)
		require.NotContains(t, query, "position(", name)
	}
	query, _, _ := BuildOverrideFieldOptionsQuery(q)
	require.Contains(t, query, "jsonb_object_keys(o.fields)")
}

func TestBuildOverrideLookupQueries(t *testing.T) {
	query, args, err := BuildOverridesByIDsQuery([]int64{1, 2})
	require.NoError(t, err)
	require.Contains(t, query, "IN")
	require.Len(t, args, 2)

	query, args, err = BuildOverridesForTargetsQuery("assign", []int64{10})
	require.NoError(t, err)
	require.Contains(t, query, "target_id")
	require.Contains(t, args, "assign")

	query, _, err = BuildDuplicateOverridesQuery("group", "borrow", nil, []int64{10})
	require.NoError(t, err)
	require.Contains(t, query, `"category_id" IS NULL`)
	require.NotContains(t, query, "string")
	query, args, err = BuildDuplicateOverridesQuery("group", "borrow", i64(5), []int64{10})
	require.NoError(t, err)
	require.Contains(t, args, int64(5))
	require.Contains(t, query, "SELECT")

	require.Contains(t, OverridesDeleteFilter([]int64{1}), "id")
	require.Equal(t, "transaction_rule_overrides", OverridesTable)
}
