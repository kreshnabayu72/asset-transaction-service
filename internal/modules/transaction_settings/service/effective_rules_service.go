package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// Errors of the effective-rules lookup.
var (
	// ErrAssetNotFound maps to 404: an unknown, deleted or disposed asset, an asset of another entity or
	// an asset outside the caller's Group scope; the whole request is rejected.
	ErrAssetNotFound = errors.New("asset not found")
	// ErrAssetDirectoryUnavailable maps to 500: no asset ownership provider is wired.
	ErrAssetDirectoryUnavailable = errors.New("effective rules: asset directory unavailable")
	// ErrDenyContactUnavailable maps to 500: an asset resolves to Deny and no contact provider is wired.
	ErrDenyContactUnavailable = errors.New("effective rules: deny contact provider unavailable")
)

// Source levels and scopes of a resolved field (OpenAPI EffectiveRulesResponse).
const (
	levelAsset      = "asset"
	levelGroup      = "group"
	levelAssetName  = "asset_name"
	levelCategory   = "category"
	levelCompany    = "company_default"
	scopeAll        = "all"
	scopeLeaf       = "leaf"
	roleGroupMgr    = "Group Manager"
	roleTotalCtrl   = "Total Control"
	overrideGroup   = "group"
	overrideAsset   = "asset"
	overrideAssetNm = "asset_name"
)

// GroupRef is a Group of an asset with its full path.
type GroupRef struct {
	ID   int64
	Path string
}

// AssetContext is what the owner modules know about an asset now. CategoryID is nil when the
// asset's Category is deleted or a parent (its values are then not applied).
type AssetContext struct {
	ID           int64
	Label        string // the asset target name "[Asset Name] - [Increment]", the source name of an Asset-level value
	CategoryID   *int64
	CategoryPath string
	AssetNameID  int64
	AssetName    string
	Groups       []GroupRef // the asset's Group first, then its ancestors
}

// AssetDirectory resolves assets in request order; it returns ErrAssetNotFound (also for an asset of another entity)
// for the whole request.
type AssetDirectory interface {
	ResolveAssets(ctx context.Context, clientID string, ids []int64) ([]AssetContext, error)
}

// DenyContactResolver finds the nearest Group Manager walking up groupIDs (the asset's Group
// first), else a Total Control user.
type DenyContactResolver interface {
	ResolveDenyContact(ctx context.Context, clientID string, groupIDs []int64) (dto.DenyContact, error)
}

// EffectiveInput carries the request-scoped inputs of effective-rules; AssetIDs are already deduplicated
// and Access bounds the assets the caller may resolve (its Group scope).
type EffectiveInput struct {
	ClientID    string
	Transaction string
	AssetIDs    []int64
	Plan        Plan
	Access      CallerAccess
}

// resolved is one level's supply of a field value.
type resolved struct {
	value any
	level string
	name  string
	scope string
}

