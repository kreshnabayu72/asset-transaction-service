// Package query_builder builds the SQL of the damaged module read operations.
package query_builder

import (
	"errors"
	"time"

	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const (
	tableDamageReports = "damage_reports"
	tableAssets        = "assets"
	damageReportsAlias = "dr"
	assetsAlias        = "a"
)

// badgeStatuses are the statuses counted by the Damaged badge.
var badgeStatuses = []string{"waiting_for_confirmation", "damaged"}

// runningStatuses are the statuses of a running damage record.
var runningStatuses = []string{"waiting_for_confirmation", "damaged", "reported_disposal", "in_disposal_process"}

// statusLabels map each running status to its wire label.
var statusLabels = []sql_query.CaseWhenPart{
	{When: "status = 'waiting_for_confirmation'", Then: "'Waiting for Confirmation'"},
	{When: "status = 'damaged'", Then: "'Damaged'"},
	{When: "status = 'reported_disposal'", Then: "'Reported Disposal'"},
	{When: "status = 'in_disposal_process'", Then: "'In Disposal Process'"},
}

// missingMarkerLabels map each missing marker to its wire label.
var missingMarkerLabels = []sql_query.CaseWhenPart{
	{When: "missing_marker = 'reported_missing'", Then: "'Reported Missing'"},
	{When: "missing_marker = 'missing'", Then: "'Missing'"},
}

// AssetScopeRow is the mirror asset row read for the scope check.
type AssetScopeRow struct {
	ID       int64  `json:"id" column:"id"`
	GroupID  *int64 `json:"group_id" column:"group_id"`
	HolderID *int64 `json:"holder_id" column:"holder_id"`
}

// RunningRecordRow is the running damage record of an asset, with its wire labels.
type RunningRecordRow struct {
	ID            int64     `json:"id" column:"id"`
	Status        string    `json:"status" column:"status"`
	MissingMarker *string   `json:"missing_marker" column:"missing_marker"`
	ReportedAt    time.Time `json:"reported_at" column:"reported_at"`
}

// BuildBadgeCountQuery counts the Waiting for Confirmation and Damaged records without a Missing
// marker. Unless allGroups (Total Control), only records whose asset is in groupIDs are counted.
func BuildBadgeCountQuery(allGroups bool, groupIDs []int64) (string, []any, error) {
	if !allGroups && len(groupIDs) == 0 {
		return "", nil, errors.New("badge count needs groups or all groups")
	}
	builder := sql_query.NewSQLCountBuilder(tableDamageReports, damageReportsAlias).
		Join(tableAssets+" "+assetsAlias, "a.id = dr.asset_id").
		Where(sql_query.WhereQuery{
			"dr.status": {Operator: sql_query.SQLOperatorIn, Value: badgeStatuses},
		}).
		Where(sql_query.WhereQuery{
			"dr.missing_marker": {Operator: sql_query.SQLOperatorIsNull},
		})
	if !allGroups {
		builder = builder.Where(sql_query.WhereQuery{
			"a.group_id": {Operator: sql_query.SQLOperatorIn, Value: groupIDs},
		})
	}
	return builder.Build()
}

// BuildAssetScopeQuery selects the group of one mirror asset. No row means the asset is unknown.
func BuildAssetScopeQuery(assetID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[AssetScopeRow](tableAssets).
		ClearSelects().
		Select("id", "group_id", "holder_id").
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: assetID},
		}).
		Build()
}

// BuildRunningRecordQuery selects the running damage record of one asset with its wire labels.
// At most one running record exists per asset.
func BuildRunningRecordQuery(assetID int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[RunningRecordRow](tableDamageReports).
		ClearSelects().
		Select("id", "reported_at").
		SelectMultiCaseWhen(statusLabels, "status", "status").
		SelectMultiCaseWhen(missingMarkerLabels, "NULL", "missing_marker").
		Where(sql_query.WhereQuery{
			"asset_id": {Operator: sql_query.SQLOperatorEqual, Value: assetID},
		}).
		Where(sql_query.WhereQuery{
			"status": {Operator: sql_query.SQLOperatorIn, Value: runningStatuses},
		}).
		Build()
}
