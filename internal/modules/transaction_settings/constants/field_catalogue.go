// Package constants holds the transaction settings field catalogue (PRD 4.1, LLD field catalogue).
package constants

// Transaction keys in tab order (OpenAPI `Transaction` enum).
const (
	TransactionBorrow         = "borrow"
	TransactionAssign         = "assign"
	TransactionTransfer       = "transfer"
	TransactionDisposal       = "disposal"
	TransactionDamagedMissing = "damaged_missing"
	TransactionAudit          = "audit"
	TransactionMaintenance    = "maintenance"
)

// TransactionOrder is the fixed response order (LLD GET step 10).
var TransactionOrder = []string{
	TransactionBorrow,
	TransactionAssign,
	TransactionTransfer,
	TransactionDisposal,
	TransactionDamagedMissing,
	TransactionAudit,
	TransactionMaintenance,
}

// AllCategoriesOnlyTransactions exist only at All categories (no Category value, Inherit or override).
var AllCategoriesOnlyTransactions = []string{TransactionDamagedMissing, TransactionAudit}

// Field keys (OpenAPI `SettingField.key`).
const (
	FieldAllowBorrow                = "allow_borrow"
	FieldAllowAssign                = "allow_assign"
	FieldAllowTransfer              = "allow_transfer"
	FieldAllowDisposal              = "allow_disposal"
	FieldDurationUnit               = "duration_unit"
	FieldMaxDuration                = "max_duration"
	FieldMaxExtensions              = "max_extensions"
	FieldMaxAssetsPerBorrower       = "max_assets_per_borrower"
	FieldAllowedBorrower            = "allowed_borrower"
	FieldBlockNewBorrowWhenOverdue  = "block_new_borrow_when_overdue"
	FieldAllowedAssigneeEmployee    = "allowed_assignee_employee"
	FieldAllowedAssigneeDivision    = "allowed_assignee_division"
	FieldMaxAssetsPerAssignee       = "max_assets_per_assignee"
	FieldNeedRecipientConfirmation  = "need_recipient_confirmation"
	FieldDestinationScope           = "destination_scope"
	FieldChangeCostCenterWithGroup  = "change_cost_center_with_group"
	FieldInTransitOverdueAfter      = "in_transit_overdue_after"
	FieldDestinationQuotaCheck      = "destination_quota_check"
	FieldAllowedDisposalTypes       = "allowed_disposal_types"
	FieldMinAssetAgeWarning         = "min_asset_age_warning"
	FieldMaxDeviationFromBookValue  = "max_deviation_from_book_value"
	FieldLongMissingAfter           = "long_missing_after"
	FieldPhotoAtStart               = "photo_at_start"
	FieldPhotoAtEnd                 = "photo_at_end"
	FieldAfterPhotoRequired         = "after_photo_required"
	FieldTransactionTimeout         = "transaction_timeout"
	FieldDueDateReminders           = "due_date_reminders"
	FieldRequireApproval            = "require_approval"
	FieldApprovalType               = "approval_type"
	FieldApprovalScope              = "approval_scope"
	FieldSelfService                = "self_service"
	FieldAutoApprove                = "auto_approve"
	FieldAutoApproveMaxDuration     = "auto_approve_max_duration"
	FieldAutoApproveSameParentGroup = "auto_approve_same_parent_group"
	FieldSendEmailNotifications     = "send_email_notifications"
	FieldGracePeriod                = "grace_period"
	FieldRequireLocation            = "require_location"
	FieldReminderBeforeDue          = "reminder_before_due"
	FieldPhotoAfter                 = "photo_after"
	FieldRequireApprovalToClose     = "require_approval_to_close"
	FieldRequireScanToStart         = "require_scan_to_start"
)

// Choice-field tokens (OpenAPI `FieldValue` value catalogue; lowercase snake_case of the option label).
const (
	ValueDays                  = "days"
	ValueHours                 = "hours"
	ValueSameGroupOnly         = "same_group_only"
	ValueSameDivisionOnly      = "same_division_only"
	ValueEmployeeInAnyGroup    = "employee_in_any_group"
	ValueWithinSameParentGroup = "within_same_parent_group"
	ValueAnyGroup              = "any_group"
	ValueNever                 = "never"
	ValueAlways                = "always"
	ValueAnyOneApprover        = "any_one_approver"
	ValueAllApprovers          = "all_approvers"
	ValueOriginOnly            = "origin_only"
	ValueDestinationOnly       = "destination_only"
	ValueBoth                  = "both"
	ValueRemindersD3D1Dday     = "d_3_d_1_d_day"
	ValueRemindersD1Dday       = "d_1_d_day"
	ValueRemindersDdayOnly     = "d_day_only"
)

