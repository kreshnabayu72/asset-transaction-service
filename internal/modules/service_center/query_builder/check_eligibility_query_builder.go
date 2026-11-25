package query_builder

import (
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

type EligibilityCenterRow struct {
	ID           int64    `json:"id" column:"id"`
	Name         string   `json:"name" column:"name"`
	IsActive     bool     `json:"is_active" column:"is_active"`
	ServiceTypes []string `json:"service_types" column:"service_types"`
}

type EligibilityIDsRow struct {
	IDs []int64 `json:"ids" column:"ids"`
}

func BuildCheckEligibilityCenterQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[EligibilityCenterRow]("service_centers").
		SelectRaw("id, name, is_active, service_types").
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}

func BuildCheckEligibilityGroupIDsQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[EligibilityIDsRow]("service_center_groups").
		SelectRaw("COALESCE(array_agg(group_id), '{}'::bigint[]) AS ids").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}

func BuildCheckEligibilityCategoryIDsQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[EligibilityIDsRow]("service_center_categories").
		SelectRaw("COALESCE(array_agg(category_id), '{}'::bigint[]) AS ids").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}

func BuildCheckEligibilityBrandIDsQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[EligibilityIDsRow]("service_center_brands").
		SelectRaw("COALESCE(array_agg(brand_id), '{}'::bigint[]) AS ids").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}
