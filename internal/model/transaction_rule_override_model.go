package model

import (
	"encoding/json"
	"time"
)

// TransactionRuleOverride is a Group, Asset Name or Asset rule override. NULL CategoryID is an
// All categories override; TargetName, CategoryPath and TargetGroupID are creation-time snapshots.
type TransactionRuleOverride struct {
	ID              int64           `json:"id" column:"transaction_rule_overrides.id"`
	CategoryID      *int64          `json:"category_id" column:"transaction_rule_overrides.category_id"`
	TransactionType string          `json:"transaction_type" column:"transaction_rule_overrides.transaction_type"`
	Scope           string          `json:"scope" column:"transaction_rule_overrides.scope"`
	TargetID        int64           `json:"target_id" column:"transaction_rule_overrides.target_id"`
	TargetName      string          `json:"target_name" column:"transaction_rule_overrides.target_name"`
	CategoryPath    *string         `json:"category_path" column:"transaction_rule_overrides.category_path"`
	TargetGroupID   *int64          `json:"target_group_id" column:"transaction_rule_overrides.target_group_id"`
	Fields          json.RawMessage `json:"fields" column:"transaction_rule_overrides.fields"`
	CreatedByID     int64           `json:"created_by_id" column:"transaction_rule_overrides.created_by_id"`
	CreatedByName   string          `json:"created_by_name" column:"transaction_rule_overrides.created_by_name"`
	CreatedAt       time.Time       `json:"created_at" column:"transaction_rule_overrides.created_at"`
}