// ValueLabels maps a choice token or disposal type to its display label (export cells).
var ValueLabels = map[string]string{
	ValueDays:                  "Days",
	ValueHours:                 "Hours",
	ValueSameGroupOnly:         "Same Group only",
	ValueSameDivisionOnly:      "Same Division only",
	ValueEmployeeInAnyGroup:    "Employee in any Group",
	ValueWithinSameParentGroup: "Within same parent Group",
	ValueAnyGroup:              "Any Group",
	ValueNever:                 "Never",
	ValueAlways:                "Always",
	ValueAnyOneApprover:        "Any one approver",
	ValueAllApprovers:          "All approvers (AND)",
	ValueOriginOnly:            "Origin only",
	ValueDestinationOnly:       "Destination only",
	ValueBoth:                  "Both",
	ValueRemindersD3D1Dday:     "D-3, D-1, D-day",
	ValueRemindersD1Dday:       "D-1, D-day",
	ValueRemindersDdayOnly:     "D-day only",
	"sell":                     "Sell",
	"scrap":                    "Scrap",
	"donate":                   "Donate",
	"trade_in":                 "Trade-in",
	"write_off":                "Write-off",
}

// DisposalTypes lists every allowed disposal type token in option order (default: all selected).
var DisposalTypes = []string{"sell", "scrap", "donate", "trade_in", "write_off"}

// DurationUnit is the Unit sentinel of a Duration-bound number: the unit is the node's duration_unit.
const DurationUnit = "duration"

// Export label pairs (true, false) of boolean fields other than the default Active/Inactive.
var (
	LabelsAllowDeny = [2]string{"Allow", "Deny"}
	LabelsYesNo     = [2]string{"Yes", "No"}
)

// FieldDef describes one field of one transaction.
type FieldDef struct {
	Key     string
	Default any
	Unit    string // export unit label; empty when none; DurationUnit when bound to duration_unit
	// AllCategoriesOnly fields have no Category-level value or Inherit mode.
	AllCategoriesOnly bool
	// BoolLabels overrides the Active/Inactive export labels of a boolean field.
	BoolLabels [2]string
}

