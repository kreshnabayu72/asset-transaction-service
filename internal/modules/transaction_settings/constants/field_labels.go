package constants

// TransactionLabels are the display names of a transaction in log entries.
var TransactionLabels = map[string]string{
	TransactionBorrow:         "Borrow",
	TransactionAssign:         "Assign",
	TransactionTransfer:       "Transfer",
	TransactionDisposal:       "Disposal",
	TransactionDamagedMissing: "Damaged & Missing",
	TransactionAudit:          "Audit",
	TransactionMaintenance:    "Maintenance",
}

// FieldLabels are the display names of a field (PRD 4.1 / LLD field catalogue) used in Changelog and Activity Log entries.
var FieldLabels = map[string]string{
	FieldAllowBorrow:                  "Allow Borrow",
	FieldAllowAssign:                  "Allow Assign",
	FieldAllowTransfer:                "Allow Transfer",
	FieldAllowDisposal:                "Allow Disposal",
	FieldDurationUnit:                 "Duration unit",
	FieldMaxDuration:                  "Max duration",
	FieldMaxExtensions:                "Max extensions",
	FieldMaxAssetsPerBorrower:         "Max assets per borrower (concurrent)",
	FieldAllowedBorrower:              "Allowed borrower",
	FieldBlockNewBorrowWhenOverdue:    "Block new borrow when overdue",
	FieldAllowedAssigneeEmployee:      "Allowed assignee: Employee",
	FieldAllowedAssigneeDivision:      "Allowed assignee: Division",
	FieldMaxAssetsPerAssignee:         "Max assets per assignee (concurrent)",
	FieldNeedRecipientConfirmation:    "Need recipient confirmation",
	FieldDirectHandoverBetweenHolders: "Direct handover between holders",
	FieldDestinationScope:             "Destination scope",
	FieldChangeCostCenterWithGroup:    "Change Cost Center with Group",
	FieldInTransitOverdueAfter:        "In-transit overdue after",
	FieldDestinationQuotaCheck:        "Destination quota check",
	FieldAllowedDisposalTypes:         "Allowed disposal types",
	FieldMinAssetAgeWarning:           "Min asset age warning",
	FieldMaxDeviationFromBookValue:    "Max deviation from book value",
	FieldLongMissingAfter:             "Long missing after",
	FieldPhotoAtStart:                 "Photo at start",
	FieldPhotoAtEnd:                   "Photo at end",
	FieldAfterPhotoRequired:           "After photo required",
	FieldTransactionTimeout:           "Transaction timeout",
	FieldDueDateReminders:             "Due date reminders",
	FieldRequireApproval:              "Require approval",
	FieldApprovalType:                 "Approval type",
	FieldApprovalScope:                "Approval scope",
	FieldSelfService:                  "Self service",
	FieldAutoApprove:                  "Auto-approve",
	FieldAutoApproveMaxDuration:       "Auto-approve when duration ≤",
	FieldAutoApproveSameParentGroup:   "Auto-approve when destination in same parent Group",
	FieldSendEmailNotifications:       "Send email notifications",
	FieldGracePeriod:                  "Grace period",
	FieldRequireLocation:              "Require location",
	FieldReminderBeforeDue:            "Reminder before due",
	FieldPhotoAfter:                   "Photo after",
	FieldRequireApprovalToClose:       "Require approval to close",
	FieldRequireScanToStart:           "Require scan to start",
}

// ZeroTexts maps a field key to the text its value 0 is logged as (LLD PATCH Logs Written).
var ZeroTexts = map[string]string{
	FieldMaxExtensions:        "No extension",
	FieldMaxAssetsPerAssignee: "Unlimited",
	FieldLongMissingAfter:     "Off",
	FieldGracePeriod:          "No grace period",
}

// ModeInheritLabel is how the Inherit mode is written in log values.
const ModeInheritLabel = "Inherit"
