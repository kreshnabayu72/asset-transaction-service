package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strconv"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// SaveInput carries the request-scoped inputs of a Save.
type SaveInput struct {
	ClientID  string
	Request   dto.SaveSettingsRequest
	Plan      Plan
	ActorID   int64
	ActorName string
}

// SaveResult says whether the Save changed anything (false answers "No changes to save.").
type SaveResult struct{ Changed bool }

// ValidationError is the 422 error list of a rejected Save; nothing was written.
type ValidationError struct{ Errors []dto.FieldError }

func (e *ValidationError) Error() string { return "validation failed" }

// allowKeys is the Allow transaction field of each transaction that has one.
var allowKeys = map[string]string{
	constants.TransactionBorrow:   constants.FieldAllowBorrow,
	constants.TransactionAssign:   constants.FieldAllowAssign,
	constants.TransactionTransfer: constants.FieldAllowTransfer,
	constants.TransactionDisposal: constants.FieldAllowDisposal,
}

// candidate is one field of the payload after structural checks.
type candidate struct {
	idx      int // position of the field in the payload, counted across all changes
	cat      *int64
	tx       string
	def      constants.FieldDef
	mode     string
	value    any
	parseMsg string
}

// changedField is a candidate that differs from the stored state.
type changedField struct {
	candidate
	oldMode  string
	oldValue any
}

// SaveTransactionSettings validates the whole draft, compares it to the stored rules and, when
// something differs, writes the changed fields and bumps the settings version in one transaction
// (LLD PATCH steps 3-8, 11). Changelog and Activity Log entries are dispatched after commit.
func (s *Service) SaveTransactionSettings(ctx context.Context, in SaveInput) (SaveResult, error) {
	cands, cats, errs := collectCandidates(in.Request)
	st, err := s.loadState(ctx, in.ClientID, cands)
	if err != nil {
		return SaveResult{}, err
	}
	cands = st.admit(cands, &errs)
	guardAssignees(st, cands, in.Plan, &errs)
	if len(errs) > 0 {
		return SaveResult{}, &ValidationError{Errors: errs}
	}
	changes := st.diff(cands)
	for _, c := range changes {
		if fieldGated(c.tx, c.def.Key, in.Plan) {
			errs = append(errs, fieldError(c.idx, "value", msgFieldGated))
		}
	}
	if len(errs) > 0 {
		return SaveResult{}, &ValidationError{Errors: errs}
	}
	// Step 5 answers after the validation list: a Category that is unknown, deleted or of another
	// entity is not found, and nothing is written.
	infos, err := s.lookupCategories(ctx, in.ClientID, cats)
	if err != nil {
		return SaveResult{}, err
	}
	if len(changes) == 0 {
		return SaveResult{}, nil
	}
	if err := s.writeChanges(ctx, in, changes); err != nil {
		return SaveResult{}, err
	}
	s.dispatchLogs(ctx, in, st, changes, infos)
	return SaveResult{Changed: true}, nil
}

func fieldError(idx int, attr, msg string) dto.FieldError {
	return dto.FieldError{Field: "fields[" + itoa(idx) + "]." + attr, Message: msg}
}

type catRef struct{ id int64 }

// collectCandidates runs the structural checks of steps 3 and 5 and parses every own value. A later
// payload entry for the same node, transaction and key replaces an earlier one.
func collectCandidates(req dto.SaveSettingsRequest) ([]candidate, []catRef, []dto.FieldError) {
	var (
		out  []candidate
		cats []catRef
		errs []dto.FieldError
		idx  int
	)
	for ci, ch := range req.Changes {
		cat, ok := parseNodeCategory(ch.CategoryID)
		first := idx
		idx += len(ch.Fields)
		if !ok {
			errs = append(errs, dto.FieldError{Field: "changes[" + itoa(ci) + "].categoryId", Message: msgInvalidCategoryID})
			continue
		}
		if cat != nil {
			cats = append(cats, catRef{id: *cat})
			if slices.Contains(constants.AllCategoriesOnlyTransactions, ch.Transaction) {
				errs = append(errs, dto.FieldError{Field: "changes[" + itoa(ci) + "].transaction", Message: msgAllCategoriesOnlyTx})
				continue
			}
		}
		for j, f := range ch.Fields {
			c, fe := newCandidate(first+j, cat, ch.Transaction, f)
			if fe != nil {
				errs = append(errs, *fe)
				continue
			}
			out = appendReplacing(out, c)
		}
	}
	return out, cats, errs
}

