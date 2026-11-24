// Package service applies the damaged module read rules to the caller's RBAC snapshot.
package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/query_builder"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

// ErrAssetNotFound reports that no mirror asset has the requested ID.
var ErrAssetNotFound = errors.New("asset not found")

// ErrGroupScope reports that the asset lies outside the caller's view scope.
var ErrGroupScope = errors.New("asset outside caller group scope")

// Repository runs the damaged module read queries on the caller's tenant.
type Repository interface {
	CountBadge(ctx context.Context, clientID string, allGroups bool, groupIDs []int64) (int, error)
	FindAssetScope(ctx context.Context, clientID string, assetID int64) (*query_builder.AssetScopeRow, error)
	FindRunningRecord(ctx context.Context, clientID string, assetID int64) (*query_builder.RunningRecordRow, error)
}

// Service implements the damaged module read operations.
type Service struct {
	repo Repository
}

// NewService builds the damaged module read service.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Summary returns the Damaged badge count of the caller. Total Control counts every group; other
// callers count the groups where they hold Manage asset & TAG Update or are Group Manager. BadgeCount
// stays nil when the caller has no such group.
func (s *Service) Summary(ctx context.Context, clientID string, roles middleware.UserAccessCache) (dto.SummaryData, error) {
	allGroups := roles.IsTotalControl(enum.ModuleAccessFixedAsset)
	var actorGroupIDs []int64
	if !allGroups {
		actorGroupIDs = roles.GroupIDsWithPermission(middleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG)
		actorGroupIDs = append(actorGroupIDs, groupManagerGroupIDs(roles)...)
		if len(actorGroupIDs) == 0 {
			return dto.SummaryData{}, nil
		}
	}
	count, err := s.repo.CountBadge(ctx, clientID, allGroups, actorGroupIDs)
	if err != nil {
		return dto.SummaryData{}, fmt.Errorf("count damaged badge: %w", err)
	}
	return dto.SummaryData{BadgeCount: &count}, nil
}

// RunningRecord returns the running damage record of an asset, or nil when it has none. The asset
// must exist in the mirror and lie inside the caller's view scope or be held by the caller.
func (s *Service) RunningRecord(ctx context.Context, clientID string, roles middleware.UserAccessCache, callerID int64, assetID int64) (*dto.RunningRecordData, error) {
	asset, err := s.repo.FindAssetScope(ctx, clientID, assetID)
	if err != nil {
		return nil, fmt.Errorf("find asset scope: %w", err)
	}
	if asset == nil {
		return nil, ErrAssetNotFound
	}
	if !canViewAsset(roles, asset.GroupID) && !holdsAsset(callerID, asset.HolderID) {
		return nil, ErrGroupScope
	}
	record, err := s.repo.FindRunningRecord(ctx, clientID, assetID)
	if err != nil {
		return nil, fmt.Errorf("find running record: %w", err)
	}
	if record == nil {
		return nil, nil
	}
	return &dto.RunningRecordData{
		ID:            record.ID,
		Status:        record.Status,
		MissingMarker: record.MissingMarker,
		ReportedAt:    record.ReportedAt.UTC(),
	}, nil
}

// canViewAsset applies the asset-detail scope. Total Control and Read Only skip the group
// predicate; other callers need the asset's group among their view groups.
func canViewAsset(roles middleware.UserAccessCache, groupID *int64) bool {
	if roles.IsTotalControl(enum.ModuleAccessFixedAsset) || roles.IsReadOnly(enum.ModuleAccessFixedAsset) {
		return true
	}
	if groupID == nil {
		return false
	}
	viewGroupIDs := roles.GroupIDsWithPermission(middleware.PermView, enum.SystemRoleKeyManageAssetAndTAG, enum.SystemRoleKeyViewAssetValue)
	viewGroupIDs = append(viewGroupIDs, roles.GroupIDsWithPermission(middleware.PermEdit, enum.SystemRoleKeyManageAssetAndTAG)...)
	viewGroupIDs = append(viewGroupIDs, groupManagerGroupIDs(roles)...)
	return slices.Contains(viewGroupIDs, *groupID)
}

// groupManagerGroupIDs returns the caller's Group Manager group ids as int64; ids that do not parse
// are skipped.
func groupManagerGroupIDs(roles middleware.UserAccessCache) []int64 {
	var out []int64
	for _, raw := range roles.GroupManagerGroupIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, id)
	}
	return out
}

// holdsAsset is the My Asset access path: the caller is the asset's holder.
func holdsAsset(callerID int64, holderID *int64) bool {
	return holderID != nil && *holderID == callerID
}
