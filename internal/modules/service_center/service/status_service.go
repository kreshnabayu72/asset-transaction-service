package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	analyticsTemporal "gitea.tagsamurai.local/BETS-V2/service-contracts/services/analytics_reporting/temporal"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
)

const (
	statusNoServiceCenterMessage = "This service center is no longer available."
	statusVersionConflictMessage = "This service center was changed by someone else. Please review."
	statusActivateWithoutGroups  = "Add at least one group before activating."
)

var ErrStatusVersionChanged = errors.New("service center version changed during status update")

type StoredServiceCenterStatus struct {
	ID       int64
	Name     string
	IsActive bool
	Version  int32
}

type StatusRepository interface {
	BeginStatusTransaction(ctx context.Context, clientID string) (StatusTransaction, error)
}

type StatusTransaction interface {
	LockStatusCenter(ctx context.Context, id int64) (StoredServiceCenterStatus, bool, error)
	LoadAvailableToGroupIDs(ctx context.Context, id int64) ([]int64, error)
	UpdateStatus(ctx context.Context, id int64, expectedVersion int32, target bool, actorID int64, actorName string) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Close()
}

type StatusChangeLogDispatcher interface {
	RecordChangeLogs(ctx context.Context, param analyticsTemporal.RecordChangeLogsParam) error
}

type StatusAuthorization struct {
	TotalControl       bool
	ManageGroupIDs     []int64
	AssetAdminGroupIDs []int64
}

func (a StatusAuthorization) HasStatusRight() bool {
	return a.TotalControl || len(a.ManageGroupIDs) > 0 || len(a.AssetAdminGroupIDs) > 0
}

// CoversAvailableTo intentionally checks only group IDs explicitly present in the
// authorization snapshot. It does not expand a manager's scope to descendants.
func (a StatusAuthorization) CoversAvailableTo(groupIDs []int64) bool {
	if a.TotalControl {
		return true
	}
	if len(groupIDs) == 0 {
		return false
	}

	allowed := make(map[int64]struct{}, len(a.ManageGroupIDs)+len(a.AssetAdminGroupIDs))
	for _, id := range a.ManageGroupIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range a.AssetAdminGroupIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range groupIDs {
		if _, ok := allowed[id]; !ok {
			return false
		}
	}
	return true
}

type StatusChangeResult struct {
	Changed bool
}

type StatusOperationService struct {
	repository    StatusRepository
	logDispatcher StatusChangeLogDispatcher
}

func NewStatusOperationService(repository StatusRepository, logDispatcher StatusChangeLogDispatcher) *StatusOperationService {
	return &StatusOperationService{repository: repository, logDispatcher: logDispatcher}
}

func (s *StatusOperationService) ChangeStatus(
	ctx context.Context,
	clientID string,
	serviceCenterID int64,
	request dto.ChangeStatusRequest,
	authorization StatusAuthorization,
	actorID int64,
	actorName string,
) (StatusChangeResult, error) {
	tx, err := s.repository.BeginStatusTransaction(ctx, clientID)
	if err != nil {
		return StatusChangeResult{}, fmt.Errorf("begin service center status transaction: %w", err)
	}
	defer tx.Close()

	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	center, found, err := tx.LockStatusCenter(ctx, serviceCenterID)
	if err != nil {
		return StatusChangeResult{}, fmt.Errorf("lock service center status: %w", err)
	}
	if !found {
		return StatusChangeResult{}, coreEntity.NotFound(statusNoServiceCenterMessage)
	}

	groupIDs, err := tx.LoadAvailableToGroupIDs(ctx, serviceCenterID)
	if err != nil {
		return StatusChangeResult{}, fmt.Errorf("load service center status scope: %w", err)
	}
	if !authorization.CoversAvailableTo(groupIDs) {
		return StatusChangeResult{}, statusForbidden("manage_scope")
	}

	// The target-state check precedes version validation by contract. A stale
	// retry that already reached its target remains a no-op.
	if center.IsActive == request.IsActive {
		return StatusChangeResult{Changed: false}, nil
	}
	if request.Version != center.Version {
		return StatusChangeResult{}, statusVersionConflict(center)
	}
	if request.IsActive && len(groupIDs) == 0 {
		return StatusChangeResult{}, coreEntity.UnprocessableEntity(
			"Invalid request body",
			[]coreValidator.BindFieldError{{Field: "isActive", Message: statusActivateWithoutGroups}},
		)
	}

	if err := tx.UpdateStatus(ctx, serviceCenterID, request.Version, request.IsActive, actorID, actorName); err != nil {
		if errors.Is(err, ErrStatusVersionChanged) {
			return StatusChangeResult{}, statusVersionConflict(center)
		}
		return StatusChangeResult{}, fmt.Errorf("update service center status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return StatusChangeResult{}, fmt.Errorf("commit service center status: %w", err)
	}
	committed = true

	if s.logDispatcher == nil {
		slog.ErrorContext(ctx, "service center status changelog dispatcher is not configured", "serviceCenterId", serviceCenterID)
	} else if err := s.logDispatcher.RecordChangeLogs(context.WithoutCancel(ctx), statusChangeLogParam(clientID, center, groupIDs, request.IsActive, actorID, actorName)); err != nil {
		slog.ErrorContext(ctx, "service center status changelog dispatch failed", "serviceCenterId", serviceCenterID, "err", err)
	}
	return StatusChangeResult{Changed: true}, nil
}

func statusChangeLogParam(clientID string, center StoredServiceCenterStatus, groupIDs []int64, target bool, actorID int64, actorName string) analyticsTemporal.RecordChangeLogsParam {
	action := "Deactivated"
	oldLabel := "Active"
	newLabel := "Inactive"
	if target {
		action = "Activated"
		oldLabel = "Inactive"
		newLabel = "Active"
	}
	field := "Status"
	objectID := strconv.FormatInt(center.ID, 10)
	objectName := center.Name
	product := analyticsTemporal.ProductFixedAsset
	scopeGroupIDs := make([]string, 0, len(groupIDs))
	for _, groupID := range groupIDs {
		scopeGroupIDs = append(scopeGroupIDs, strconv.FormatInt(groupID, 10))
	}

	return analyticsTemporal.RecordChangeLogsParam{
		ClientID: clientID,
		ChangeLogs: []analyticsTemporal.ChangeLog{{
			Action:            action,
			Field:             &field,
			OldValue:          &oldLabel,
			NewValue:          &newLabel,
			LogTable:          "service_centers",
			CreatedByID:       strconv.FormatInt(actorID, 10),
			CreatedBySnapshot: actorName,
			EntityID:          clientID,
			EntityType:        analyticsTemporal.EntityTypeClient,
			Product:           &product,
			Object:            "Service Center",
			ObjectID:          &objectID,
			ObjectName:        &objectName,
			ScopeGroupIDs:     scopeGroupIDs,
		}},
	}
}

func statusForbidden(reason string) *coreEntity.HttpError {
	err := coreEntity.Forbidden("Forbidden")
	err.Data = map[string]string{"reason": reason}
	return err
}

func statusVersionConflict(center StoredServiceCenterStatus) *coreEntity.HttpError {
	err := coreEntity.Conflict(statusVersionConflictMessage)
	err.Data = map[string]any{
		"reason": "version_conflict",
		"current": map[string]any{
			"_id":     strconv.FormatInt(center.ID, 10),
			"name":    center.Name,
			"version": center.Version,
		},
	}
	return err
}
