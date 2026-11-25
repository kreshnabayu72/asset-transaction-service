package dto

import (
	"encoding/json"
	"time"
)

// CreateOverrideField is one field of an override; Value is raw so the service can type-check it.
type CreateOverrideField struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

// CreateOverrideRequest is the body of POST /transaction-settings/overrides. Validation literals
// (LLD create step 3) come from the service, so nothing here is tagged required.
type CreateOverrideRequest struct {
	Scope       string                `json:"scope"`
	TargetIDs   []string              `json:"targetIds"`
	CategoryID  *string               `json:"categoryId"`
	Transaction string                `json:"transaction"`
	Fields      []CreateOverrideField `json:"fields"`
}

// DeleteOverridesRequest is the body of DELETE /transaction-settings/overrides.
type DeleteOverridesRequest struct {
	IDs []string `json:"ids"`
}

// SkippedOverride is a row a delete did not remove.
type SkippedOverride struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// DeleteOverridesData is data of the 200 and 409 delete answers.
type DeleteOverridesData struct {
	DeletedIDs []string          `json:"deletedIds"`
	Skipped    []SkippedOverride `json:"skipped"`
}

// DateRangeFilter is the createdAt range in UTC epoch milliseconds.
type DateRangeFilter struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// ListOverridesRequest is the body of POST /transaction-settings/overrides/list. Paging, sort and
// the date range are pointers or raw so a missing or malformed value answers 422, not 400.
type ListOverridesRequest struct {
	Page        *int            `json:"page"`
	Limit       *int            `json:"limit"`
	SortBy      *string         `json:"sortBy"`
	SortOrder   *int            `json:"sortOrder"`
	Columns     []string        `json:"columns"`
	Search      string          `json:"search"`
	CategoryID  *string         `json:"categoryId"`
	Transaction string          `json:"transaction"`
	Scope       []string        `json:"scope"`
	Overridden  []string        `json:"overridden"`
	CreatedBy   []string        `json:"createdBy"`
	Category    []string        `json:"category"`
	CreatedAt   json.RawMessage `json:"createdAt"`
}

// OverriddenField is one changed field of an override row.
type OverriddenField struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Value        any    `json:"value"`
	DisplayValue string `json:"displayValue"`
}

// OverrideRow is one row of the overrides list.
type OverrideRow struct {
	ID               string            `json:"_id"`
	Scope            string            `json:"scope"`
	Target           string            `json:"target"`
	Category         string            `json:"category"`
	Transaction      string            `json:"transaction"`
	Overridden       []OverriddenField `json:"overridden"`
	Applied          bool              `json:"applied"`
	NotAppliedReason *string           `json:"notAppliedReason"`
	CreatedBy        string            `json:"createdBy"`
	CreatedAt        time.Time         `json:"createdAt"`
	CanDelete        bool              `json:"canDelete"`
}

// ListOverridesData is data of the list answer.
type ListOverridesData struct {
	Data         []OverrideRow `json:"data"`
	TotalRecords int           `json:"totalRecords"`
}

// OverridesOptionsRequest is the body of POST /transaction-settings/overrides/options.
type OverridesOptionsRequest struct {
	CategoryID  *string         `json:"categoryId"`
	Transaction string          `json:"transaction"`
	Scope       bool            `json:"scope"`
	Overridden  bool            `json:"overridden"`
	CreatedBy   bool            `json:"createdBy"`
	Category    []string        `json:"category"`
	CreatedAt   json.RawMessage `json:"createdAt"`
}

// OptionItem is one filter option; display name in both fields.
type OptionItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// OverridesOptionsData carries only the arrays that were requested.
type OverridesOptionsData struct {
	Scope      *[]OptionItem `json:"scope,omitempty"`
	Overridden *[]OptionItem `json:"overridden,omitempty"`
	CreatedBy  *[]OptionItem `json:"createdBy,omitempty"`
}

// DuplicateOverrideData is data of the 409 create answer.
type DuplicateOverrideData struct {
	DuplicateTargetIDs []string `json:"duplicateTargetIds"`
}

// OverrideQuery is the resolved filter of an overrides list or options read.
type OverrideQuery struct {
	Transaction string
	CategoryID  *int64 // nil = All categories node (every override of the transaction)
	Search      string
	// SearchColumns are the searchable columns among the requested ones: scope, target,
	// overridden, createdBy, category.
	SearchColumns []string
	// SearchFieldKeys are the field keys whose display name contains the search text.
	SearchFieldKeys []string
	Scopes          []string
	OverriddenKeys  []string
	CreatedBy       []string
	CategoryIDs     []int64
	CreatedFrom     *int64
	CreatedTo       *int64
	// VisibleGroupIDs restricts the rows to Group overrides of these Groups and Asset overrides of
	// assets in them; nil = every row (an empty non-nil slice = no row).
	VisibleGroupIDs *[]int64
	SortBy          string
	SortOrder       int
	Page            int
	Limit           int
	// Option flags (options read only).
	WantScope, WantOverridden, WantCreatedBy bool
}

// OverrideListRecord is one raw row of the list query.
type OverrideListRecord struct {
	ID          string          `json:"_id"`
	Scope       string          `json:"scope"`
	Target      string          `json:"target"`
	Category    string          `json:"category"`
	Transaction string          `json:"transaction"`
	Fields      json.RawMessage `json:"fields"`
	TargetID    string          `json:"targetId"`
	CategoryID  *string         `json:"categoryId"`
	GroupID     *string         `json:"groupId"`
	CreatedBy   string          `json:"createdBy"`
	CreatedAt   time.Time       `json:"createdAt"`
}

// OptionRecord is one distinct value of an options read.
type OptionRecord struct {
	Value string `json:"value"`
}

// DuplicateRecord is the target of an existing active override.
type DuplicateRecord struct {
	TargetID int64 `json:"target_id" column:"target_id"`
}
