package query_builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildExportFieldsQuery(t *testing.T) {
	query, args, err := BuildExportFieldsQuery(nil)
	require.NoError(t, err)
	upper := strings.ToUpper(query)
	require.Contains(t, upper, "TRANSACTION_RULE_FIELDS")
	require.NotContains(t, upper, "WHERE")
	require.Empty(t, args)

	id := int64(7)
	query, args, err = BuildExportFieldsQuery(&id)
	require.NoError(t, err)
	require.Contains(t, strings.ToUpper(query), "WHERE")
	require.Equal(t, []any{int64(7)}, args)
}

func TestBuildExportOverridesQuery(t *testing.T) {
	query, args, err := BuildExportOverridesQuery(nil)
	require.NoError(t, err)
	upper := strings.ToUpper(query)
	for _, f := range []string{"TRANSACTION_RULE_OVERRIDES", "CREATED_AT", "DESC", "TARGET_NAME", "FIELDS"} {
		require.Contains(t, upper, f)
	}
	require.NotContains(t, upper, "WHERE")
	require.Empty(t, args)

	id := int64(7)
	query, args, err = BuildExportOverridesQuery(&id)
	require.NoError(t, err)
	require.Contains(t, strings.ToUpper(query), "WHERE")
	require.Equal(t, []any{int64(7)}, args)
}

func TestBuildAllCategoriesOverridesQuery(t *testing.T) {
	query, args, err := BuildAllCategoriesOverridesQuery()
	require.NoError(t, err)
	require.Contains(t, strings.ToUpper(query), "IS NULL")
	require.Empty(t, args)
}
