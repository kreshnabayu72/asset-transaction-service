package model

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// TransactionSettings is the singleton configuration in each tenant schema.
type TransactionSettings struct {
	ID                                  int64          `json:"id,string" column:"transaction_settings.id"`
	DefaultBorrowTime                   int32          `json:"default_borrow_time" column:"transaction_settings.default_borrow_time"`
	MaxBorrowTime                       int32          `json:"max_borrow_time" column:"transaction_settings.max_borrow_time"`
	ConfirmationEmailTimeout            int32          `json:"confirmation_email_timeout" column:"transaction_settings.confirmation_email_timeout"`
	ExtensionDefaultDuration            int32          `json:"extension_default_duration" column:"transaction_settings.extension_default_duration"`
	MaxExtensionDuration                int32          `json:"max_extension_duration" column:"transaction_settings.max_extension_duration"`
	MaxExtensionCount                   int32          `json:"max_extension_count" column:"transaction_settings.max_extension_count"`
	IsOverdueReminderEnabled            bool           `json:"is_overdue_reminder_enabled" column:"transaction_settings.is_overdue_reminder_enabled"`
	IsScanAtReturnRequired              bool           `json:"is_scan_at_return_required" column:"transaction_settings.is_scan_at_return_required"`
	BorrowTransactionTimeout            int32          `json:"borrow_transaction_timeout" column:"transaction_settings.borrow_transaction_timeout"`
	MaxTransferTransitTime              int32          `json:"max_transfer_transit_time" column:"transaction_settings.max_transfer_transit_time"`
	IsScanAtArrivalRequired             bool           `json:"is_scan_at_arrival_required" column:"transaction_settings.is_scan_at_arrival_required"`
	IsTransferDocumentGenerationEnabled bool           `json:"is_transfer_document_generation_enabled" column:"transaction_settings.is_transfer_document_generation_enabled"`
	TransferTransactionTimeout          int32          `json:"transfer_transaction_timeout" column:"transaction_settings.transfer_transaction_timeout"`
	IsDocumentationRequired             bool           `json:"is_documentation_required" column:"transaction_settings.is_documentation_required"`
	DisposalTransactionTimeout          int32          `json:"disposal_transaction_timeout" column:"transaction_settings.disposal_transaction_timeout"`
	DisposalFinanceThreshold            pgtype.Numeric `json:"disposal_finance_threshold" column:"transaction_settings.disposal_finance_threshold"`
	IsDamagedReminderEnabled            bool           `json:"is_damaged_reminder_enabled" column:"transaction_settings.is_damaged_reminder_enabled"`
	DamagedReminderInterval             int32          `json:"damaged_reminder_interval" column:"transaction_settings.damaged_reminder_interval"`
	IsMissingReminderEnabled            bool           `json:"is_missing_reminder_enabled" column:"transaction_settings.is_missing_reminder_enabled"`
	MissingReminderInterval             int32          `json:"missing_reminder_interval" column:"transaction_settings.missing_reminder_interval"`
	AuditGracePeriod                    int32          `json:"audit_grace_period" column:"transaction_settings.audit_grace_period"`
	IsAuditGeolocationRequired          bool           `json:"is_audit_geolocation_required" column:"transaction_settings.is_audit_geolocation_required"`
	ModifiedByID                        *int64         `json:"modified_by_id,string" column:"transaction_settings.modified_by_id"`
	ModifiedByName                      *string        `json:"modified_by_name" column:"transaction_settings.modified_by_name"`
	CreatedAt                           time.Time      `json:"created_at" column:"transaction_settings.created_at"`
	UpdatedAt                           time.Time      `json:"updated_at" column:"transaction_settings.updated_at"`
}
