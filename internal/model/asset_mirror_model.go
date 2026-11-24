package model

import "time"

// AssetMirror is the damaged module's read copy of master-asset-registry-service assets.
type AssetMirror struct {
	ID               int64      `json:"id,string" column:"assets.id"`
	AssetCode        *string    `json:"asset_code" column:"assets.asset_code"`
	AssetNameID      *int64     `json:"asset_name_id,string" column:"assets.asset_name_id"`
	AssetName        *string    `json:"asset_name" column:"assets.asset_name"`
	AssetIncrement   *int64     `json:"asset_increment" column:"assets.asset_increment"`
	AssetNameModelID *int64     `json:"asset_name_model_id,string" column:"assets.asset_name_model_id"`
	BrandID          *int64     `json:"brand_id,string" column:"assets.brand_id"`
	BrandName        *string    `json:"brand_name" column:"assets.brand_name"`
	DivisionID       *int64     `json:"division_id,string" column:"assets.division_id"`
	DivisionName     *string    `json:"division_name" column:"assets.division_name"`
	GroupID          *int64     `json:"group_id,string" column:"assets.group_id"`
	GroupName        *string    `json:"group_name" column:"assets.group_name"`
	PurchaseDate     *time.Time `json:"purchase_date" column:"assets.purchase_date"`
	PurchasePrice    *float64   `json:"purchase_price" column:"assets.purchase_price"` // NUMERIC(18,2) as float64, as in master-asset-registry-service
	WarrantyExpiry   *time.Time `json:"warranty_expiry" column:"assets.warranty_expiry"`
	LifecycleStatus  string     `json:"lifecycle_status" column:"assets.lifecycle_status"`
	CustodyStatus    string     `json:"custody_status" column:"assets.custody_status"`
	HolderID         *int64     `json:"holder_id,string" column:"assets.holder_id"`
	Condition        string     `json:"condition" column:"assets.condition"`
	RegisteredAt     *time.Time `json:"registered_at" column:"assets.registered_at"`
	UpdatedAt        time.Time  `json:"updated_at" column:"assets.updated_at"`
}
