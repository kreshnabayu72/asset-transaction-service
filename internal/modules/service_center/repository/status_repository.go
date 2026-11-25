package repository

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	statusService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/jackc/pgx/v5"
)

type StatusRepository struct {
	dbName string
}

func NewStatusRepository(dbName string) *StatusRepository {
	return &StatusRepository{dbName: dbName}
}

func (r *StatusRepository) BeginStatusTransaction(ctx context.Context, clientID string) (statusService.StatusTransaction, error) {
	svc := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)
	tx, err := svc.GetPool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin PostgreSQL transaction: %w", err)
	}
	svc.SetTransaction(tx)
	return &statusTransaction{svc: svc}, nil
}

type statusTransaction struct {
	svc coreService.PostgreSQLServicer
}

func (t *statusTransaction) LockStatusCenter(ctx context.Context, id int64) (statusService.StoredServiceCenterStatus, bool, error) {
	query, args, err := query_builder.BuildLockStatusCenterQuery(id)
	if err != nil {
		return statusService.StoredServiceCenterStatus{}, false, fmt.Errorf("build service center status lock query: %w", err)
	}

	var row query_builder.StatusCenterRow
	if err := t.svc.SelectOne(&row, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return statusService.StoredServiceCenterStatus{}, false, nil
		}
		return statusService.StoredServiceCenterStatus{}, false, err
	}
	return statusService.StoredServiceCenterStatus{
		ID:       row.ID,
		Name:     row.Name,
		IsActive: row.IsActive,
		Version:  row.Version,
	}, true, nil
}

func (t *statusTransaction) LoadAvailableToGroupIDs(ctx context.Context, id int64) ([]int64, error) {
	query, args, err := query_builder.BuildStatusGroupIDsQuery(id)
	if err != nil {
		return nil, fmt.Errorf("build service center status scope query: %w", err)
	}

	var row query_builder.StatusGroupIDsRow
	if err := t.svc.SelectOne(&row, ctx, query, args...); err != nil {
		return nil, err
	}
	return row.IDs, nil
}

func (t *statusTransaction) UpdateStatus(ctx context.Context, id int64, expectedVersion int32, target bool, actorID int64, actorName string) error {
	query, args, err := query_builder.BuildUpdateStatusQuery(id, expectedVersion, target, actorID, actorName)
	if err != nil {
		return fmt.Errorf("build service center status update query: %w", err)
	}
	if _, err := t.svc.UpdateOne(ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return statusService.ErrStatusVersionChanged
		}
		return err
	}
	return nil
}

func (t *statusTransaction) Commit(ctx context.Context) error {
	return t.svc.CommitTransaction(ctx)
}

func (t *statusTransaction) Rollback(ctx context.Context) error {
	return t.svc.RollbackTransaction(ctx)
}

// Close leaves the process-wide pool open; Commit or Rollback releases the transaction.
func (t *statusTransaction) Close() {}
