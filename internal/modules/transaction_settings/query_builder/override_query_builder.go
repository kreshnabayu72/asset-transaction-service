package query_builder

import (
	"sort"
	"strings"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const (
	// OverridesTable is the table the override operations read and write.
	OverridesTable = tableRuleOverrides
	overrideAlias  = "o"
	icuCollation   = ` COLLATE "en-US-x-icu"`
)

var (
	scopeLabelSQL    = labelCase("o.scope", constants.ScopeLabels, "Asset")
	categoryLabelSQL = "COALESCE(o.category_path, '" + constants.CategoryAll + "')"
	// firstFieldLabelSQL is the alphabetically first display name among the changed fields.
	firstFieldLabelSQL = "(SELECT MIN(" + labelCase("fk.key", constants.FieldLabels, "fk.key") + ") FROM jsonb_object_keys(o.fields) AS fk(key))"
)

// labelCase renders CASE col WHEN 'key' THEN 'Label' ... ELSE fallback END (keys sorted).
func labelCase(col string, labels map[string]string, fallback string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString("CASE " + col)
	for _, k := range keys {
		sb.WriteString(" WHEN '" + k + "' THEN '" + strings.ReplaceAll(labels[k], "'", "''") + "'")
	}
	if strings.Contains(fallback, ".") {
		sb.WriteString(" ELSE " + fallback + " END")
	} else {
		sb.WriteString(" ELSE '" + fallback + "' END")
	}
	return sb.String()
}

var overrideSelectColumns = []string{
	`o.id::text AS "_id"`,
	`o.scope AS "scope"`,
	`o.target_name AS "target"`,
	categoryLabelSQL + ` AS "category"`,
	`o.transaction_type AS "transaction"`,
	`o.fields AS "fields"`,
	`o.target_id::text AS "targetId"`,
	`o.category_id::text AS "categoryId"`,
	`o.target_group_id::text AS "groupId"`,
	`o.created_by_name AS "createdBy"`,
	`o.created_at AS "createdAt"`,
}

var overrideSortExpr = map[string]string{
	"scope":      scopeLabelSQL + icuCollation,
	"target":     "o.target_name" + icuCollation,
	"overridden": firstFieldLabelSQL + icuCollation,
	"createdBy":  "o.created_by_name" + icuCollation,
	"category":   categoryLabelSQL + icuCollation,
	"createdAt":  "o.created_at",
}

// BuildOverrideListQuery returns one page of overrides with the total (LLD list steps 2-6).
func BuildOverrideListQuery(q dto.OverrideQuery) (string, []any, error) {
	b := overrideBase[dto.OverrideListRecord](q, true).Select(overrideSelectColumns...)
	sortBy := q.SortBy
	if _, ok := overrideSortExpr[sortBy]; !ok {
		sortBy = "createdAt"
	}
	asc := q.SortOrder > 0
	b.OrderBy([]string{overrideSortExpr[sortBy]}, asc)
	if sortBy != "createdAt" {
		b.OrderBy([]string{"o.created_at"}, false)
	}
	b.OrderBy([]string{"o.id"}, asc)
	b.Paginate(sql_query.Pagination{Page: q.Page, Limit: q.Limit})
	return b.Build()
}

// BuildOverrideScopeOptionsQuery returns the distinct scopes of the row set.
func BuildOverrideScopeOptionsQuery(q dto.OverrideQuery) (string, []any, error) {
	return overrideBase[dto.OptionRecord](q, false).Select(`o.scope AS "value"`).GroupBy("o.scope").Build()
}

// BuildOverrideCreatedByOptionsQuery returns the distinct creator names of the row set.
func BuildOverrideCreatedByOptionsQuery(q dto.OverrideQuery) (string, []any, error) {
	return overrideBase[dto.OptionRecord](q, false).
		Select(`o.created_by_name AS "value"`).GroupBy("o.created_by_name").Build()
}

// BuildOverrideFieldOptionsQuery returns the distinct changed field keys of the row set.
func BuildOverrideFieldOptionsQuery(q dto.OverrideQuery) (string, []any, error) {
	return overrideBase[dto.OptionRecord](q, false).
		CrossJoinLateral("SELECT jsonb_object_keys(o.fields) AS key", "ok").
		Select(`ok.key AS "value"`).GroupBy("ok.key").Build()
}

func overrideBase[T any](q dto.OverrideQuery, withFilters bool) sql_query.Selector {
	b := sql_query.NewSQLSelectBuilder[T](tableRuleOverrides, overrideAlias).ClearSelects()
	w := sql_query.WhereQuery{
		"o.transaction_type": {Operator: sql_query.SQLOperatorEqual, Value: q.Transaction},
	}
	b.Where(w)
	if q.CategoryID != nil {
		b.Where(sql_query.WhereQuery{"node": {
			Operator: sql_query.SQLOperatorRaw, Value: "(o.category_id = ? OR o.category_id IS NULL)", ExtraArgs: []any{*q.CategoryID},
		}})
	}
	if len(q.CategoryIDs) > 0 {
		b.Where(sql_query.WhereQuery{"o.category_id": {Operator: sql_query.SQLOperatorIn, Value: q.CategoryIDs}})
	}
	if q.CreatedFrom != nil && q.CreatedTo != nil {
		b.Where(sql_query.WhereQuery{"createdAt": {
			Operator:  sql_query.SQLOperatorRaw,
			Value:     "o.created_at >= to_timestamp(CAST(? AS BIGINT) / 1000.0) AND o.created_at <= to_timestamp(CAST(? AS BIGINT) / 1000.0)",
			ExtraArgs: []any{*q.CreatedFrom, *q.CreatedTo},
		}})
	}
	if q.VisibleGroupIDs != nil {
		ids := *q.VisibleGroupIDs
		if ids == nil {
			ids = []int64{}
		}
		b.Where(sql_query.WhereQuery{"visible": {
			Operator:  sql_query.SQLOperatorRaw,
			Value:     "((o.scope = 'group' AND o.target_id = ANY(?)) OR (o.scope = 'asset' AND o.target_group_id = ANY(?)))",
			ExtraArgs: []any{ids, ids},
		}})
	}
	if withFilters {
		applyOverrideFilters(b, q)
	}
	return b
}

func applyOverrideFilters(b sql_query.Selector, q dto.OverrideQuery) {
	if len(q.Scopes) > 0 {
		b.Where(sql_query.WhereQuery{"o.scope": {Operator: sql_query.SQLOperatorIn, Value: q.Scopes}})
	}
	if len(q.CreatedBy) > 0 {
		b.Where(sql_query.WhereQuery{"o.created_by_name": {Operator: sql_query.SQLOperatorIn, Value: q.CreatedBy}})
	}
	if q.OverriddenKeys != nil {
		b.Where(sql_query.WhereQuery{"overridden": {
			Operator: sql_query.SQLOperatorRaw, Value: "jsonb_exists_any(o.fields, ?)", ExtraArgs: []any{q.OverriddenKeys},
		}})
	}
	if q.Search != "" {
		b.Where(sql_query.WhereQuery{"search": searchCondition(q)})
	}
}

// searchCondition matches the search text as a case-insensitive literal substring (no wildcard
// meaning for `%`) against the searchable columns; with none, nothing matches.
func searchCondition(q dto.OverrideQuery) sql_query.SQLCondition {
	var parts []string
	var args []any
	contains := func(expr string) {
		parts = append(parts, "position(lower(?) in lower("+expr+")) > 0")
		args = append(args, q.Search)
	}
	for _, col := range q.SearchColumns {
		switch col {
		case "scope":
			contains(scopeLabelSQL)
		case "target":
			contains("o.target_name")
		case "createdBy":
			contains("o.created_by_name")
		case "category":
			contains(categoryLabelSQL)
		case "overridden":
			if len(q.SearchFieldKeys) > 0 {
				parts = append(parts, "jsonb_exists_any(o.fields, ?)")
				args = append(args, q.SearchFieldKeys)
			}
		}
	}
	if len(parts) == 0 {
		return sql_query.SQLCondition{Operator: sql_query.SQLOperatorRaw, Value: "FALSE"}
	}
	return sql_query.SQLCondition{Operator: sql_query.SQLOperatorRaw, Value: "(" + strings.Join(parts, " OR ") + ")", ExtraArgs: args}
}

// BuildOverridesByIDsQuery selects the overrides with the given ids.
func BuildOverridesByIDsQuery(ids []int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.TransactionRuleOverride](tableRuleOverrides).
		Where(sql_query.WhereQuery{"id": {Operator: sql_query.SQLOperatorIn, Value: ids}}).
		OrderBy([]string{"id"}, true).
		Build()
}

