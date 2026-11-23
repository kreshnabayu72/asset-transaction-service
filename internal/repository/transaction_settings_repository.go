// Package repository holds the tenant-schema data access for asset-transaction-service.
package repository

import (
	"context"
	"fmt"

	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

const (
	tableTransactionSettings     = "transaction_settings"
	tableTransactionRuleFields   = "transaction_rule_fields"
	tableTransactionRuleOverride = "transaction_rule_overrides"
)

// TransactionSettingsRepository writes denormalized user names on transaction_settings and the rule tables.
type TransactionSettingsRepository struct{}

// NewTransactionSettingsRepository returns the repository used by the iam user-renamed consumer.
func NewTransactionSettingsRepository() *TransactionSettingsRepository {
	return &TransactionSettingsRepository{}
}

// UpdateModifiedByName rewrites modified_by_name where modified_by_id matches userID. svc must
// already be enlisted in the caller's transaction; the write is idempotent, so a redelivered
// event is harmless.
func (r *TransactionSettingsRepository) UpdateModifiedByName(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	userID int64,
	name string,
) error {
	return renameUser(ctx, svc, tableTransactionSettings, "modified_by_id", "modified_by_name", userID, name)
}

// UpdateRuleFieldsModifiedByName rewrites transaction_rule_fields.modified_by_name for userID.
func (r *TransactionSettingsRepository) UpdateRuleFieldsModifiedByName(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	userID int64,
	name string,
) error {
	return renameUser(ctx, svc, tableTransactionRuleFields, "modified_by_id", "modified_by_name", userID, name)
}

// UpdateRuleOverridesCreatedByName rewrites transaction_rule_overrides.created_by_name for userID.
func (r *TransactionSettingsRepository) UpdateRuleOverridesCreatedByName(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	userID int64,
	name string,
) error {
	return renameUser(ctx, svc, tableTransactionRuleOverride, "created_by_id", "created_by_name", userID, name)
}

func renameUser(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	table, idColumn, nameColumn string,
	userID int64,
	name string,
) error {
	_, err := svc.UpdateManyWithData(ctx, table, sql_query.WhereQuery{
		idColumn: {Operator: sql_query.SQLOperatorEqual, Value: userID},
	}, sql_query.UpdateQuery{nameColumn: name})
	if err != nil {
		return fmt.Errorf("transactionSettingsRepository.rename %s: %w", table, err)
	}
	return nil
}
