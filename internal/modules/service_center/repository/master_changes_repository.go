package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/jackc/pgx/v5"
)

// MasterChangesEffect contains inputs that the local service can safely derive after commit.
type MasterChangesEffect struct {
	ClientID          string
	Type              string
	Action            string
	ServiceCenterID   int64
	ServiceCenterName string
	CreatedByID       int64
	BeforeIDs         []int64
	AfterIDs          []int64
	WasActive         bool
	IsActive          bool
	AutoDeactivated   bool
}

// MasterChangesSideEffectDispatcher runs optional best-effort post-commit effects.
type MasterChangesSideEffectDispatcher func(context.Context, MasterChangesEffect) error

type MasterChangesRepository struct {
	dbName         string
	newService     func(string) coreService.PostgreSQLServicer
	runTransaction masterChangesTransactionFunc
	dispatcher     MasterChangesSideEffectDispatcher
}

func NewMasterChangesRepository(dbName string, dispatcher ...MasterChangesSideEffectDispatcher) *MasterChangesRepository {
	repository := &MasterChangesRepository{
		dbName: dbName,
		newService: func(databaseName string) coreService.PostgreSQLServicer {
			return coreService.MakePostgreSQLService(databaseName)
		},
		runTransaction: runMasterChangesTransaction,
	}
	if len(dispatcher) > 0 {
		repository.dispatcher = dispatcher[0]
	}
	return repository
}

type masterChangesTransactionFunc func(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	operation func() (int64, error),
) (int64, error)

func runMasterChangesTransaction(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	operation func() (int64, error),
) (int64, error) {
	return coreService.UseTransactions(ctx, svc.GetPool(), func(tx pgx.Tx) (int64, error) {
		svc.SetTransaction(tx)
		defer svc.SetTransaction(nil)
		return operation()
	})
}

func (r *MasterChangesRepository) ApplyMasterChanges(
	ctx context.Context,
	clientID string,
	change dto.MasterChangesRequest,
) (int64, error) {
	svc := r.newService(clientID + ":" + r.dbName)

	targets, err := r.findTargetCenters(ctx, svc, change)
	if err != nil {
		return 0, fmt.Errorf("failed to find Service Centers for master change: %w", err)
	}
	if len(targets) == 0 {
		return 0, nil
	}

	var effects []MasterChangesEffect
	affected, err := r.runTransaction(ctx, svc, func() (int64, error) {
		var affected int64
		for _, serviceCenterID := range targets {
			effect, changed, err := r.applyToServiceCenter(ctx, svc, serviceCenterID, change)
			if err != nil {
				return 0, err
			}
			if changed {
				effect.ClientID = clientID
				effects = append(effects, effect)
				affected++
			}
		}
		return affected, nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to apply master change transaction: %w", err)
	}

	if r.dispatcher != nil {
		for _, effect := range effects {
			if err := r.dispatcher(ctx, effect); err != nil {
				slog.WarnContext(ctx, "failed to dispatch best-effort Service Center master change effect",
					"clientId", effect.ClientID,
					"serviceCenterId", effect.ServiceCenterID,
					"type", effect.Type,
					"action", effect.Action,
					"error", err,
				)
			}
		}
	}
	return affected, nil
}

func (r *MasterChangesRepository) findTargetCenters(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	change dto.MasterChangesRequest,
) ([]int64, error) {
	query, args, err := query_builder.BuildTargetCentersQuery(change.Type, change.IDs)
	if err != nil {
		return nil, err
	}
	var rows []query_builder.TargetCenterRow
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	targets := make([]int64, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, row.ServiceCenterID)
	}
	return targets, nil
}

