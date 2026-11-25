// Package query_builder builds the SQL of the transaction settings read and export queries.
package query_builder

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const (
	tableRuleFields  = "transaction_rule_fields"
	tableSettings    = "transaction_settings"
	settingsSingleID = 1
)

// SettingsVersionRow is the settings version of the entity.
type SettingsVersionRow struct {
	SettingsVersion int64 `json:"settings_version" column:"settings_version"`
}

// BuildNodeFieldsQuery selects the rule fields stored at one node: the Company default rows
// (category_id IS NULL) when categoryID is nil, the rows of that Category otherwise.
func BuildNodeFieldsQuery(categoryID *int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.TransactionRuleField](tableRuleFields).
		Where(categoryCondition(categoryID)).
		OrderBy([]string{"transaction_type", "field_key"}, true).
		Build()
}

// BuildSettingsVersionQuery selects settings_version of the transaction_settings singleton.
func BuildSettingsVersionQuery() (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[SettingsVersionRow](tableSettings).
		Select("settings_version").
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: settingsSingleID},
		}).
		Build()
}

// categoryCondition filters category_id: IS NULL for the Company default, equality for a Category.
func categoryCondition(categoryID *int64) sql_query.WhereQuery {
	if categoryID == nil {
		return sql_query.WhereQuery{"category_id": {Operator: sql_query.SQLOperatorIsNull}}
	}
	return sql_query.WhereQuery{"category_id": {Operator: sql_query.SQLOperatorEqual, Value: *categoryID}}
}
