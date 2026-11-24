// Package repository runs the damaged module read queries on the caller's tenant database.
package repository

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/jackc/pgx/v5"
)

// ServicerFactory returns the tenant PostgreSQL servicer of a client. Each call opens its own
// servicer, which the caller closes and never shares across goroutines.
type ServicerFactory func(clientID string) coreService.PostgreSQLServicer

// NewTenantServicerFactory builds the factory for the tenant database dbName.
func NewTenantServicerFactory(dbName string) ServicerFactory {
	return func(clientID string) coreService.PostgreSQLServicer {
		return coreService.MakePostgreSQLService(clientID + ":" + dbName)
	}
}

// Repository executes the damaged module read queries.
type Repository struct {
	open ServicerFactory
}

// NewRepository builds the repository over the tenant servicer factory.
func NewRepository(open ServicerFactory) *Repository {
	return &Repository{open: open}
}

// CountBadge runs the Damaged badge count of the tenant.
func (r *Repository) CountBadge(ctx context.Context, clientID string, allGroups bool, groupIDs []int64) (int, error) {
	query, args, err := query_builder.BuildBadgeCountQuery(allGroups, groupIDs)
	if err != nil {
		return 0, fmt.Errorf("build badge count query: %w", err)
	}
	svc := r.open(clientID)
	defer svc.Close()

	count, err := svc.Count(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("count damaged badge: %w", err)
	}
	return count, nil
}

// FindAssetScope returns the mirror row of an asset, or nil when no asset has the ID.
func (r *Repository) FindAssetScope(ctx context.Context, clientID string, assetID int64) (*query_builder.AssetScopeRow, error) {
	query, args, err := query_builder.BuildAssetScopeQuery(assetID)
	if err != nil {
		return nil, fmt.Errorf("build asset scope query: %w", err)
	}
	svc := r.open(clientID)
	defer svc.Close()

	var row query_builder.AssetScopeRow
	if err := svc.SelectOne(&row, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select asset scope: %w", err)
	}
	return &row, nil
}

// FindRunningRecord returns the running damage record of an asset, or nil when it has none.
func (r *Repository) FindRunningRecord(ctx context.Context, clientID string, assetID int64) (*query_builder.RunningRecordRow, error) {
	query, args, err := query_builder.BuildRunningRecordQuery(assetID)
	if err != nil {
		return nil, fmt.Errorf("build running record query: %w", err)
	}
	svc := r.open(clientID)
	defer svc.Close()

	var row query_builder.RunningRecordRow
	if err := svc.SelectOne(&row, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("select running record: %w", err)
	}
	return &row, nil
}
