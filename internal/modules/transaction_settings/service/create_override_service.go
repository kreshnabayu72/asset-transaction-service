package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// DuplicateOverrideError is the 409 of a create: targets that already have an active override.
type DuplicateOverrideError struct{ TargetIDs []int64 }

func (e *DuplicateOverrideError) Error() string { return "active override already exists" }

// ErrTargetNotFound maps to 404 (LLD create step 4): a Group, Asset Name or Asset target that is unknown
// or of another entity, or an Asset target that is deleted or disposed; nothing is created.
var ErrTargetNotFound = errors.New("override target not found")

// CreateOverrideInput carries the request-scoped inputs of an override create.
type CreateOverrideInput struct {
	ClientID  string
	Request   dto.CreateOverrideRequest
	Plan      Plan
	ActorID   int64
	ActorName string
	Access    CallerAccess
}

// newOverrideField is one validated field of the override being created.
type newOverrideField struct {
	idx   int
	def   constants.FieldDef
	value any
}

// parsedCreate is the request after the structural checks of LLD create step 3.
type parsedCreate struct {
	categoryID *int64
	targetIDs  []int64
	fields     []newOverrideField
}

// CreateOverrides creates one override per target for one transaction atomically (LLD create
// steps 3-9): validation, target and Category checks, the plan gate, the Allowed assignee guard,
// the duplicate check, one transaction inserting the rows and bumping the settings version, then
// one Activity Log entry per override after commit.
func (s *Service) CreateOverrides(ctx context.Context, in CreateOverrideInput) error {
	if s.overrides == nil {
		return ErrOverrideStorageUnavailable
	}
	if !in.Access.canWriteAny() || (isScope(in.Request.Scope) && !in.Access.canWriteScope(in.Request.Scope)) {
		return ErrOverrideForbidden
	}
	parsed, errs := parseCreate(in.Request)
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	// The 422 entries of step 3 and step 4 are listed together; the not-found answers of step 4 follow them.
	info, catMissing, err := s.checkCreateCategory(ctx, in, parsed, &errs)
	if err != nil {
		return err
	}
	targets, tgtMissing, err := s.checkCreateTargets(ctx, in, parsed, &errs)
	if err != nil {
		return err
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	if catMissing {
		return ErrCategoryNotFound
	}
	if tgtMissing {
		return ErrTargetNotFound
	}
	for _, f := range parsed.fields {
		if fieldGated(in.Request.Transaction, f.def.Key, in.Plan) {
			errs = append(errs, fieldError(f.idx, "value", msgFieldGated))
		}
	}
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	stored, err := s.loadCreateState(ctx, in, parsed)
	if err != nil {
		return err
	}
	if errs := guardCreate(in, parsed, stored); len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	dups, err := s.overrides.FindDuplicateTargetIDs(ctx, in.ClientID, in.Request.Scope, in.Request.Transaction, parsed.categoryID, parsed.targetIDs)
	if err != nil {
		return fmt.Errorf("check duplicate overrides: %w", err)
	}
	if len(dups) > 0 {
		return &DuplicateOverrideError{TargetIDs: orderLike(parsed.targetIDs, dups)}
	}
	rows := buildOverrideRows(in, parsed, info, targets)
	if err := s.writeOverrides(ctx, in, parsed, rows); err != nil {
		return err
	}
	s.dispatchCreateLogs(ctx, in, parsed, rows, stored)
	return nil
}

// parseCreate runs the structural checks of LLD create step 3.
func parseCreate(req dto.CreateOverrideRequest) (parsedCreate, []dto.FieldError) {
	var (
		p    parsedCreate
		errs []dto.FieldError
	)
	if !isScope(req.Scope) {
		errs = append(errs, dto.FieldError{Field: "scope", Message: msgPickScope})
	}
	if !isOverrideTransaction(req.Transaction) {
		errs = append(errs, dto.FieldError{Field: "transaction", Message: msgInvalidChoice})
	}
	if req.CategoryID != nil && *req.CategoryID != "" {
		id, ok := parseSnowflake(*req.CategoryID)
		if !ok {
			errs = append(errs, dto.FieldError{Field: "categoryId", Message: msgInvalidID})
		}
		p.categoryID = &id
	}
	if len(req.TargetIDs) == 0 {
		errs = append(errs, dto.FieldError{Field: "targetIds", Message: pickTargetMessage(req.Scope)})
	}
	seen := map[int64]bool{}
	for i, raw := range req.TargetIDs {
		id, ok := parseSnowflake(raw)
		switch {
		case !ok:
			errs = append(errs, dto.FieldError{Field: "targetIds[" + strconv.Itoa(i) + "]", Message: msgInvalidID})
		case !seen[id]:
			seen[id] = true
			p.targetIDs = append(p.targetIDs, id)
		}
	}
	if len(req.Fields) == 0 {
		errs = append(errs, dto.FieldError{Field: "fields", Message: msgPickField})
	}
	keys := map[string]bool{}
	for i, f := range req.Fields {
		field, fe := parseCreateField(i, req.Transaction, f, keys)
		if fe != nil {
			errs = append(errs, *fe)
			continue
		}
		p.fields = append(p.fields, field)
	}
	return p, errs
}

func pickTargetMessage(scope string) string {
	switch scope {
	case constants.ScopeAssetName:
		return msgPickAssetName
	case constants.ScopeAsset:
		return msgPickAsset
	}
	return msgPickGroup
}

func parseCreateField(idx int, tx string, f dto.CreateOverrideField, seen map[string]bool) (newOverrideField, *dto.FieldError) {
	errAt := func(msg string) (newOverrideField, *dto.FieldError) {
		fe := fieldError(idx, "value", msg)
		return newOverrideField{}, &fe
	}
	if f.Key == "" {
		return errAt(msgPickField)
	}
	def, ok := findDef(tx, f.Key)
	switch {
	case !ok || !overridable(def):
		return errAt(msgUnknownField)
	case seen[f.Key]:
		return errAt(msgDuplicateField)
	}
	seen[f.Key] = true
	rule := ruleFor(def)
	value, msg := parseValue(rule, f.Value)
	if msg == msgValueRequired && rule.kind != kindInt {
		msg = msgPickValue
	}
	if msg != "" {
		return errAt(msg)
	}
	return newOverrideField{idx: idx, def: def, value: value}, nil
}

// checkCreateCategory validates categoryId: a leaf Category of the caller's entity. A parent Category is
// a 422 entry; an unknown, deleted or other-entity Category is reported as missing (404).
func (s *Service) checkCreateCategory(ctx context.Context, in CreateOverrideInput, p parsedCreate, errs *[]dto.FieldError) (CategoryInfo, bool, error) {
	if p.categoryID == nil {
		return CategoryInfo{}, false, nil
	}
	if s.categories == nil {
		return CategoryInfo{}, false, ErrOverrideDirectoryUnavailable
	}
	info, err := s.categories.LookupCategory(ctx, in.ClientID, *p.categoryID)
	switch {
	case err != nil:
		return info, false, fmt.Errorf("look up category: %w", err)
	case !info.Exists:
		return info, true, nil
	case !info.Leaf:
		*errs = append(*errs, dto.FieldError{Field: "categoryId", Message: msgNotLeaf})
	}
	return info, false, nil
}

// checkCreateTargets resolves the targets. An unknown, deleted or disposed target, or one of another entity,
// is reported as missing (404); a target outside the caller's scope answers ErrOverrideForbidden (403).
func (s *Service) checkCreateTargets(ctx context.Context, in CreateOverrideInput, p parsedCreate, errs *[]dto.FieldError) (map[int64]OverrideTarget, bool, error) {
	if s.targets == nil {
		return nil, false, ErrOverrideDirectoryUnavailable
	}
	list, err := s.targets.ResolveTargets(ctx, in.ClientID, in.Request.Scope, p.targetIDs)
	if err != nil {
		return nil, false, fmt.Errorf("resolve override targets: %w", err)
	}
	byID := make(map[int64]OverrideTarget, len(list))
	for _, t := range list {
		byID[t.ID] = t
	}
	missing := false
	for _, id := range p.targetIDs {
		t, ok := byID[id]
		switch {
		case !ok || !t.Exists:
			missing = true
		case in.Request.Scope != constants.ScopeAssetName && !in.Access.TotalControl && !in.Access.coversChain(t.GroupChain):
			return nil, false, ErrOverrideForbidden
		}
	}
	return byID, missing, nil
}

// createState is the stored data the guard and the log text read.
type createState struct {
	cat, company fieldIndex
	siblings     map[int64]map[string]any // existing All categories override of the target, by target id
}

func (s *Service) loadCreateState(ctx context.Context, in CreateOverrideInput, p parsedCreate) (createState, error) {
	st := createState{siblings: map[int64]map[string]any{}}
	rows, err := s.repo.FindNodeFields(ctx, in.ClientID, nil)
	if err != nil {
		return st, fmt.Errorf("read company default fields: %w", err)
	}
	st.company = newFieldIndex(rows)
	if p.categoryID != nil {
		rows, err := s.repo.FindNodeFields(ctx, in.ClientID, p.categoryID)
		if err != nil {
			return st, fmt.Errorf("read category fields: %w", err)
		}
		st.cat = newFieldIndex(rows)
	}
	if in.Request.Transaction != constants.TransactionAssign || p.categoryID == nil {
		return st, nil
	}
	existing, err := s.overrides.FindOverridesForTargets(ctx, in.ClientID, constants.TransactionAssign, p.targetIDs)
	if err != nil {
		return st, fmt.Errorf("read sibling overrides: %w", err)
	}
	for _, o := range existing {
		if o.Scope == in.Request.Scope && o.CategoryID == nil {
			st.siblings[o.TargetID] = decodeOverrideFields(o.Fields)
		}
	}
	return st, nil
}

// guardCreate is the Allowed assignee guard of LLD create step 6, evaluated per target on the
// override being created over the target's All categories override, the Category and the Company
// default. Group and Asset Name levels above an asset are not resolved (no asset provider).
func guardCreate(in CreateOverrideInput, p parsedCreate, st createState) []dto.FieldError {
	if in.Request.Transaction != constants.TransactionAssign || transactionLevel(constants.TransactionAssign, in.Plan) != constants.LevelFull {
		return nil
	}
	own := make(map[string]any, len(p.fields))
	idx := map[string]int{}
	for _, f := range p.fields {
		own[f.def.Key] = f.value
		idx[f.def.Key] = f.idx
	}
	failed := false
	for _, id := range p.targetIDs {
		layers := []map[string]any{own}
		if sib, ok := st.siblings[id]; ok {
			layers = append(layers, sib)
		}
		if deny, both := assignState(layers, st.cat, st.company); !deny && both {
			failed = true
			break
		}
	}
	if !failed {
		return nil
	}
	if i, ok := idx[constants.FieldAllowAssign]; ok {
		return []dto.FieldError{fieldError(i, "value", msgNoAssigneeType)}
	}
	var errs []dto.FieldError
	for _, key := range []string{constants.FieldAllowedAssigneeEmployee, constants.FieldAllowedAssigneeDivision} {
		if i, ok := idx[key]; ok {
			errs = append(errs, fieldError(i, "value", msgNoAssigneeType))
		}
	}
	if len(errs) == 0 {
		errs = append(errs, dto.FieldError{Field: "fields", Message: msgNoAssigneeType})
	}
	return errs
}

func orderLike(order, subset []int64) []int64 {
	in := map[int64]bool{}
	for _, id := range subset {
		in[id] = true
	}
	out := make([]int64, 0, len(subset))
	for _, id := range order {
		if in[id] {
			out = append(out, id)
		}
	}
	return out
}

func buildOverrideRows(in CreateOverrideInput, p parsedCreate, info CategoryInfo, targets map[int64]OverrideTarget) []model.TransactionRuleOverride {
	values := make(map[string]any, len(p.fields))
	for _, f := range p.fields {
		values[f.def.Key] = f.value
	}
	// Marshal cannot fail on a map of int, bool and string values.
	raw, _ := json.Marshal(values)
	var path *string
	if p.categoryID != nil {
		v := info.Path
		if v == "" {
			v = strconv.FormatInt(*p.categoryID, 10)
		}
		path = &v
	}
	now := time.Now()
	rows := make([]model.TransactionRuleOverride, 0, len(p.targetIDs))
	for _, id := range p.targetIDs {
		t := targets[id]
		var group *int64
		switch in.Request.Scope {
		case constants.ScopeGroup:
			g := id
			group = &g
		case constants.ScopeAsset:
			group = t.GroupID
		}
		rows = append(rows, model.TransactionRuleOverride{
			CategoryID: p.categoryID, TransactionType: in.Request.Transaction, Scope: in.Request.Scope,
			TargetID: id, TargetName: t.Name, CategoryPath: path, TargetGroupID: group, Fields: raw,
			CreatedByID: in.ActorID, CreatedByName: in.ActorName, CreatedAt: now,
		})
	}
	return rows
}

// writeOverrides inserts the rows and bumps the settings version once in one transaction. A
// unique violation (a racing submit) answers the 409 with the targets found after the rollback.
func (s *Service) writeOverrides(ctx context.Context, in CreateOverrideInput, p parsedCreate, rows []model.TransactionRuleOverride) error {
	tx, err := s.overrides.BeginOverrideWrite(ctx, in.ClientID)
	if err != nil {
		return err
	}
	defer tx.Close()
	if err := tx.InsertOverrides(ctx, rows); err != nil {
		rbErr := rollbackOverrides(ctx, tx, err)
		if !errors.Is(err, ErrDuplicateOverride) {
			return rbErr
		}
		dups, derr := s.overrides.FindDuplicateTargetIDs(ctx, in.ClientID, in.Request.Scope, in.Request.Transaction, p.categoryID, p.targetIDs)
		if derr != nil || len(dups) == 0 {
			dups = p.targetIDs
		}
		return &DuplicateOverrideError{TargetIDs: orderLike(p.targetIDs, dups)}
	}
	if err := tx.BumpSettingsVersion(ctx, in.ActorID, in.ActorName); err != nil {
		return rollbackOverrides(ctx, tx, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit override create: %w", err)
	}
	return nil
}

func rollbackOverrides(ctx context.Context, tx OverrideTx, cause error) error {
	if err := tx.Rollback(ctx); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback override write: %w", err))
	}
	return cause
}

// dispatchCreateLogs sends one Activity Log entry per created override after commit, best-effort.
func (s *Service) dispatchCreateLogs(ctx context.Context, in CreateOverrideInput, p parsedCreate, rows []model.TransactionRuleOverride, st createState) {
	if s.activityEntries == nil || len(rows) == 0 {
		return
	}
	own := make(map[string]any, len(p.fields))
	for _, f := range p.fields {
		own[f.def.Key] = f.value
	}
	unit := overrideUnit([]map[string]any{own}, st.cat, st.company)
	entries := make([]ActivityLogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, ActivityLogEntry{
			ActorID: in.ActorID, ActorName: in.ActorName, LogTable: LogTableTransactionSettings,
			Action: actionCreate, Object: constants.TransactionLabels[row.TransactionType],
			ObjectName: overrideLogName(row), Field: overrideLogFields(row.TransactionType, own, unit),
		})
	}
	if err := s.activityEntries.DispatchActivityEntries(context.WithoutCancel(ctx), in.ClientID, entries); err != nil {
		log.Printf("transaction settings override activity log dispatch failed: %v", err)
	}
}

// overrideLogMessage is "[Name] <verb> an override on [Transaction] for [Target] in [Category]".
func overrideLogName(row model.TransactionRuleOverride) string {
	category := constants.CategoryAll
	if row.CategoryPath != nil {
		category = *row.CategoryPath
	}
	return row.TargetName + " in " + category
}

// overrideLogFields lists every field pair "Label (value)" comma separated, in catalogue order.
func overrideLogFields(tx string, fields map[string]any, unit string) string {
	var parts []string
	for _, def := range overriddenDefs(tx, fields) {
		parts = append(parts, constants.FieldLabels[def.Key]+" ("+logValue(def, constants.ModeOwn, fields[def.Key], unit)+")")
	}
	return strings.Join(parts, ", ")
}
