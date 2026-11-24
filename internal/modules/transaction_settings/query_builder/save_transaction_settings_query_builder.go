package query_builder

import "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"

const (
	// RuleFieldsTable is the table the save upserts into.
	RuleFieldsTable = tableRuleFields
	// RuleFieldsConflictTarget is the unique key (NULL category_id counts as equal) of transaction_rule_fields.
	RuleFieldsConflictTarget = "(category_id, transaction_type, field_key)"
	// SettingsTable is the table holding the settings version.
	SettingsTable = tableSettings
	// SettingsSingletonID is the id of the transaction_settings singleton row.
	SettingsSingletonID = settingsSingleID
)

// RuleFieldsUpsertColumns are the columns a save overwrites on conflict.
var RuleFieldsUpsertColumns = []string{"mode", "value", "modified_by_id", "modified_by_name", "updated_at"}

// BuildBumpSettingsVersionQuery increases settings_version of the singleton by one.
func BuildBumpSettingsVersionQuery(actorID int64, actorName string) (string, []any, error) {
	return sql_query.NewSQLUpdateBuilder(tableSettings).
		Update(sql_query.UpdateQuery{
			"settings_version": sql_query.UpdateRawSQL{Expr: "settings_version + 1"},
			"modified_by_id":   actorID,
			"modified_by_name": actorName,
		}).
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: settingsSingleID},
		}).
		Build()
}