// Catalogue lists the fields of each transaction, in OpenAPI key order, with Company defaults.
var Catalogue = map[string][]FieldDef{
	TransactionBorrow: {
		{Key: FieldAllowBorrow, Default: true, BoolLabels: LabelsAllowDeny},
		{Key: FieldDurationUnit, Default: ValueDays},
		{Key: FieldMaxDuration, Default: 14, Unit: DurationUnit},
		{Key: FieldMaxExtensions, Default: 1},
		{Key: FieldMaxAssetsPerBorrower, Default: 3},
		{Key: FieldAllowedBorrower, Default: ValueEmployeeInAnyGroup},
		{Key: FieldBlockNewBorrowWhenOverdue, Default: true},
		{Key: FieldRequireApproval, Default: true},
		{Key: FieldSelfService, Default: false},
		{Key: FieldAutoApprove, Default: true},
		{Key: FieldAutoApproveMaxDuration, Default: 3, Unit: DurationUnit},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover},
		{Key: FieldDueDateReminders, Default: ValueRemindersD3D1Dday},
		{Key: FieldPhotoAtStart, Default: true},
		{Key: FieldPhotoAtEnd, Default: true},
		{Key: FieldTransactionTimeout, Default: 7, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldSendEmailNotifications, Default: true, AllCategoriesOnly: true},
	},
	TransactionAssign: {
		{Key: FieldAllowAssign, Default: true, BoolLabels: LabelsAllowDeny},
		{Key: FieldAllowedAssigneeEmployee, Default: true},
		{Key: FieldAllowedAssigneeDivision, Default: true},
		{Key: FieldMaxAssetsPerAssignee, Default: 0},
		{Key: FieldNeedRecipientConfirmation, Default: true, BoolLabels: LabelsYesNo},
		{Key: FieldRequireApproval, Default: false},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover},
		{Key: FieldPhotoAtStart, Default: true},
		{Key: FieldPhotoAtEnd, Default: true},
		{Key: FieldTransactionTimeout, Default: 7, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldSendEmailNotifications, Default: false, AllCategoriesOnly: true},
	},
	TransactionTransfer: {
		{Key: FieldAllowTransfer, Default: true, BoolLabels: LabelsAllowDeny},
		{Key: FieldDestinationScope, Default: ValueAnyGroup},
		{Key: FieldChangeCostCenterWithGroup, Default: ValueNever, AllCategoriesOnly: true},
		{Key: FieldInTransitOverdueAfter, Default: 7, Unit: "days"},
		{Key: FieldDestinationQuotaCheck, Default: true},
		{Key: FieldRequireApproval, Default: true},
		{Key: FieldAutoApprove, Default: true},
		{Key: FieldAutoApproveSameParentGroup, Default: true},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover},
		{Key: FieldApprovalScope, Default: ValueBoth},
		{Key: FieldPhotoAtStart, Default: true},
		{Key: FieldPhotoAtEnd, Default: true},
		{Key: FieldTransactionTimeout, Default: 7, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldSendEmailNotifications, Default: false, AllCategoriesOnly: true},
	},
	TransactionDisposal: {
		{Key: FieldAllowDisposal, Default: true, BoolLabels: LabelsAllowDeny},
		{Key: FieldAllowedDisposalTypes, Default: DisposalTypes},
		{Key: FieldMinAssetAgeWarning, Default: 12, Unit: "months"},
		{Key: FieldMaxDeviationFromBookValue, Default: 30, Unit: "%"},
		{Key: FieldRequireApproval, Default: true},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover},
		{Key: FieldTransactionTimeout, Default: 7, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldSendEmailNotifications, Default: false, AllCategoriesOnly: true},
	},
	TransactionDamagedMissing: {
		{Key: FieldLongMissingAfter, Default: 30, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldAfterPhotoRequired, Default: false, AllCategoriesOnly: true},
	},
	TransactionAudit: {
		{Key: FieldGracePeriod, Default: 3, Unit: "days", AllCategoriesOnly: true},
		{Key: FieldRequireApproval, Default: true, AllCategoriesOnly: true},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover, AllCategoriesOnly: true},
		{Key: FieldSendEmailNotifications, Default: false, AllCategoriesOnly: true},
		{Key: FieldRequireLocation, Default: false, AllCategoriesOnly: true},
	},
	TransactionMaintenance: {
		{Key: FieldReminderBeforeDue, Default: 3, Unit: "days"},
		{Key: FieldPhotoAfter, Default: true},
		{Key: FieldRequireApprovalToClose, Default: false},
		{Key: FieldRequireScanToStart, Default: false},
		{Key: FieldApprovalType, Default: ValueAnyOneApprover},
		{Key: FieldRequireLocation, Default: false, AllCategoriesOnly: true},
	},
}

// ApprovalWorkflowGatedFields are unavailable while the plan has no Approval Workflow (every transaction).
var ApprovalWorkflowGatedFields = []string{
	FieldRequireApproval,
	FieldAutoApprove,
	FieldAutoApproveMaxDuration,
	FieldAutoApproveSameParentGroup,
	FieldApprovalType,
	FieldApprovalScope,
	FieldRequireApprovalToClose,
}

// BasicGatedFields lists, per transaction, the fields unavailable at Feature Suite level Basic
// (Disposal gates none).
var BasicGatedFields = map[string][]string{
	TransactionBorrow:   {FieldSelfService, FieldDurationUnit},
	TransactionAssign:   {FieldAllowedAssigneeEmployee, FieldAllowedAssigneeDivision},
	TransactionTransfer: {FieldApprovalScope, FieldInTransitOverdueAfter, FieldPhotoAtEnd},
}

// Feature Suite levels (wire values).
const (
	LevelNone  = "none"
	LevelBasic = "basic"
	LevelFull  = "full"
)

// CompanyDefaultSource is the source text of an inherited value.
const CompanyDefaultSource = "Company default"

// Node and mode literals.
const (
	NodeAllCategories = "All categories"
	ModeInherit       = "inherit"
	ModeOwn           = "own"
)
