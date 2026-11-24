// Package service holds the transaction settings business logic.
//
// Stored rule fields, the settings version and overrides come from the repository and are merged
// over the catalogue defaults (a field without a stored row reads as its Company default). The
// owner-service providers live in package provider and enter through the seams below. The plan arrives per request
// from the handler (the plan provider), and the change and activity logs go through the seams below.
package service

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// InitialSettingsVersion is the settings version of an entity that never saved a change.
const InitialSettingsVersion = 1

// SettingsRepository reads the stored rules of one tenant.
type SettingsRepository interface {
	// FindNodeFields returns the stored fields of one node (nil = Company default).
	FindNodeFields(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleField, error)
	FindSettingsVersion(ctx context.Context, clientID string) (int64, error)
	// FindExportFields returns every stored field (nil) or the Company default plus the leaf's fields.
	FindExportFields(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleField, error)
	// FindExportOverrides returns every override (nil) or the leaf's plus the All categories overrides.
	FindExportOverrides(ctx context.Context, clientID string, categoryID *int64) ([]model.TransactionRuleOverride, error)
	// BeginSave opens the one database transaction of a settings save.
	BeginSave(ctx context.Context, clientID string) (SaveTx, error)
}

// ErrExportRendererUnavailable maps to 500 when the service has no workbook renderer.
var ErrExportRendererUnavailable = errors.New("transaction settings export: workbook renderer unavailable")

// Plan is the entity plan input of the settings page.
type Plan struct {
	ApprovalWorkflowAvailable bool
	FeatureSuite              dto.FeatureSuite
	Addons                    dto.Addons
	// HasApprover is the iam-service approver-presence answer per transaction (GET step 9). A nil
	// map or a missing transaction means the presence is unknown, so noApprover reads false.
	HasApprover map[string]bool
}

// FullPlan gates nothing: Approval Workflow available and every Feature Suite at Full. The app reads the
// plan from the plan provider per request; FullPlan serves the tests.
var FullPlan = Plan{
	ApprovalWorkflowAvailable: true,
	FeatureSuite: dto.FeatureSuite{
		Borrowing:  constants.LevelFull,
		Assignment: constants.LevelFull,
		Transfer:   constants.LevelFull,
		Disposal:   constants.LevelFull,
	},
	Addons: dto.Addons{Audit: true, Maintenance: true},
}

// WorkbookRenderer renders the export sheets into one xlsx file.
type WorkbookRenderer interface {
	Render(sheets []dto.Sheet) ([]byte, error)
}

// Service answers the transaction settings operations.
type Service struct {
	repo            SettingsRepository
	renderer        WorkbookRenderer
	activityEntries ActivityEntryDispatcher

	// Optional providers (nil until they exist), set through the With* methods of seams.go.
	categories   CategoryDirectory
	changeLogs   ChangeLogDispatcher
	assets       AssetDirectory
	denyContacts DenyContactResolver
	targets      OverrideTargetDirectory
	groupScope   GroupScopeDirectory

	// overrides is the override storage; NewService takes it from repo when repo implements it.
	overrides OverrideRepository
}

// NewService builds the service; renderer may be nil until the workbook provider exists.
func NewService(repo SettingsRepository, renderer WorkbookRenderer) *Service {
	s := &Service{repo: repo, renderer: renderer}
	if o, ok := repo.(OverrideRepository); ok {
		s.overrides = o
	}
	return s
}

// resolveLeafCategory answers a categoryId of GET and export (LLD GET step 2, export step 1). A nil id selects
// All categories and needs no lookup. An id the directory does not know as a leaf Category answers
// ErrCategoryNotFound (404). An unset directory accepts every id, logged by its id.
func (s *Service) resolveLeafCategory(ctx context.Context, clientID string, id *int64) (CategoryInfo, error) {
	if id == nil || s.categories == nil {
		return CategoryInfo{}, nil
	}
	info, err := s.categories.LookupCategory(ctx, clientID, *id)
	if err != nil {
		return CategoryInfo{}, fmt.Errorf("look up category: %w", err)
	}
	if !info.Exists || !info.Leaf {
		return CategoryInfo{}, ErrCategoryNotFound
	}
	return info, nil
}
