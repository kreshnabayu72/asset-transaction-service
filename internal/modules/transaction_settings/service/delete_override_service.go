package service

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// DeleteOverridesInput carries the request-scoped inputs of an override delete.
type DeleteOverridesInput struct {
	ClientID  string
	Request   dto.DeleteOverridesRequest
	Plan      Plan
	ActorID   int64
	ActorName string
	Access    CallerAccess
}

// DeleteOverridesResult is the outcome of a delete.
type DeleteOverridesResult struct {
	DeletedIDs []int64
	Skipped    []SkippedRow
	// NoneExist is true when none of the ids exists: the answer is 204.
	NoneExist bool
}

// SkippedRow is a row the delete did not remove.
type SkippedRow struct {
	ID     int64
	Reason string
}

// GuardSkipError is the 409 of a delete where nothing is deletable and at least one row was
// skipped by the Allowed assignee guard; Skipped lists only those rows.
type GuardSkipError struct{ Skipped []SkippedRow }

func (e *GuardSkipError) Error() string { return "at least one assignee type must be enabled" }

// DeleteOverrides deletes the overrides the caller's scope covers and the guard allows (LLD
// delete steps 3-7). Absent ids count as deleted (idempotent); skipped rows never raise the version.
func (s *Service) DeleteOverrides(ctx context.Context, in DeleteOverridesInput) (DeleteOverridesResult, error) {
	var res DeleteOverridesResult
	if s.overrides == nil {
		return res, ErrOverrideStorageUnavailable
	}
	if !in.Access.canWriteAny() {
		return res, ErrOverrideForbidden
	}
	ids, errs := parseDeleteIDs(in.Request.IDs)
	if len(errs) > 0 {
		return res, &ValidationError{Errors: errs}
	}
	found, err := s.overrides.FindOverridesByIDs(ctx, in.ClientID, ids)
	if err != nil {
		return res, fmt.Errorf("read overrides: %w", err)
	}
	byID := make(map[int64]model.TransactionRuleOverride, len(found))
	for _, o := range found {
		byID[o.ID] = o
	}
	covered, err := s.coveredRows(ctx, in, found)
	if err != nil {
		return res, err
	}
	guarded, err := s.guardedRows(ctx, in, found, covered)
	if err != nil {
		return res, err
	}
	var deletable []model.TransactionRuleOverride
	var guardSkips, scopeSkips []SkippedRow
	for _, id := range ids {
		o, ok := byID[id]
		switch {
		case !ok:
			res.DeletedIDs = append(res.DeletedIDs, id)
		case !covered[id]:
			scopeSkips = append(scopeSkips, SkippedRow{ID: id, Reason: constants.SkipOutOfScope})
		case guarded[id]:
			guardSkips = append(guardSkips, SkippedRow{ID: id, Reason: constants.SkipAssigneeGuard})
		default:
			deletable = append(deletable, o)
			res.DeletedIDs = append(res.DeletedIDs, id)
		}
	}
	if len(deletable) == 0 {
		return res, nothingDeletable(&res, guardSkips, scopeSkips)
	}
	res.Skipped = append(guardSkips, scopeSkips...)
	if err := s.removeOverrides(ctx, in, deletable); err != nil {
		return DeleteOverridesResult{}, err
	}
	s.dispatchDeleteLogs(ctx, in, deletable)
	return res, nil
}

// nothingDeletable answers LLD delete step 4.
func nothingDeletable(res *DeleteOverridesResult, guardSkips, scopeSkips []SkippedRow) error {
	switch {
	case len(guardSkips) > 0:
		return &GuardSkipError{Skipped: guardSkips}
	case len(scopeSkips) > 0:
		return ErrOverrideForbidden
	}
	// No row exists: every id is already deleted.
	res.NoneExist = true
	return nil
}