// ResolveEffectiveRules resolves per asset and field the value of the closest level and applies
// plan gating (LLD effective-rules steps 2-7).
func (s *Service) ResolveEffectiveRules(ctx context.Context, in EffectiveInput) ([]dto.EffectiveAsset, error) {
	if s.assets == nil {
		return nil, ErrAssetDirectoryUnavailable
	}
	assets, err := s.assets.ResolveAssets(ctx, in.ClientID, in.AssetIDs)
	if err != nil {
		return nil, err
	}
	for _, a := range assets {
		if !scopeCovers(in.Access, a.Groups) {
			return nil, ErrAssetNotFound
		}
	}
	version, err := s.repo.FindSettingsVersion(ctx, in.ClientID)
	if err != nil {
		return nil, fmt.Errorf("read settings version: %w", err)
	}
	companyRows, err := s.repo.FindNodeFields(ctx, in.ClientID, nil)
	if err != nil {
		return nil, fmt.Errorf("read company default fields: %w", err)
	}
	company := newFieldIndex(companyRows)
	leaves := map[int64]fieldIndex{}
	for _, a := range assets {
		if a.CategoryID == nil || !hasCategoryLevel(in.Transaction) {
			continue
		}
		if _, done := leaves[*a.CategoryID]; done {
			continue
		}
		rows, err := s.repo.FindNodeFields(ctx, in.ClientID, a.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("read category fields: %w", err)
		}
		leaves[*a.CategoryID] = newFieldIndex(rows)
	}
	overrides, err := s.loadOverrides(ctx, in)
	if err != nil {
		return nil, err
	}
	out := make([]dto.EffectiveAsset, 0, len(assets))
	for _, a := range assets {
		item, err := s.resolveAsset(ctx, in, a, int(version), company, leaves[derefID(a.CategoryID)], overrides)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

// scopeCovers reports whether the caller may resolve an asset whose Group chain (its Group first, then
// its ancestors) is given: Total Control and the read-all rights see every asset, a caller whose rights
// come from Group Roles sees the assets inside its view Groups, and a caller holding no Group Role has no
// Group scope to narrow the answer.
func scopeCovers(a CallerAccess, groups []GroupRef) bool {
	if a.TotalControl || a.ViewAll || len(a.ViewGroupIDs) == 0 {
		return true
	}
	for _, g := range groups {
		if slices.Contains(a.ViewGroupIDs, g.ID) {
			return true
		}
	}
	return false
}

func derefID(id *int64) int64 {
	if id == nil {
		return 0
	}
	return *id
}

// hasCategoryLevel is false for the transactions that resolve the Company default only.
func hasCategoryLevel(tx string) bool {
	return !slices.Contains(constants.AllCategoriesOnlyTransactions, tx)
}

// hasOverrideLevel is true for the four transactions that carry Group, Asset Name and Asset overrides.
func hasOverrideLevel(tx string) bool {
	return slices.Contains([]string{constants.TransactionBorrow, constants.TransactionAssign, constants.TransactionTransfer, constants.TransactionDisposal}, tx)
}

// storedOverride is an override of the transaction with its decoded field values.
type storedOverride struct {
	model.TransactionRuleOverride
	values map[string]any
}

func (s *Service) loadOverrides(ctx context.Context, in EffectiveInput) ([]storedOverride, error) {
	if !hasOverrideLevel(in.Transaction) {
		return nil, nil
	}
	rows, err := s.repo.FindExportOverrides(ctx, in.ClientID, nil)
	if err != nil {
		return nil, fmt.Errorf("read overrides: %w", err)
	}
	var out []storedOverride
	for _, row := range rows {
		if row.TransactionType != in.Transaction {
			continue
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(row.Fields, &raw); err != nil {
			return nil, fmt.Errorf("decode override %d fields: %w", row.ID, err)
		}
		values := make(map[string]any, len(raw))
		for k, v := range raw {
			values[k] = decodeStored(v)
		}
		out = append(out, storedOverride{TransactionRuleOverride: row, values: values})
	}
	return out, nil
}

// appliedOverride is an override that applies to an asset, with the level and name it supplies.
type appliedOverride struct {
	o     storedOverride
	level string
	name  string
}

// applicable returns the overrides that apply to the asset now, in precedence order: Asset, Group
// (innermost first), Asset Name; at the same level a leaf-Category override precedes All categories.
func applicable(a AssetContext, overrides []storedOverride) []appliedOverride {
	var out []appliedOverride
	add := func(scope string, match func(storedOverride) bool, level string, nameOf func(storedOverride) string) {
		for _, leafFirst := range []bool{true, false} {
			for _, o := range overrides {
				if o.Scope != scope || !match(o) || (o.CategoryID != nil) != leafFirst {
					continue
				}
				if o.CategoryID != nil && (a.CategoryID == nil || *o.CategoryID != *a.CategoryID) {
					continue
				}
				out = append(out, appliedOverride{o: o, level: level, name: nameOf(o)})
			}
		}
	}
	add(overrideAsset, func(o storedOverride) bool { return o.TargetID == a.ID }, levelAsset, func(storedOverride) string { return a.Label })
	for _, g := range a.Groups {
		add(overrideGroup, func(o storedOverride) bool { return o.TargetID == g.ID }, levelGroup, func(storedOverride) string { return g.Path })
	}
	add(overrideAssetNm, func(o storedOverride) bool { return o.TargetID == a.AssetNameID }, levelAssetName, func(storedOverride) string { return a.AssetName })
	return out
}

func (s *Service) resolveAsset(ctx context.Context, in EffectiveInput, a AssetContext, version int, company, leaf fieldIndex, overrides []storedOverride) (dto.EffectiveAsset, error) {
	applying := applicable(a, overrides)
	unitDef := constants.FieldDef{Key: constants.FieldDurationUnit, Default: constants.ValueDays}
	item := dto.EffectiveAsset{AssetID: strconv.FormatInt(a.ID, 10), SettingsVersion: version, Allowed: true, Fields: []dto.EffectiveField{}}
	level := transactionLevel(in.Transaction, in.Plan)
	var deny bool
	// the duration unit of the asset decides how Basic converts hours to days
	hoursUnit := false
	if in.Transaction == constants.TransactionBorrow {
		hoursUnit = pick(unitDef, in.Transaction, a, applying, company, leaf).value == constants.ValueHours
	}
	for _, def := range constants.Catalogue[in.Transaction] {
		r := pick(def, in.Transaction, a, applying, company, leaf)
		if allow, ok := allowKeys[in.Transaction]; ok && def.Key == allow {
			if v, isBool := r.value.(bool); isBool && !v {
				deny = true
			}
			if denied, ok := denyOverride(def, applying); ok {
				r, deny = denied, true
			}
		}
		gated := level == constants.LevelNone || isGated(in.Transaction, def.Key, in.Plan)
		item.Fields = append(item.Fields, dto.EffectiveField{
			Key: def.Key, Value: gatedValue(in, def, r.value, gated, level, hoursUnit),
			SourceLevel: r.level, SourceName: r.name, SourceCategoryScope: r.scope, Gated: gated,
		})
	}
	_, hasAllow := allowKeys[in.Transaction]
	if hasAllow && level == constants.LevelNone {
		item.Allowed = false
	}
	if deny {
		item.Allowed = false
		if s.denyContacts == nil {
			return item, ErrDenyContactUnavailable
		}
		ids := make([]int64, 0, len(a.Groups))
		for _, g := range a.Groups {
			ids = append(ids, g.ID)
		}
		contact, err := s.denyContacts.ResolveDenyContact(ctx, in.ClientID, ids)
		if err != nil {
			return item, fmt.Errorf("resolve deny contact: %w", err)
		}
		item.DenyContact = &contact
	}
	return item, nil
}

// denyOverride finds an applicable override whose Allow value is Deny; it wins over every level.
func denyOverride(def constants.FieldDef, applying []appliedOverride) (resolved, bool) {
	for _, e := range applying {
		if v, ok := e.o.values[def.Key].(bool); ok && !v {
			return resolved{value: false, level: e.level, name: e.name, scope: overrideScope(e.o)}, true
		}
	}
	return resolved{}, false
}

func overrideScope(o storedOverride) string {
	if o.CategoryID == nil {
		return scopeAll
	}
	return scopeLeaf
}

// pick chooses the value of the closest level for one field (LLD step 3).
func pick(def constants.FieldDef, tx string, a AssetContext, applying []appliedOverride, company, leaf fieldIndex) resolved {
	companyRes := resolved{value: companyValue(company, tx, def), level: levelCompany, name: constants.CompanyDefaultSource, scope: scopeAll}
	if def.AllCategoriesOnly || !hasCategoryLevel(tx) {
		return companyRes
	}
	if hasOverrideLevel(tx) {
		for _, e := range applying {
			if v, ok := e.o.values[def.Key]; ok {
				return resolved{value: v, level: e.level, name: e.name, scope: overrideScope(e.o)}
			}
		}
	}
	if a.CategoryID != nil {
		if v, ok := leaf.own(tx, def.Key); ok {
			return resolved{value: v, level: levelCategory, name: a.CategoryPath, scope: scopeLeaf}
		}
	}
	return companyRes
}

// gatedValue applies the plan's effective values (LLD step 4); a field gated for any other reason
// (Feature Suite None, inactive addon) keeps its stored value.
func gatedValue(in EffectiveInput, def constants.FieldDef, value any, gated bool, level string, hoursUnit bool) any {
	if level == constants.LevelBasic && in.Transaction == constants.TransactionBorrow && hoursUnit &&
		(def.Key == constants.FieldMaxDuration || def.Key == constants.FieldAutoApproveMaxDuration) {
		if n, ok := value.(int); ok {
			return (n + 23) / 24
		}
	}
	if !gated || level == constants.LevelNone {
		return value
	}
	plan := in.Plan
	switch def.Key {
	case constants.FieldRequireApproval:
		if !plan.ApprovalWorkflowAvailable {
			return false
		}
	case constants.FieldSelfService:
		return false
	case constants.FieldDurationUnit:
		return constants.ValueDays
	case constants.FieldAllowedAssigneeEmployee:
		return true
	case constants.FieldAllowedAssigneeDivision:
		return false
	case constants.FieldApprovalScope:
		if plan.ApprovalWorkflowAvailable && level == constants.LevelBasic {
			return constants.ValueOriginOnly
		}
	}
	return value
}
