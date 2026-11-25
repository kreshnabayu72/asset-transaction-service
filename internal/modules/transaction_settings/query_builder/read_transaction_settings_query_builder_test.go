package query_builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildNodeFieldsQuery_CompanyDefault(t *testing.T) {
	query, args, err := BuildNodeFieldsQuery(nil)
	require.NoError(t, err)
	upper := strings.ToUpper(query)
	for _, f := range []string{"TRANSACTION_RULE_FIELDS", "CATEGORY_ID", "IS NULL", "FIELD_KEY", "MODE", "VALUE", "ORDER BY"} {
		require.Contains(t, upper, f)
	}
	require.Empty(t, args)
}

func TestBuildNodeFieldsQuery_Category(t *testing.T) {
	id := int64(42)
	query, args, err := BuildNodeFieldsQuery(&id)
	require.NoError(t, err)
	require.NotContains(t, strings.ToUpper(query), "IS NULL")
	require.Equal(t, []any{int64(42)}, args)
}

func TestBuildSettingsVersionQuery(t *testing.T) {
	query, args, err := BuildSettingsVersionQuery()
	require.NoError(t, err)
	upper := strings.ToUpper(query)
	require.Contains(t, upper, "SETTINGS_VERSION")
	require.Contains(t, upper, "TRANSACTION_SETTINGS")
	require.Len(t, args, 1)
}
