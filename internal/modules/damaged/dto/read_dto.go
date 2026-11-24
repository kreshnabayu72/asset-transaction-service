// Package dto holds the response data shapes of the damaged module read operations.
package dto

import "time"

// SummaryData is the data of GET /v2/asset-transaction/damaged/summary. BadgeCount is nil when the
// caller is not Aktor Damaged on any group.
type SummaryData struct {
	BadgeCount *int `json:"badgeCount"`
}

// RunningRecordData is the data of GET /v2/asset-transaction/damaged/assets/:assetId/running-record.
type RunningRecordData struct {
	ID            int64     `json:"_id,string"`
	Status        string    `json:"status"`
	MissingMarker *string   `json:"missingMarker"`
	ReportedAt    time.Time `json:"reportedAt"`
}
