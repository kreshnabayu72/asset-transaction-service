package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type CheckEligibilityRequest struct {
	ServiceCenterID int64
	ServiceType     string
	Assets          []EligibilityAsset
}

type EligibilityAsset struct {
	GroupID    int64
	CategoryID int64
	BrandID    *int64
}

type EligibilitySnapshot struct {
	Name         string
	IsActive     bool
	ServiceTypes []string
	GroupIDs     []int64
	CategoryIDs  []int64
	BrandIDs     []int64
}

type CheckEligibilityResponse struct {
	Eligible bool    `json:"eligible"`
	Reason   *string `json:"reason"`
	Message  *string `json:"message"`
}

type EligibilityValidationError struct {
	Field   string
	Message string
}

func ParseCheckEligibilityRequest(body []byte) (CheckEligibilityRequest, []EligibilityValidationError, error) {
	var request CheckEligibilityRequest
	if !json.Valid(body) {
		return request, nil, errors.New("invalid JSON")
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil || raw == nil {
		return request, []EligibilityValidationError{{Field: "", Message: "must be a JSON object"}}, nil
	}

	var validationErrors []EligibilityValidationError

	serviceCenterID, ok := parseRequiredID(raw, "serviceCenterId", &validationErrors)
	if ok {
		request.ServiceCenterID = serviceCenterID
	}

	serviceType, ok := parseRequiredString(raw, "serviceType", &validationErrors)
	if ok {
		if serviceType != "Routine" && serviceType != "Repair" {
			validationErrors = append(validationErrors, EligibilityValidationError{Field: "serviceType", Message: "must be Routine or Repair"})
		} else {
			request.ServiceType = serviceType
		}
	}

	assetsRaw, exists := raw["assets"]
	if !exists {
		validationErrors = append(validationErrors, EligibilityValidationError{Field: "assets", Message: "is required"})
	} else {
		var assetValues []json.RawMessage
		if err := json.Unmarshal(assetsRaw, &assetValues); err != nil || assetValues == nil {
			validationErrors = append(validationErrors, EligibilityValidationError{Field: "assets", Message: "must be an array"})
		} else if len(assetValues) == 0 {
			validationErrors = append(validationErrors, EligibilityValidationError{Field: "assets", Message: "must contain at least one asset"})
		} else {
			request.Assets = make([]EligibilityAsset, 0, len(assetValues))
			for i, assetRaw := range assetValues {
				fieldPrefix := fmt.Sprintf("assets[%d]", i)
				var assetObject map[string]json.RawMessage
				if err := json.Unmarshal(assetRaw, &assetObject); err != nil || assetObject == nil {
					validationErrors = append(validationErrors, EligibilityValidationError{Field: fieldPrefix, Message: "must be a JSON object"})
					continue
				}

				var asset EligibilityAsset
				if id, valid := parseRequiredID(assetObject, "groupId", &validationErrors); valid {
					asset.GroupID = id
				} else {
					rewriteLastErrorField(&validationErrors, "groupId", fieldPrefix+".groupId")
				}
				if id, valid := parseRequiredID(assetObject, "categoryId", &validationErrors); valid {
					asset.CategoryID = id
				} else {
					rewriteLastErrorField(&validationErrors, "categoryId", fieldPrefix+".categoryId")
				}
				if id, valid := parseRequiredNullableID(assetObject, "brandId", &validationErrors); valid {
					asset.BrandID = id
				} else {
					rewriteLastErrorField(&validationErrors, "brandId", fieldPrefix+".brandId")
				}
				request.Assets = append(request.Assets, asset)
			}
		}
	}

	return request, validationErrors, nil
}

func parseRequiredString(object map[string]json.RawMessage, field string, validationErrors *[]EligibilityValidationError) (string, bool) {
	raw, exists := object[field]
	if !exists {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "is required"})
		return "", false
	}

	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "must be a string"})
		return "", false
	}
	if strings.TrimSpace(value) == "" {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "is required"})
		return "", false
	}
	return value, true
}

func parseRequiredID(object map[string]json.RawMessage, field string, validationErrors *[]EligibilityValidationError) (int64, bool) {
	raw, exists := object[field]
	if !exists {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "is required"})
		return 0, false
	}
	return parseID(raw, field, validationErrors)
}

func parseRequiredNullableID(object map[string]json.RawMessage, field string, validationErrors *[]EligibilityValidationError) (*int64, bool) {
	raw, exists := object[field]
	if !exists {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "is required"})
		return nil, false
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, true
	}
	id, ok := parseID(raw, field, validationErrors)
	if !ok {
		return nil, false
	}
	return &id, true
}

func parseID(raw json.RawMessage, field string, validationErrors *[]EligibilityValidationError) (int64, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "must be a Snowflake ID string"})
		return 0, false
	}
	if value == "" {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "must be a Snowflake ID string"})
		return 0, false
	}
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "must be a Snowflake ID string"})
			return 0, false
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		*validationErrors = append(*validationErrors, EligibilityValidationError{Field: field, Message: "must be a positive Snowflake ID"})
		return 0, false
	}
	return id, true
}

func rewriteLastErrorField(validationErrors *[]EligibilityValidationError, suffix, field string) {
	for i := len(*validationErrors) - 1; i >= 0; i-- {
		if (*validationErrors)[i].Field == suffix {
			(*validationErrors)[i].Field = field
			return
		}
	}
}
