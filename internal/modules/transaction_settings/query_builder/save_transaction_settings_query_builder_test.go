package query_builder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildBumpSettingsVersionQuery(t *testing.T) {
	query, args, err := BuildBumpSettingsVersionQuery(7, "Rina Wulandari")
	require.NoError(t, err)
	require.Contains(t, query, "UPDATE transaction_settings")
	require.Contains(t, query, "settings_version\" = settings_version + 1")
	require.Contains(t, query, "id")
	require.Contains(t, args, int64(7))
	require.Contains(t, args, "Rina Wulandari")
}

func TestSaveConstants(t *testing.T) {
	require.Equal(t, "transaction_rule_fields", RuleFieldsTable)
	require.Equal(t, "(category_id, transaction_type, field_key)", RuleFieldsConflictTarget)
	require.Equal(t, "transaction_settings", SettingsTable)
	require.Equal(t, 1, SettingsSingletonID)
	require.Contains(t, RuleFieldsUpsertColumns, "mode")
}
