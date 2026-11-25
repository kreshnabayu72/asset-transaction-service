package dto

import "encoding/json"

// SaveField is one changed field of a node; Value is raw so the service can type-check it.
type SaveField struct {
	Key   string          `json:"key" validate:"required"`
	Mode  string          `json:"mode" validate:"required,oneof=inherit own"`
	Value json.RawMessage `json:"value"`
}

// SaveChange is the changed fields of one transaction at one node (nil CategoryID = All categories).
type SaveChange struct {
	CategoryID  *string     `json:"categoryId"`
	Transaction string      `json:"transaction" validate:"required,oneof=borrow assign transfer disposal damaged_missing audit maintenance"`
	Fields      []SaveField `json:"fields" validate:"required,min=1,dive"`
}

// SaveSettingsRequest is the body of PATCH /transaction-settings.
type SaveSettingsRequest struct {
	Changes []SaveChange `json:"changes" validate:"required,min=1,dive"`
}

// FieldError is one entry of the 422 error list.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}
