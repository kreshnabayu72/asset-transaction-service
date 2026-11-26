package repository

import (
	"context"
	"fmt"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/query_builder"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

// SaveTransaction is one open database transaction of a settings save.
type SaveTransaction struct {
	svc coreService.PostgreSQLServicer
}

// BeginSave opens the tenant servicer and a transaction on it; the caller commits or rolls back,
// then closes.
func (r *TransactionSettingsRepository) BeginSave(ctx context.Context, clientID string) (service.SaveTx, error) {
	svc := r.open(clientID)
	tx, err := svc.GetPool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin settings save transaction: %w", err)
	}
	svc.SetTransaction(tx)
	return &SaveTransaction{svc: svc}, nil
}

// UpsertFields writes the rows by (category_id, transaction_type, field_key).
func (t *SaveTransaction) UpsertFields(ctx context.Context, rows []model.TransactionRuleField) error {
	if len(rows) == 0 {
		return nil
	}
	if _, err := t.svc.UpsertManyWithData(ctx, query_builder.RuleFieldsTable, rows,
		query_builder.RuleFieldsConflictTarget, query_builder.RuleFieldsUpsertColumns); err != nil {
		return fmt.Errorf("upsert rule fields: %w", err)
	}
	return nil
}

// BumpSettingsVersion increases settings_version by one; an entity without the singleton row gets
// it created at version 2 (the first Save with a difference).
func (t *SaveTransaction) BumpSettingsVersion(ctx context.Context, actorID int64, actorName string) error {
	query, args, err := query_builder.BuildBumpSettingsVersionQuery(actorID, actorName)
	if err != nil {
		return fmt.Errorf("build bump settings version query: %w", err)
	}
	n, err := t.svc.UpdateMany(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("bump settings version: %w", err)
	}
	if n > 0 {
		return nil
	}
	now := time.Now()
	row := model.TransactionSettings{
		ID: query_builder.SettingsSingletonID, SettingsVersion: InitialSettingsVersion + 1,
		ModifiedByID: &actorID, ModifiedByName: &actorName, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := t.svc.InsertOneWithData(ctx, query_builder.SettingsTable, row); err != nil {
		return fmt.Errorf("insert settings version: %w", err)
	}
	return nil
}

// Commit commits the transaction.
func (t *SaveTransaction) Commit(ctx context.Context) error { return t.svc.CommitTransaction(ctx) }

// Rollback rolls the transaction back.
func (t *SaveTransaction) Rollback(ctx context.Context) error { return t.svc.RollbackTransaction(ctx) }

// Close is a no-op: be-core-utils shares one connection pool per database, so closing the servicer would
// close it for every request. Commit or Rollback ends the transaction.
func (t *SaveTransaction) Close() {}
