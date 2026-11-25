package constants

// Override scopes (OpenAPI CreateOverridesRequest.scope).
const (
	ScopeGroup     = "group"
	ScopeAssetName = "asset_name"
	ScopeAsset     = "asset"
)

// ScopeLabels are the display names of the override scopes (list filter options, sort and search).
var ScopeLabels = map[string]string{
	ScopeGroup:     "Group",
	ScopeAssetName: "Asset Name",
	ScopeAsset:     "Asset",
}

// ScopeOrder is the fixed order of the scopes.
var ScopeOrder = []string{ScopeGroup, ScopeAssetName, ScopeAsset}

// OverrideTransactions are the transactions that carry overrides (not damaged_missing, audit, maintenance).
var OverrideTransactions = []string{TransactionBorrow, TransactionAssign, TransactionTransfer, TransactionDisposal}

// Reasons a list row is not applied, and why a delete row is skipped.
const (
	NotAppliedRemoved = "target_or_category_removed"
	NotAppliedPlan    = "plan"
	SkipOutOfScope    = "out_of_scope"
	SkipAssigneeGuard = "assignee_guard"
)

// CategoryAll is the Category display of an override stored at All categories.
const CategoryAll = NodeAllCategories

// Override list sort keys and the maximum page sizes (LLD list step 5-6).
var (
	OverrideSortKeys = []string{"scope", "target", "overridden", "createdBy", "category", "createdAt"}
	PageSizes        = []int{10, 25, 50, 100}
)
