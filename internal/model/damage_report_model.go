package model

import (
	"encoding/json"
	"time"
)

// DamageReport is one damage record; at most one running record exists per asset.
type DamageReport struct {
	ID                      int64            `json:"id,string" column:"damage_reports.id"`
	AssetID                 int64            `json:"asset_id,string" column:"damage_reports.asset_id"`
	Status                  string           `json:"status" column:"damage_reports.status"`
	Source                  string           `json:"source" column:"damage_reports.source"`
	ReporterType            string           `json:"reporter_type" column:"damage_reports.reporter_type"`
	Effect                  string           `json:"effect" column:"damage_reports.effect"`
	MissingMarker           *string          `json:"missing_marker" column:"damage_reports.missing_marker"`
	WhatHappened            *string          `json:"what_happened" column:"damage_reports.what_happened"`
	LastHolderID            *int64           `json:"last_holder_id,string" column:"damage_reports.last_holder_id"`
	LastHolderFullName      *string          `json:"last_holder_full_name" column:"damage_reports.last_holder_full_name"`
	LastHolderFirstName     *string          `json:"last_holder_first_name" column:"damage_reports.last_holder_first_name"`
	LastHolderLastName      *string          `json:"last_holder_last_name" column:"damage_reports.last_holder_last_name"`
	LastHolderDivision      *string          `json:"last_holder_division" column:"damage_reports.last_holder_division"`
	LastHolderEmployeeID    *int64           `json:"last_holder_employee_id,string" column:"damage_reports.last_holder_employee_id"`
	LastHolderPicName       *string          `json:"last_holder_pic_name" column:"damage_reports.last_holder_pic_name"`
	ReportedByID            int64            `json:"reported_by_id,string" column:"damage_reports.reported_by_id"`
	ReportedByFullName      string           `json:"reported_by_full_name" column:"damage_reports.reported_by_full_name"`
	ReportedByFirstName     string           `json:"reported_by_first_name" column:"damage_reports.reported_by_first_name"`
	ReportedByLastName      string           `json:"reported_by_last_name" column:"damage_reports.reported_by_last_name"`
	OnBehalfOfEmployeeID    *int64           `json:"on_behalf_of_employee_id,string" column:"damage_reports.on_behalf_of_employee_id"`
	OnBehalfOfName          *string          `json:"on_behalf_of_name" column:"damage_reports.on_behalf_of_name"`
	ReportedAt              time.Time        `json:"reported_at" column:"damage_reports.reported_at"`
	LastNodeByID            *int64           `json:"last_node_by_id,string" column:"damage_reports.last_node_by_id"`
	LastNodeByName          *string          `json:"last_node_by_name" column:"damage_reports.last_node_by_name"`
	LastNodeAt              *time.Time       `json:"last_node_at" column:"damage_reports.last_node_at"`
	DisposalReportID        *int64           `json:"disposal_report_id,string" column:"damage_reports.disposal_report_id"`
	ResolvedByID            *int64           `json:"resolved_by_id,string" column:"damage_reports.resolved_by_id"`
	ResolvedByName          *string          `json:"resolved_by_name" column:"damage_reports.resolved_by_name"`
	ResolvedAt              *time.Time       `json:"resolved_at" column:"damage_reports.resolved_at"`
	ResolvedGroupID         *int64           `json:"resolved_group_id,string" column:"damage_reports.resolved_group_id"`
	ResolvedGroupName       *string          `json:"resolved_group_name" column:"damage_reports.resolved_group_name"`
	ResolvedCategoryID      *int64           `json:"resolved_category_id,string" column:"damage_reports.resolved_category_id"`
	ResolvedCategoryName    *string          `json:"resolved_category_name" column:"damage_reports.resolved_category_name"`
	ResolvedCommercialValue *json.RawMessage `json:"resolved_commercial_value" column:"damage_reports.resolved_commercial_value"`
	ResolvedFiscalValue     *json.RawMessage `json:"resolved_fiscal_value" column:"damage_reports.resolved_fiscal_value"`
	CreatedAt               time.Time        `json:"created_at" column:"damage_reports.created_at"`
	UpdatedAt               time.Time        `json:"updated_at" column:"damage_reports.updated_at"`
}
