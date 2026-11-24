package model

import "time"

// TransactionSettings is the singleton row in each tenant schema. It holds only the settings version
// and audit fields; rule values live in TransactionRuleField and TransactionRuleOverride.
type TransactionSettings struct {
	ID              int64     `json:"id,string" column:"transaction_settings.id"`
	SettingsVersion int64     `json:"settings_version" column:"transaction_settings.settings_version"`
	ModifiedByID    *int64    `json:"modified_by_id,string" column:"transaction_settings.modified_by_id"`
	ModifiedByName  *string   `json:"modified_by_name" column:"transaction_settings.modified_by_name"`
	CreatedAt       time.Time `json:"created_at" column:"transaction_settings.created_at"`
	UpdatedAt       time.Time `json:"updated_at" column:"transaction_settings.updated_at"`
}
