package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/query_builder"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreDto "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/dto"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
)

const pgUniqueViolation = "23505"

// ListOverrides returns one page of overrides and the total across all pages.
func (r *TransactionSettingsRepository) ListOverrides(ctx context.Context, clientID string, q dto.OverrideQuery) ([]dto.OverrideListRecord, int, error) {
	query, args, err := query_builder.BuildOverrideListQuery(q)
	if err != nil {
		return nil, 0, fmt.Errorf("build override list query: %w", err)
	}
	svc := r.open(clientID)
	defer svc.Close()

	results := []coreDto.PaginationResult[dto.OverrideListRecord]{}
	if err := svc.SelectMany(&results, ctx, query, args...); err != nil {
		return nil, 0, fmt.Errorf("select override list: %w", err)
	}
	page := coreHelper.FormatPaginationResult(results)
	return page.Data, page.TotalRecords, nil
}

// FindOverrideOptions returns the distinct scopes, creator names and changed field keys of the
// row set, for the flags set on q.
func (r *TransactionSettingsRepository) FindOverrideOptions(ctx context.Context, clientID string, q dto.OverrideQuery) (service.OverrideOptionValues, error) {
	svc := r.open(clientID)
	defer svc.Close()

	var out service.OverrideOptionValues
	read := func(want bool, build func(dto.OverrideQuery) (string, []any, error), into *[]string) error {
		if !want {
			return nil
		}
		query, args, err := build(q)
		if err != nil {
			return fmt.Errorf("build override options query: %w", err)
		}
		var rows []dto.OptionRecord
		if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
			return fmt.Errorf("select override options: %w", err)
		}
		for _, row := range rows {
			*into = append(*into, row.Value)
		}
		return nil
	}
	if err := read(q.WantScope, query_builder.BuildOverrideScopeOptionsQuery, &out.Scopes); err != nil {
		return out, err
	}
	if err := read(q.WantOverridden, query_builder.BuildOverrideFieldOptionsQuery, &out.FieldKeys); err != nil {
		return out, err
	}
	err := read(q.WantCreatedBy, query_builder.BuildOverrideCreatedByOptionsQuery, &out.CreatedBy)
	return out, err
}

// FindOverridesByIDs returns the stored overrides with the given ids.
func (r *TransactionSettingsRepository) FindOverridesByIDs(ctx context.Context, clientID string, ids []int64) ([]model.TransactionRuleOverride, error) {
	query, args, err := query_builder.BuildOverridesByIDsQuery(ids)
	if err != nil {
		return nil, fmt.Errorf("build overrides by ids query: %w", err)
	}
	return r.selectOverrides(ctx, clientID, query, args)
}

// FindOverridesForTargets returns the overrides of one transaction on the given targets.
func (r *TransactionSettingsRepository) FindOverridesForTargets(ctx context.Context, clientID, transaction string, targetIDs []int64) ([]model.TransactionRuleOverride, error) {
	query, args, err := query_builder.BuildOverridesForTargetsQuery(transaction, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("build overrides for targets query: %w", err)
	}
	return r.selectOverrides(ctx, clientID, query, args)
}

func (r *TransactionSettingsRepository) selectOverrides(ctx context.Context, clientID, query string, args []any) ([]model.TransactionRuleOverride, error) {
	svc := r.open(clientID)
	defer svc.Close()

	var rows []model.TransactionRuleOverride
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select overrides: %w", err)
	}
	return rows, nil
}

// FindDuplicateTargetIDs returns the targets that already have an active override for the scope,
// transaction and Category (nil = All categories).
func (r *TransactionSettingsRepository) FindDuplicateTargetIDs(ctx context.Context, clientID, scope, transaction string, categoryID *int64, targetIDs []int64) ([]int64, error) {
	query, args, err := query_builder.BuildDuplicateOverridesQuery(scope, transaction, categoryID, targetIDs)
	if err != nil {
		return nil, fmt.Errorf("build duplicate overrides query: %w", err)
	}
	svc := r.open(clientID)
	defer svc.Close()

	var rows []dto.DuplicateRecord
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select duplicate overrides: %w", err)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TargetID)
	}
	return ids, nil
}

// OverrideTransaction is one open database transaction of an override create or delete.
type OverrideTransaction struct {
	*SaveTransaction
}

// BeginOverrideWrite opens the tenant servicer and a transaction on it; the caller commits or
// rolls back, then closes.
func (r *TransactionSettingsRepository) BeginOverrideWrite(ctx context.Context, clientID string) (service.OverrideTx, error) {
	tx, err := r.BeginSave(ctx, clientID)
	if err != nil {
		return nil, err
	}
	return &OverrideTransaction{SaveTransaction: tx.(*SaveTransaction)}, nil
}

// InsertOverrides inserts the rows; a unique violation (an active override for the same Category,
// target and transaction) answers service.ErrDuplicateOverride.
func (t *OverrideTransaction) InsertOverrides(ctx context.Context, rows []model.TransactionRuleOverride) error {
	if _, err := t.svc.InsertManyWithData(ctx, query_builder.OverridesTable, rows); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
			return service.ErrDuplicateOverride
		}
		return fmt.Errorf("insert overrides: %w", err)
	}
	return nil
}

// DeleteOverrides deletes the overrides by id and returns how many rows were removed.
func (t *OverrideTransaction) DeleteOverrides(ctx context.Context, ids []int64) (int64, error) {
	n, err := t.svc.DeleteManyWithFilter(ctx, query_builder.OverridesTable, query_builder.OverridesDeleteFilter(ids))
	if err != nil {
		return 0, fmt.Errorf("delete overrides: %w", err)
	}
	return n, nil
}
