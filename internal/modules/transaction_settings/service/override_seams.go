package service

import (
	"context"
	"errors"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// Errors of the override operations.
var (
	// ErrOverrideForbidden maps to 403: the caller's scope does not cover the request; nothing is written.
	ErrOverrideForbidden = errors.New("override: forbidden")
	// ErrDuplicateOverride is returned by the write transaction on a unique violation.
	ErrDuplicateOverride = errors.New("override: active override already exists")
	// ErrOverrideStorageUnavailable maps to 500: the repository has no override storage.
	ErrOverrideStorageUnavailable = errors.New("override: storage unavailable")
	// ErrOverrideDirectoryUnavailable maps to 500: a provider the request needs (target, Group
	// scope or Category directory) is not wired. The LLD defines no answer for missing data, so
	// the request fails instead of guessing.
	ErrOverrideDirectoryUnavailable = errors.New("override: directory provider unavailable")
)

// OverrideOptionValues are the distinct values an options read returns.
type OverrideOptionValues struct {
	Scopes    []string
	FieldKeys []string
	CreatedBy []string
}

// OverrideTx is the open database transaction of an override create or delete.
type OverrideTx interface {
	InsertOverrides(ctx context.Context, rows []model.TransactionRuleOverride) error
	DeleteOverrides(ctx context.Context, ids []int64) (int64, error)
	BumpSettingsVersion(ctx context.Context, actorID int64, actorName string) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Close()
}

// OverrideRepository is the override storage; the repository implements it next to
// SettingsRepository and NewService picks it up from the same value.
type OverrideRepository interface {
	ListOverrides(ctx context.Context, clientID string, q dto.OverrideQuery) ([]dto.OverrideListRecord, int, error)
	FindOverrideOptions(ctx context.Context, clientID string, q dto.OverrideQuery) (OverrideOptionValues, error)
	FindOverridesByIDs(ctx context.Context, clientID string, ids []int64) ([]model.TransactionRuleOverride, error)
	FindOverridesForTargets(ctx context.Context, clientID, transaction string, targetIDs []int64) ([]model.TransactionRuleOverride, error)
	FindDuplicateTargetIDs(ctx context.Context, clientID, scope, transaction string, categoryID *int64, targetIDs []int64) ([]int64, error)
	BeginOverrideWrite(ctx context.Context, clientID string) (OverrideTx, error)
}

// OverrideTarget is what the target provider answers for one Group, Asset Name or Asset id.
type OverrideTarget struct {
	ID int64
	// Exists is false when the target is unknown or deleted: MAR and EMS answer found=false for both, so the
	// service answers both as not found (404).
	Exists bool
	// Name is the snapshot stored in target_name: the full Group path (Parent > Child), the Asset
	// Name, or "[Asset Name] - [Increment]" for an Asset, verbatim.
	Name string
	// GroupID is the asset's Group (Asset scope); unused for the other scopes.
	GroupID *int64
	// GroupChain is the Group (the target itself for a Group override, the asset's Group for an
	// Asset override) followed by its ancestors; the Group Role scope check matches any of them.
	GroupChain []int64
}

// OverrideTargetDirectory resolves override targets of one scope (settings-groups,
// settings-asset-name, asset). Ids it does not return are treated as deleted.
type OverrideTargetDirectory interface {
	ResolveTargets(ctx context.Context, clientID, scope string, ids []int64) ([]OverrideTarget, error)
}

// GroupScopeDirectory answers Group tree questions for the Group Role scope of list and delete.
type GroupScopeDirectory interface {
	// ExpandGroups returns the given Groups and all their descendants.
	ExpandGroups(ctx context.Context, clientID string, groupIDs []int64) ([]int64, error)
	// CurrentAssetGroups returns the current Group of each asset that still exists (a Transfer moves
	// the asset); assets that no longer exist are omitted.
	CurrentAssetGroups(ctx context.Context, clientID string, assetIDs []int64) (map[int64]int64, error)
}

// WithOverrideTargetDirectory sets the override target provider.
func (s *Service) WithOverrideTargetDirectory(d OverrideTargetDirectory) *Service {
	s.targets = d
	return s
}

// WithGroupScopeDirectory sets the Group tree provider of the Group Role scope.
func (s *Service) WithGroupScopeDirectory(d GroupScopeDirectory) *Service {
	s.groupScope = d
	return s
}

// CallerAccess is the caller's rights on overrides, derived from the roles snapshot.
type CallerAccess struct {
	// ReadOnly limits every right to Read, whatever other roles the caller holds.
	ReadOnly bool
	// TotalControl covers every scope and every row (Default User is equivalent).
	TotalControl bool
	// ViewAll sees every row: Total Control, Read Only or a global Manage transaction settings view.
	ViewAll bool
	// AssetNameWrite is Total Control or Manage transaction settings (Update): Asset Name scope.
	AssetNameWrite bool
	// WriteGroupIDs are the Groups where the caller holds Manage asset & TAG (Update); a Group
	// Manager's assignment counts the same.
	WriteGroupIDs []int64
	// ViewGroupIDs are the Groups where the caller holds Manage asset & TAG (View).
	ViewGroupIDs []int64
}

func (a CallerAccess) canWriteAny() bool {
	return !a.ReadOnly && (a.TotalControl || a.AssetNameWrite || len(a.WriteGroupIDs) > 0)
}

func (a CallerAccess) canWriteScope(scope string) bool {
	if a.ReadOnly {
		return false
	}
	if a.TotalControl {
		return true
	}
	if scope == "asset_name" {
		return a.AssetNameWrite
	}
	return len(a.WriteGroupIDs) > 0
}

func (a CallerAccess) coversChain(chain []int64) bool {
	for _, g := range chain {
		for _, w := range a.WriteGroupIDs {
			if g == w {
				return true
			}
		}
	}
	return false
}
