package model

import "time"

// AssetAttribute is the transaction-scoped display row per asset; the damaged module uses transaction_type 'damaged'.
type AssetAttribute struct {
	ID               int64     `json:"id,string" column:"asset_attributes.id"`
	AssetID          int64     `json:"asset_id,string" column:"asset_attributes.asset_id"`
	TransactionType  string    `json:"transaction_type" column:"asset_attributes.transaction_type"`
	NameWithSequence *string   `json:"name_with_sequence" column:"asset_attributes.name_with_sequence"`
	BrandName        *string   `json:"brand_name" column:"asset_attributes.brand_name"`
	ModelName        *string   `json:"model_name" column:"asset_attributes.model_name"`
	CategoryName     *string   `json:"category_name" column:"asset_attributes.category_name"`
	AliasName        *string   `json:"alias_name" column:"asset_attributes.alias_name"`
	Status           *string   `json:"status" column:"asset_attributes.status"`
	GroupID          *int64    `json:"group_id,string" column:"asset_attributes.group_id"`
	GroupName        *string   `json:"group_name" column:"asset_attributes.group_name"`
	IsDeleted        bool      `json:"is_deleted" column:"asset_attributes.is_deleted"`
	CreatedAt        time.Time `json:"created_at" column:"asset_attributes.created_at"`
	UpdatedAt        time.Time `json:"updated_at" column:"asset_attributes.updated_at"`
}