func parseDeleteIDs(raw []string) ([]int64, []dto.FieldError) {
	if len(raw) == 0 {
		return nil, []dto.FieldError{{Field: "ids", Message: msgPickIDs}}
	}
	var (
		ids  []int64
		errs []dto.FieldError
	)
	seen := map[int64]bool{}
	for i, r := range raw {
		id, ok := parseSnowflake(r)
		switch {
		case !ok:
			errs = append(errs, dto.FieldError{Field: "ids[" + strconv.Itoa(i) + "]", Message: msgInvalidID})
		case !seen[id]:
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, errs
}

// coveredRows evaluates each row against its own scope now: Total Control covers all, Manage
// transaction settings covers Asset Name rows, Group Roles cover Group rows and Asset rows inside
// their Group subtree (the asset's current Group while it exists, else the stored snapshot).
func (s *Service) coveredRows(ctx context.Context, in DeleteOverridesInput, rows []model.TransactionRuleOverride) (map[int64]bool, error) {
	covered := make(map[int64]bool, len(rows))
	a := in.Access
	var groups map[int64]bool
	var assetIDs []int64
	for _, o := range rows {
		if a.TotalControl {
			covered[o.ID] = true
			continue
		}
		if o.Scope == constants.ScopeAssetName {
			covered[o.ID] = a.AssetNameWrite
			continue
		}
		if o.Scope == constants.ScopeAsset {
			assetIDs = append(assetIDs, o.TargetID)
		}
	}
	needGroups := len(assetIDs) > 0
	for _, o := range rows {
		needGroups = needGroups || (o.Scope == constants.ScopeGroup && !a.TotalControl)
	}
	if !needGroups || len(a.WriteGroupIDs) == 0 {
		return covered, nil
	}
	if s.groupScope == nil {
		return nil, ErrOverrideDirectoryUnavailable
	}
	expanded, err := s.groupScope.ExpandGroups(ctx, in.ClientID, a.WriteGroupIDs)
	if err != nil {
		return nil, fmt.Errorf("expand write groups: %w", err)
	}
	groups = make(map[int64]bool, len(expanded))
	for _, g := range expanded {
		groups[g] = true
	}
	current := map[int64]int64{}
	if len(assetIDs) > 0 {
		if current, err = s.groupScope.CurrentAssetGroups(ctx, in.ClientID, assetIDs); err != nil {
			return nil, fmt.Errorf("read current asset groups: %w", err)
		}
	}
	for _, o := range rows {
		if covered[o.ID] || a.TotalControl || o.Scope == constants.ScopeAssetName {
			continue
		}
		switch o.Scope {
		case constants.ScopeGroup:
			covered[o.ID] = groups[o.TargetID]
		case constants.ScopeAsset:
			g, ok := current[o.TargetID]
			if !ok && o.TargetGroupID != nil {
				g, ok = *o.TargetGroupID, true
			}
			covered[o.ID] = ok && groups[g]
		}
	}
	return covered, nil
}

// guardedRows marks the covered Assign rows whose deletion would leave another override of the
// same target with Allowed assignee: Employee and Division both OFF while its Allow Assign is not
// Deny. Each row is evaluated alone against the stored state, and the effective combination of a
// remaining override reads its own fields, the target's other overrides, the Category and the
// Company default (Open Items 6 and 7 of the PRD leave the evaluation order and the asset-level
// scope undecided; no asset provider resolves Group and Asset Name levels above an asset).
func (s *Service) guardedRows(ctx context.Context, in DeleteOverridesInput, found []model.TransactionRuleOverride, covered map[int64]bool) (map[int64]bool, error) {
	guarded := map[int64]bool{}
	if transactionLevel(constants.TransactionAssign, in.Plan) != constants.LevelFull {
		return guarded, nil
	}
	var rows []model.TransactionRuleOverride
	for _, o := range found {
		if covered[o.ID] {
			rows = append(rows, o)
		}
	}
	var targets []int64
	for _, o := range rows {
		if o.TransactionType == constants.TransactionAssign {
			targets = append(targets, o.TargetID)
		}
	}
	if len(targets) == 0 {
		return guarded, nil
	}
	siblings, err := s.overrides.FindOverridesForTargets(ctx, in.ClientID, constants.TransactionAssign, targets)
	if err != nil {
		return nil, fmt.Errorf("read sibling overrides: %w", err)
	}
	company, err := s.nodeIndex(ctx, in.ClientID, nil)
	if err != nil {
		return nil, err
	}
	cats := map[int64]fieldIndex{}
	for _, o := range rows {
		if o.TransactionType != constants.TransactionAssign {
			continue
		}
		for _, other := range siblings {
			if other.ID == o.ID || other.Scope != o.Scope || other.TargetID != o.TargetID {
				continue
			}
			cat, err := s.categoryIndex(ctx, in.ClientID, other.CategoryID, cats)
			if err != nil {
				return nil, err
			}
			layers := []map[string]any{decodeOverrideFields(other.Fields)}
			for _, third := range siblings {
				if third.ID != o.ID && third.ID != other.ID && third.Scope == o.Scope && third.TargetID == o.TargetID && other.CategoryID != nil && third.CategoryID == nil {
					layers = append(layers, decodeOverrideFields(third.Fields))
				}
			}
			if deny, both := assignState(layers, cat, company); !deny && both {
				guarded[o.ID] = true
				break
			}
		}
	}
	return guarded, nil
}

func (s *Service) nodeIndex(ctx context.Context, clientID string, cat *int64) (fieldIndex, error) {
	rows, err := s.repo.FindNodeFields(ctx, clientID, cat)
	if err != nil {
		return nil, fmt.Errorf("read stored fields: %w", err)
	}
	return newFieldIndex(rows), nil
}

func (s *Service) categoryIndex(ctx context.Context, clientID string, cat *int64, cache map[int64]fieldIndex) (fieldIndex, error) {
	if cat == nil {
		return nil, nil
	}
	if idx, ok := cache[*cat]; ok {
		return idx, nil
	}
	idx, err := s.nodeIndex(ctx, clientID, cat)
	if err != nil {
		return nil, err
	}
	cache[*cat] = idx
	return idx, nil
}

// removeOverrides deletes the rows and bumps the settings version once in one transaction.
func (s *Service) removeOverrides(ctx context.Context, in DeleteOverridesInput, rows []model.TransactionRuleOverride) error {
	ids := make([]int64, 0, len(rows))
	for _, o := range rows {
		ids = append(ids, o.ID)
	}
	tx, err := s.overrides.BeginOverrideWrite(ctx, in.ClientID)
	if err != nil {
		return err
	}
	defer tx.Close()
	n, err := tx.DeleteOverrides(ctx, ids)
	if err != nil {
		return rollbackOverrides(ctx, tx, err)
	}
	if n > 0 {
		if err := tx.BumpSettingsVersion(ctx, in.ActorID, in.ActorName); err != nil {
			return rollbackOverrides(ctx, tx, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit override delete: %w", err)
	}
	return nil
}

// dispatchDeleteLogs sends one Activity Log entry per deleted override after commit, best-effort.
func (s *Service) dispatchDeleteLogs(ctx context.Context, in DeleteOverridesInput, rows []model.TransactionRuleOverride) {
	if s.activityEntries == nil || len(rows) == 0 {
		return
	}
	entries := make([]ActivityLogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ActivityLogEntry{
			ActorID: in.ActorID, ActorName: in.ActorName, LogTable: LogTableTransactionSettings,
			Action: actionDelete, Object: constants.TransactionLabels[row.TransactionType], ObjectName: overrideLogName(row),
		})
	}
	if err := s.activityEntries.DispatchActivityEntries(context.WithoutCancel(ctx), in.ClientID, entries); err != nil {
		log.Printf("transaction settings override delete activity log dispatch failed: %v", err)
	}
}
