package dto

import "fmt"

const (
	MasterChangeTypeGroup    = "group"
	MasterChangeTypeCategory = "category"
	MasterChangeTypeBrand    = "brand"
)

// MasterChangesRequest is the validated internal form of the owner event.
// The handler parses Snowflake IDs from decimal strings into int64 values.
type MasterChangesRequest struct {
	Type         string
	Action       string
	IDs          []int64
	MergedIntoID *int64
}

// MasterChangesValidationError describes one invalid field.
type MasterChangesValidationError struct {
	Field   string
	Message string
}

// MasterChangesData is the successful response payload.
type MasterChangesData struct {
	Affected int64 `json:"affected"`
}

// Validate checks master change action and Snowflake ID constraints.
func (r MasterChangesRequest) Validate() []MasterChangesValidationError {
	var validationErrors []MasterChangesValidationError

	allowedActions, ok := masterChangeActions[r.Type]
	if !ok {
		validationErrors = append(validationErrors, MasterChangesValidationError{
			Field:   "type",
			Message: "must be one of: group, category, brand",
		})
	} else if !allowedActions[r.Action] {
		validationErrors = append(validationErrors, MasterChangesValidationError{
			Field:   "action",
			Message: fmt.Sprintf("is not allowed for type %s", r.Type),
		})
	}

	if len(r.IDs) == 0 {
		validationErrors = append(validationErrors, MasterChangesValidationError{
			Field:   "ids",
			Message: "must contain at least one Snowflake ID",
		})
	} else {
		for i, id := range r.IDs {
			if id <= 0 {
				validationErrors = append(validationErrors, MasterChangesValidationError{
					Field:   fmt.Sprintf("ids[%d]", i),
					Message: "must be a positive Snowflake ID",
				})
			}
		}
	}

	if r.Action == "merged" {
		if r.Type != MasterChangeTypeBrand {
			validationErrors = append(validationErrors, MasterChangesValidationError{
				Field:   "action",
				Message: "merged is allowed only for type brand",
			})
		}
		if r.MergedIntoID == nil {
			validationErrors = append(validationErrors, MasterChangesValidationError{
				Field:   "mergedIntoId",
				Message: "is required when action is merged",
			})
		} else if *r.MergedIntoID <= 0 {
			validationErrors = append(validationErrors, MasterChangesValidationError{
				Field:   "mergedIntoId",
				Message: "must be a positive Snowflake ID",
			})
		}
	} else if r.MergedIntoID != nil {
		validationErrors = append(validationErrors, MasterChangesValidationError{
			Field:   "mergedIntoId",
			Message: "must be null unless action is merged",
		})
	}

	return validationErrors
}

// IsNoOp identifies owner changes that leave Service Center references unchanged.
func (r MasterChangesRequest) IsNoOp() bool {
	switch r.Type {
	case MasterChangeTypeGroup, MasterChangeTypeCategory:
		return r.Action == "moved" || r.Action == "renamed"
	case MasterChangeTypeBrand:
		return r.Action == "deactivated" || r.Action == "renamed"
	default:
		return false
	}
}

var masterChangeActions = map[string]map[string]bool{
	MasterChangeTypeGroup: {
		"deleted":     true,
		"moved":       true,
		"deactivated": true,
		"archived":    true,
		"renamed":     true,
	},
	MasterChangeTypeCategory: {
		"deleted": true,
		"moved":   true,
		"renamed": true,
	},
	MasterChangeTypeBrand: {
		"deleted":     true,
		"deactivated": true,
		"merged":      true,
		"renamed":     true,
	},
}