func (r *MasterChangesRepository) applyToServiceCenter(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	serviceCenterID int64,
	change dto.MasterChangesRequest,
) (MasterChangesEffect, bool, error) {
	lockQuery, lockArgs, err := query_builder.BuildLockServiceCenterQuery(serviceCenterID)
	if err != nil {
		return MasterChangesEffect{}, false, err
	}
	var center query_builder.LockedServiceCenterRow
	if err := svc.SelectOne(&center, ctx, lockQuery, lockArgs...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MasterChangesEffect{}, false, nil
		}
		return MasterChangesEffect{}, false, fmt.Errorf("failed to lock Service Center %d: %w", serviceCenterID, err)
	}

	before, err := readCurrentLinks(ctx, svc, change.Type, serviceCenterID)
	if err != nil {
		return MasterChangesEffect{}, false, fmt.Errorf("failed to read Service Center %d links: %w", serviceCenterID, err)
	}
	var mergeTx pgx.Tx
	if change.Type == dto.MasterChangeTypeBrand && change.Action == "merged" {
		if change.MergedIntoID == nil {
			return MasterChangesEffect{}, false, fmt.Errorf("mergedIntoId is required for a brand merge")
		}
		mergeTx = svc.GetTransaction()
		if mergeTx == nil {
			return MasterChangesEffect{}, false, fmt.Errorf("brand merge requires an active database transaction")
		}
	}

	deleteQuery, deleteArgs, err := query_builder.BuildDeleteLinksQuery(change.Type, serviceCenterID, change.IDs)
	if err != nil {
		return MasterChangesEffect{}, false, err
	}
	if _, err := svc.DeleteMany(ctx, deleteQuery, deleteArgs...); err != nil {
		return MasterChangesEffect{}, false, fmt.Errorf("failed to delete Service Center %d links: %w", serviceCenterID, err)
	}

	if mergeTx != nil {
		const mergeBrandQuery = `INSERT INTO service_center_brands (service_center_id, brand_id) VALUES ($1, $2) ON CONFLICT (service_center_id, brand_id) DO NOTHING`
		if _, err := mergeTx.Exec(ctx, mergeBrandQuery, serviceCenterID, *change.MergedIntoID); err != nil {
			return MasterChangesEffect{}, false, fmt.Errorf("failed to merge Brand link for Service Center %d: %w", serviceCenterID, err)
		}
	}

	after, err := readCurrentLinks(ctx, svc, change.Type, serviceCenterID)
	if err != nil {
		return MasterChangesEffect{}, false, fmt.Errorf("failed to read updated Service Center %d links: %w", serviceCenterID, err)
	}
	linksChanged := !sameIDs(before, after)
	autoDeactivate := isGroupRemoval(change) && center.IsActive && len(after) == 0
	if !linksChanged && !autoDeactivate {
		return MasterChangesEffect{}, false, nil
	}

	updateQuery, updateArgs, err := query_builder.BuildServiceCenterUpdateQuery(serviceCenterID, autoDeactivate)
	if err != nil {
		return MasterChangesEffect{}, false, err
	}
	if _, err := svc.UpdateOne(ctx, updateQuery, updateArgs...); err != nil {
		return MasterChangesEffect{}, false, fmt.Errorf("failed to update Service Center %d version: %w", serviceCenterID, err)
	}
	return MasterChangesEffect{
		Type: change.Type, Action: change.Action,
		ServiceCenterID: serviceCenterID, ServiceCenterName: center.Name, CreatedByID: center.CreatedByID,
		BeforeIDs: append([]int64(nil), before...), AfterIDs: append([]int64(nil), after...),
		WasActive: center.IsActive, IsActive: center.IsActive && !autoDeactivate, AutoDeactivated: autoDeactivate,
	}, true, nil
}

func readCurrentLinks(
	ctx context.Context,
	svc coreService.PostgreSQLServicer,
	changeType string,
	serviceCenterID int64,
) ([]int64, error) {
	query, args, err := query_builder.BuildCurrentLinksQuery(changeType, serviceCenterID)
	if err != nil {
		return nil, err
	}
	var rows []query_builder.LinkIDRow
	if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func sameIDs(left, right []int64) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[int64]int, len(left))
	for _, id := range left {
		counts[id]++
	}
	for _, id := range right {
		counts[id]--
		if counts[id] < 0 {
			return false
		}
	}
	return true
}

func isGroupRemoval(change dto.MasterChangesRequest) bool {
	return change.Type == dto.MasterChangeTypeGroup &&
		(change.Action == "deleted" || change.Action == "deactivated" || change.Action == "archived")
}