// BuildOverridesForTargetsQuery selects the overrides of one transaction on the given targets.
func BuildOverridesForTargetsQuery(transaction string, targetIDs []int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.TransactionRuleOverride](tableRuleOverrides).
		Where(sql_query.WhereQuery{
			"transaction_type": {Operator: sql_query.SQLOperatorEqual, Value: transaction},
			"target_id":        {Operator: sql_query.SQLOperatorIn, Value: targetIDs},
		}).
		OrderBy([]string{"id"}, true).
		Build()
}

// BuildDuplicateOverridesQuery selects the targets that already have an active override for the
// scope, transaction and Category (nil = All categories).
func BuildDuplicateOverridesQuery(scope, transaction string, categoryID *int64, targetIDs []int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[dto.DuplicateRecord](tableRuleOverrides).
		ClearSelects().
		Select("target_id").
		Where(sql_query.WhereQuery{
			"scope":            {Operator: sql_query.SQLOperatorEqual, Value: scope},
			"transaction_type": {Operator: sql_query.SQLOperatorEqual, Value: transaction},
			"target_id":        {Operator: sql_query.SQLOperatorIn, Value: targetIDs},
		}).
		Where(categoryCondition(categoryID)).
		Build()
}

// OverridesDeleteFilter selects the overrides with the given ids for deletion.
func OverridesDeleteFilter(ids []int64) sql_query.WhereQuery {
	return sql_query.WhereQuery{"id": {Operator: sql_query.SQLOperatorIn, Value: ids}}
}
