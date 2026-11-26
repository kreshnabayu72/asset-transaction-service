package query_builder

import (
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

type StatusCenterRow struct {
	ID       int64  `json:"id" column:"id"`
	Name     string `json:"name" column:"name"`
	IsActive bool   `json:"is_active" column:"is_active"`
	Version  int32  `json:"version" column:"version"`
}

type StatusGroupIDsRow struct {
	IDs []int64 `json:"ids" column:"ids"`
}

func BuildLockStatusCenterQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[StatusCenterRow]("service_centers").
		Select("id", "name", "is_active", "version").
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		ForUpdate(nil).
		Build()
}

func BuildStatusGroupIDsQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[StatusGroupIDsRow]("service_center_groups").
		SelectRaw("COALESCE(array_agg(group_id ORDER BY group_id), '{}'::bigint[]) AS ids").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}

func BuildUpdateStatusQuery(id int64, expectedVersion int32, isActive bool, actorID int64, actorName string) (string, []any, error) {
	return sql_query.NewSQLUpdateBuilder("service_centers").
		Update(sql_query.UpdateQuery{
			"is_active":        isActive,
			"version":          sql_query.UpdateRawSQL{Expr: "version + 1"},
			"modified_by_id":   actorID,
			"modified_by_name": actorName,
		}).
		Where(sql_query.WhereQuery{
			"id":      {Operator: sql_query.SQLOperatorEqual, Value: id},
			"version": {Operator: sql_query.SQLOperatorEqual, Value: expectedVersion},
		}).
		Return("id").
		Build()
}
