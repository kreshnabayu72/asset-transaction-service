package dto

// EffectiveRulesRequest is the body of POST /transaction-settings/effective-rules.
type EffectiveRulesRequest struct {
	Transaction string   `json:"transaction" validate:"required,oneof=borrow assign transfer disposal damaged_missing audit maintenance"`
	AssetIDs    []string `json:"assetIds" validate:"required,min=1,dive,required"`
}

// DenyContact is who to contact when an Allow transaction field resolves to Deny.
type DenyContact struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

// EffectiveField is one resolved field of one asset.
type EffectiveField struct {
	Key                 string `json:"key"`
	Value               any    `json:"value"`
	SourceLevel         string `json:"sourceLevel"`
	SourceName          string `json:"sourceName"`
	SourceCategoryScope string `json:"sourceCategoryScope"`
	Gated               bool   `json:"gated"`
}

// EffectiveAsset is the effective rules of one asset for one transaction.
type EffectiveAsset struct {
	AssetID         string           `json:"assetId"`
	SettingsVersion int              `json:"settingsVersion"`
	Allowed         bool             `json:"allowed"`
	DenyContact     *DenyContact     `json:"denyContact"`
	Fields          []EffectiveField `json:"fields"`
}