func parseNodeCategory(raw *string) (*int64, bool) {
	if raw == nil || *raw == "" {
		return nil, true
	}
	id, err := strconv.ParseInt(*raw, 10, 64)
	if err != nil || id <= 0 || *raw != strconv.FormatInt(id, 10) {
		return nil, false
	}
	return &id, true
}

func newCandidate(idx int, cat *int64, tx string, f dto.SaveField) (candidate, *dto.FieldError) {
	def, ok := findDef(tx, f.Key)
	if !ok {
		fe := fieldError(idx, "value", msgUnknownField)
		return candidate{}, &fe
	}
	switch {
	case cat == nil && f.Mode == constants.ModeInherit:
		fe := fieldError(idx, "value", msgInheritNotAllowed)
		return candidate{}, &fe
	case cat != nil && def.AllCategoriesOnly:
		fe := fieldError(idx, "value", msgOwnNotAllowed)
		return candidate{}, &fe
	}
	c := candidate{idx: idx, cat: cat, tx: tx, def: def, mode: f.Mode}
	if f.Mode == constants.ModeOwn {
		c.value, c.parseMsg = parseValue(ruleFor(def), f.Value)
	}
	return c, nil
}

func findDef(tx, key string) (constants.FieldDef, bool) {
	for _, d := range constants.Catalogue[tx] {
		if d.Key == key {
			return d, true
		}
	}
	return constants.FieldDef{}, false
}

