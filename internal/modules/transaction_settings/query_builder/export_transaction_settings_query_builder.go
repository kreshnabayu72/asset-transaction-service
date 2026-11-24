package query_builder

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const tableRuleOverrides = "transaction_rule_overrides"

// BuildExportFieldsQuery selects the rule fields of an export: every stored row (Company default
// and every Category) when categoryID is nil, the rows of that Category otherwise. The caller
// adds the Company default rows of a leaf export with BuildNodeFieldsQuery(nil).
func BuildExportFieldsQuery(categoryID *int64) (string, []any, error) {
	b := sql_query.NewSQLSelectBuilder[model.TransactionRuleField](tableRuleFields)
	if categoryID != nil {
		b = b.Where(categoryCondition(categoryID))
	}
	return b.OrderBy([]string{"category_id", "transaction_type", "field_key"}, true).Build()
}

// BuildExportOverridesQuery selects the overrides of an export, newest first: every override when
// categoryID is nil, those of that Category otherwise. The caller adds the All categories
// overrides (BuildAllCategoriesOverridesQuery) for a leaf export.
func BuildExportOverridesQuery(categoryID *int64) (string, []any, error) {
	b := sql_query.NewSQLSelectBuilder[model.TransactionRuleOverride](tableRuleOverrides)
	if categoryID != nil {
		b = b.Where(categoryCondition(categoryID))
	}
	return b.OrderBy([]string{"created_at"}, false).OrderBy([]string{"id"}, true).Build()
}

// BuildAllCategoriesOverridesQuery selects the overrides stored at All categories (category_id IS NULL).
func BuildAllCategoriesOverridesQuery() (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.TransactionRuleOverride](tableRuleOverrides).
		Where(categoryCondition(nil)).
		OrderBy([]string{"created_at"}, false).
		OrderBy([]string{"id"}, true).
		Build()
}
