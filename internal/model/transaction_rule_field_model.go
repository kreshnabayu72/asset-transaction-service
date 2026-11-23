package model

import (
	"encoding/json"
	"time"
)

// TransactionRuleField is one rule value. NULL CategoryID is the authoritative Company default.
type TransactionRuleField struct {
	ID              int64            `json:"id,string" column:"transaction_rule_fields.id"`
	CategoryID      *int64           `json:"category_id,string" column:"transaction_rule_fields.category_id"`
	TransactionType string           `json:"transaction_type" column:"transaction_rule_fields.transaction_type"`
	FieldKey        string           `json:"field_key" column:"transaction_rule_fields.field_key"`
	Mode            string           `json:"mode" column:"transaction_rule_fields.mode"`
	Value           *json.RawMessage `json:"value" column:"transaction_rule_fields.value"`
	ModifiedByID    *int64           `json:"modified_by_id,string" column:"transaction_rule_fields.modified_by_id"`
	ModifiedByName  *string          `json:"modified_by_name" column:"transaction_rule_fields.modified_by_name"`
	CreatedAt       time.Time        `json:"created_at" column:"transaction_rule_fields.created_at"`
	UpdatedAt       time.Time        `json:"updated_at" column:"transaction_rule_fields.updated_at"`
}