func sameNode(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func appendReplacing(list []candidate, c candidate) []candidate {
	for i := range list {
		if sameNode(list[i].cat, c.cat) && list[i].tx == c.tx && list[i].def.Key == c.def.Key {
			list[i] = c
			return list
		}
	}
	return append(list, c)
}

// ErrCategoryNotFound maps to 404 (LLD PATCH step 5): a categoryId that is unknown, deleted or of
// another entity; nothing is written. A parent Category is accepted.
var ErrCategoryNotFound = errors.New("category not found")

// lookupCategories resolves each distinct Category; the directory answers Exists=false for an unknown,
// deleted or other-entity id, which answers ErrCategoryNotFound before any validation.
func (s *Service) lookupCategories(ctx context.Context, clientID string, cats []catRef) (map[int64]CategoryInfo, error) {
	infos := map[int64]CategoryInfo{}
	if s.categories == nil {
		return infos, nil
	}
	for _, ref := range cats {
		if _, done := infos[ref.id]; done {
			continue
		}
		info, err := s.categories.LookupCategory(ctx, clientID, ref.id)
		if err != nil {
			return nil, fmt.Errorf("look up category: %w", err)
		}
		if !info.Exists {
			return nil, ErrCategoryNotFound
		}
		infos[ref.id] = info
	}
	return infos, nil
}

// saveState is the stored rules of the nodes a Save touches plus the pending payload values.
type saveState struct {
	stored  map[string]fieldIndex
	pending map[string]map[string]candidate
}

func nodeID(cat *int64) string {
	if cat == nil {
		return "all"
	}
	return "c" + strconv.FormatInt(*cat, 10)
}

func (s *Service) loadState(ctx context.Context, clientID string, cands []candidate) (*saveState, error) {
	st := &saveState{stored: map[string]fieldIndex{}, pending: map[string]map[string]candidate{}}
	nodes := map[string]*int64{"all": nil}
	for _, c := range cands {
		nodes[nodeID(c.cat)] = c.cat
	}
	for id, cat := range nodes {
		rows, err := s.repo.FindNodeFields(ctx, clientID, cat)
		if err != nil {
			return nil, fmt.Errorf("read stored fields: %w", err)
		}
		st.stored[id] = newFieldIndex(rows)
	}
	return st, nil
}

// effective is the effective value of a field at a node: the pending own value, else the stored
// own value, else (Category) the Company default value, else the catalogue default.
func (st *saveState) effective(cat *int64, tx string, def constants.FieldDef) any {
	id := nodeID(cat)
	if p, ok := st.pending[id][indexKey(tx, def.Key)]; ok && p.mode == constants.ModeOwn {
		return p.value
	}
	if cat == nil {
		return companyValue(st.stored["all"], tx, def)
	}
	if _, pendingInherit := st.pending[id][indexKey(tx, def.Key)]; !pendingInherit {
		if v, ok := st.stored[id].own(tx, def.Key); ok {
			return v
		}
	}
	return st.effective(nil, tx, def)
}

// hidden reports whether the field is hidden by Allow = Deny or by Require approval OFF in the
// effective state of its node (LLD PATCH step 3: such a field is skipped by validation).
func (st *saveState) hidden(c candidate) bool {
	if allow, ok := allowKeys[c.tx]; ok && c.def.Key != allow {
		if def, found := findDef(c.tx, allow); found && st.effective(c.cat, c.tx, def) == false {
			return true
		}
	}
	if c.def.Key == constants.FieldAutoApproveMaxDuration {
		if def, found := findDef(c.tx, constants.FieldRequireApproval); found && st.effective(c.cat, c.tx, def) == false {
			return true
		}
	}
	return false
}

// admit moves valid candidates into the pending state, drops invalid values of hidden fields
// (their stored value is kept) and reports every other invalid value.
func (st *saveState) admit(cands []candidate, errs *[]dto.FieldError) []candidate {
	for _, c := range cands {
		if c.parseMsg != "" {
			continue
		}
		id := nodeID(c.cat)
		if st.pending[id] == nil {
			st.pending[id] = map[string]candidate{}
		}
		st.pending[id][indexKey(c.tx, c.def.Key)] = c
	}
	kept := make([]candidate, 0, len(cands))
	for _, c := range cands {
		switch {
		case c.parseMsg == "":
			kept = append(kept, c)
		case st.hidden(c):
		default:
			*errs = append(*errs, fieldError(c.idx, "value", c.parseMsg))
		}
	}
	return kept
}

// guardAssignees rejects a node whose effective Allowed assignee: Employee and Division are both
// OFF while Allow Assign is not Deny (not evaluated at Assignment Basic, where Employee is
// effectively ON). Existing overrides are not re-checked here (overrides storage format is owned
// by the overrides operations).
func guardAssignees(st *saveState, cands []candidate, plan Plan, errs *[]dto.FieldError) {
	if transactionLevel(constants.TransactionAssign, plan) != constants.LevelFull {
		return
	}
	seen := map[string]bool{}
	for _, c := range cands {
		id := nodeID(c.cat)
		if c.tx != constants.TransactionAssign || seen[id] {
			continue
		}
		seen[id] = true
		emp, _ := findDef(c.tx, constants.FieldAllowedAssigneeEmployee)
		div, _ := findDef(c.tx, constants.FieldAllowedAssigneeDivision)
		allow, _ := findDef(c.tx, constants.FieldAllowAssign)
		if st.effective(c.cat, c.tx, allow) == false || st.effective(c.cat, c.tx, emp) != false || st.effective(c.cat, c.tx, div) != false {
			continue
		}
		last := c.idx
		for _, o := range cands {
			if o.tx == c.tx && sameNode(o.cat, c.cat) && o.idx > last {
				last = o.idx
			}
		}
		for _, key := range []string{constants.FieldAllowedAssigneeEmployee, constants.FieldAllowedAssigneeDivision} {
			at := last
			if p, ok := st.pending[id][indexKey(c.tx, key)]; ok {
				at = p.idx
			}
			*errs = append(*errs, fieldError(at, "value", msgNoAssigneeType))
		}
	}
}

// stored returns the mode and value stored for a field; an unstored Company field reads as its own
// default, an unstored Category field as Inherit.
func (st *saveState) storedState(c candidate) (string, any) {
	if row, ok := st.stored[nodeID(c.cat)][indexKey(c.tx, c.def.Key)]; ok {
		if row.Mode == constants.ModeOwn && row.Value != nil {
			return constants.ModeOwn, decodeStored(*row.Value)
		}
		return row.Mode, nil
	}
	if c.cat == nil {
		return constants.ModeOwn, c.def.Default
	}
	return constants.ModeInherit, nil
}

// diff keeps the candidates whose mode or value differs from the stored state (step 6).
func (st *saveState) diff(cands []candidate) []changedField {
	var out []changedField
	for _, c := range cands {
		oldMode, oldValue := st.storedState(c)
		if c.mode == oldMode && (c.mode == constants.ModeInherit || sameValue(c.value, oldValue)) {
			continue
		}
		out = append(out, changedField{candidate: c, oldMode: oldMode, oldValue: oldValue})
	}
	return out
}

// fieldGated reports whether the plan or an inactive addon rejects a change of the field.
func fieldGated(tx, key string, plan Plan) bool {
	return transactionLevel(tx, plan) == constants.LevelNone || isGated(tx, key, plan)
}

func (s *Service) writeChanges(ctx context.Context, in SaveInput, changes []changedField) error {
	now := time.Now()
	rows := make([]model.TransactionRuleField, 0, len(changes))
	for _, c := range changes {
		row := model.TransactionRuleField{
			CategoryID: c.cat, TransactionType: c.tx, FieldKey: c.def.Key, Mode: c.mode,
			ModifiedByID: &in.ActorID, ModifiedByName: &in.ActorName, CreatedAt: now, UpdatedAt: now,
		}
		if c.mode == constants.ModeOwn {
			// c.value is an int, bool, string or []string: Marshal cannot fail on those.
			raw, _ := json.Marshal(c.value)
			msg := json.RawMessage(raw)
			row.Value = &msg
		}
		rows = append(rows, row)
	}
	tx, err := s.repo.BeginSave(ctx, in.ClientID)
	if err != nil {
		return err
	}
	defer tx.Close()
	if err := tx.UpsertFields(ctx, rows); err != nil {
		return rollbackWith(ctx, tx, err)
	}
	if err := tx.BumpSettingsVersion(ctx, in.ActorID, in.ActorName); err != nil {
		return rollbackWith(ctx, tx, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit settings save: %w", err)
	}
	return nil
}

func rollbackWith(ctx context.Context, tx SaveTx, cause error) error {
	if err := tx.Rollback(ctx); err != nil {
		return errors.Join(cause, fmt.Errorf("rollback settings save: %w", err))
	}
	return cause
}

// dispatchLogs sends one Changelog document and one Activity Log entry per changed field after
// commit, best-effort: a failure is logged and does not change the response.
func (s *Service) dispatchLogs(ctx context.Context, in SaveInput, st *saveState, changes []changedField, infos map[int64]CategoryInfo) {
	logCtx := context.WithoutCancel(ctx)
	entries := make([]ChangeLogEntry, 0, len(changes))
	activity := make([]ActivityLogEntry, 0, len(changes))
	for _, c := range changes {
		entity := constants.NodeAllCategories
		if c.cat != nil {
			entity = strconv.FormatInt(*c.cat, 10)
			if info, ok := infos[*c.cat]; ok && info.Path != "" {
				entity = info.Path
			}
		}
		unit := ""
		if c.def.Unit == constants.DurationUnit {
			unitDef, _ := findDef(constants.TransactionBorrow, constants.FieldDurationUnit)
			unit, _ = st.effective(c.cat, c.tx, unitDef).(string)
		}
		oldText := logValue(c.def, c.oldMode, c.oldValue, unit)
		newText := logValue(c.def, c.mode, c.value, unit)
		transaction := constants.TransactionLabels[c.tx]
		field := constants.FieldLabels[c.def.Key]
		entries = append(entries, ChangeLogEntry{
			Action: actionEdit, Entity: entity, ObjectPath: LogTableTransactionSettings,
			Field: transaction + " (" + field + ")", OldValue: oldText, NewValue: newText,
			ModifiedBy: in.ActorName, CreatedByID: in.ActorID, LogTable: LogTableTransactionSettings,
		})
		activity = append(activity, ActivityLogEntry{
			ActorID: in.ActorID, ActorName: in.ActorName, LogTable: LogTableTransactionSettings,
			Action: actionEdit, Object: transaction, ObjectName: entity, Field: field,
			OldValue: oldText, NewValue: newText,
		})
	}
	if s.changeLogs != nil {
		if err := s.changeLogs.DispatchChangeLogs(logCtx, in.ClientID, entries); err != nil {
			log.Printf("transaction settings changelog dispatch failed: %v", err)
		}
	}
	if s.activityEntries != nil {
		if err := s.activityEntries.DispatchActivityEntries(logCtx, in.ClientID, activity); err != nil {
			log.Printf("transaction settings activity log dispatch failed: %v", err)
		}
	}
}

// logValue renders a value for Changelog and Activity Log (LLD PATCH Logs Written).
func logValue(def constants.FieldDef, mode string, value any, durationUnit string) string {
	if mode == constants.ModeInherit {
		return constants.ModeInheritLabel
	}
	switch v := value.(type) {
	case bool:
		labels := def.BoolLabels
		if labels == ([2]string{}) {
			labels = [2]string{"ON", "OFF"}
		}
		if v {
			return labels[0]
		}
		return labels[1]
	case int:
		return logNumber(def, v, durationUnit)
	case string:
		if label, ok := constants.ValueLabels[v]; ok {
			return label
		}
		return v
	case []string:
		out := ""
		for i, t := range optionOrder(v) {
			if i > 0 {
				out += ", "
			}
			out += constants.ValueLabels[t]
		}
		return out
	}
	return fmt.Sprint(value)
}

func logNumber(def constants.FieldDef, v int, durationUnit string) string {
	if v == 0 {
		if text, ok := constants.ZeroTexts[def.Key]; ok {
			return text
		}
	}
	switch def.Unit {
	case "":
		return itoa(v)
	case constants.DurationUnit:
		return itoa(v) + " " + durationUnit
	case "%":
		return itoa(v) + "%"
	default:
		return itoa(v) + " " + def.Unit
	}
}
