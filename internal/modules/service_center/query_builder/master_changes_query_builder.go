package query_builder

import (
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const (
	serviceCentersTable          = "service_centers"
	serviceCenterGroupsTable     = "service_center_groups"
	serviceCenterCategoriesTable = "service_center_categories"
	serviceCenterBrandsTable     = "service_center_brands"
)

// TargetCenterRow is one distinct Service Center linked to changed master IDs.
type TargetCenterRow struct {
	ServiceCenterID int64 `json:"service_center_id" column:"service_center_id"`
}

// LockedServiceCenterRow contains fields read while the Service Center row is locked.
type LockedServiceCenterRow struct {
	ID          int64  `json:"id" column:"id"`
	IsActive    bool   `json:"is_active" column:"is_active"`
	Version     int32  `json:"version" column:"version"`
	Name        string `json:"name" column:"name"`
	CreatedByID int64  `json:"created_by_id" column:"created_by_id"`
}

// LinkIDRow holds a linked master ID.
type LinkIDRow struct {
	ID int64 `json:"id" column:"id"`
}

func BuildTargetCentersQuery(changeType string, ids []int64) (string, []any, error) {
	table, column, err := masterLink(changeType)
	if err != nil {
		return "", nil, err
	}
	return sql_query.NewSQLSelectBuilder[TargetCenterRow](table).
		ClearSelects().
		Select("DISTINCT service_center_id").
		Where(sql_query.WhereQuery{
			column: {Operator: sql_query.SQLOperatorIn, Value: ids},
		}).
		OrderBy([]string{"service_center_id"}, true).
		Build()
}

func BuildLockServiceCenterQuery(serviceCenterID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[LockedServiceCenterRow](serviceCentersTable).
		ClearSelects().
		Select("id", "is_active", "version", "name", "created_by_id").
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		ForUpdate(nil).
		Build()
}

func BuildCurrentLinksQuery(changeType string, serviceCenterID int64) (string, []any, error) {
	table, column, err := masterLink(changeType)
	if err != nil {
		return "", nil, err
	}
	return sql_query.NewSQLSelectBuilder[LinkIDRow](table).
		ClearSelects().
		Select(column+" AS id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		OrderBy([]string{column}, true).
		Build()
}

func BuildDeleteLinksQuery(changeType string, serviceCenterID int64, ids []int64) (string, []any, error) {
	table, column, err := masterLink(changeType)
	if err != nil {
		return "", nil, err
	}
	deleteBuilder := sql_query.NewSQLDeleteBuilder(table).
		Delete("service_center_id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Where(sql_query.WhereQuery{
			column: {Operator: sql_query.SQLOperatorIn, Value: ids},
		})
	return deleteBuilder.Build()
}

func BuildServiceCenterUpdateQuery(serviceCenterID int64, autoDeactivate bool) (string, []any, error) {
	update := sql_query.UpdateQuery{
		"version":          sql_query.UpdateRawSQL{Expr: "version + 1"},
		"modified_by_id":   nil,
		"modified_by_name": "System",
	}
	if autoDeactivate {
		update["is_active"] = false
	}
	return sql_query.NewSQLUpdateBuilder(serviceCentersTable).
		Update(update).
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID},
		}).
		Build()
}

func masterLink(changeType string) (table, column string, err error) {
	switch changeType {
	case dto.MasterChangeTypeGroup:
		return serviceCenterGroupsTable, "group_id", nil
	case dto.MasterChangeTypeCategory:
		return serviceCenterCategoriesTable, "category_id", nil
	case dto.MasterChangeTypeBrand:
		return serviceCenterBrandsTable, "brand_id", nil
	default:
		return "", "", fmt.Errorf("unsupported master change type %q", changeType)
	}
}
