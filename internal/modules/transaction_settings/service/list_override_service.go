package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// ListOverridesInput carries the request-scoped inputs of the overrides list.
type ListOverridesInput struct {
	ClientID string
	Request  dto.ListOverridesRequest
	Plan     Plan
	Access   CallerAccess
}

// OverridesOptionsInput carries the request-scoped inputs of the overrides options.
type OverridesOptionsInput struct {
	ClientID string
	Request  dto.OverridesOptionsRequest
	Access   CallerAccess
}

// searchableColumns are the columns the search text matches among the requested ones.
var searchableColumns = []string{"scope", "target", "overridden", "createdBy", "category"}

// ListOverrides lists the overrides of a node and transaction with search, filters, sort and
// pagination done in SQL (LLD list steps 2-8); the per-row display, applied state and canDelete
// are derived here from the page.
func (s *Service) ListOverrides(ctx context.Context, in ListOverridesInput) (dto.ListOverridesData, error) {
	if s.overrides == nil {
		return dto.ListOverridesData{}, ErrOverrideStorageUnavailable
	}
	q, errs := parseListQuery(in.Request)
	if len(errs) > 0 {
		return dto.ListOverridesData{}, &ValidationError{Errors: errs}
	}
	empty, err := s.applyVisibility(ctx, in.ClientID, in.Access, &q)
	if err != nil || empty {
		return dto.ListOverridesData{Data: []dto.OverrideRow{}}, err
	}
	records, total, err := s.overrides.ListOverrides(ctx, in.ClientID, q)
	if err != nil {
		return dto.ListOverridesData{}, err
	}
	rows, err := s.buildListRows(ctx, in, records)
	if err != nil {
		return dto.ListOverridesData{}, err
	}
	return dto.ListOverridesData{Data: rows, TotalRecords: total}, nil
}

// parseListQuery validates the request and resolves it into the SQL filter.
func parseListQuery(req dto.ListOverridesRequest) (dto.OverrideQuery, []dto.FieldError) {
	var errs []dto.FieldError
	q := dto.OverrideQuery{Transaction: req.Transaction, SortBy: "createdAt", SortOrder: -1, Page: 1, Limit: 10}
	if req.Columns == nil {
		errs = append(errs, dto.FieldError{Field: "columns", Message: msgPickColumns})
	}
	if req.Page != nil {
		q.Page = *req.Page
		if q.Page < 1 {
			errs = append(errs, dto.FieldError{Field: "page", Message: msgMinOne})
		}
	}
	if req.Limit != nil {
		q.Limit = *req.Limit
		if !slices.Contains(constants.PageSizes, q.Limit) {
			errs = append(errs, dto.FieldError{Field: "limit", Message: msgInvalidOption})
		}
	}
	if req.SortBy != nil {
		q.SortBy = *req.SortBy
		if !slices.Contains(constants.OverrideSortKeys, q.SortBy) {
			errs = append(errs, dto.FieldError{Field: "sortBy", Message: msgInvalidOption})
		}
	}
	if req.SortOrder != nil {
		q.SortOrder = *req.SortOrder
		if q.SortOrder != 1 && q.SortOrder != -1 {
			errs = append(errs, dto.FieldError{Field: "sortOrder", Message: msgInvalidOption})
		}
	}
	errs = append(errs, parseNodeFilter(&q, req.Transaction, req.CategoryID, req.Category, req.CreatedAt)...)
	for _, sc := range req.Scope {
		if !isScope(sc) {
			errs = append(errs, dto.FieldError{Field: "scope", Message: msgInvalidChoice})
			break
		}
	}
	q.Scopes = req.Scope
	q.CreatedBy = req.CreatedBy
	q.OverriddenKeys = keysForLabels(req.Transaction, req.Overridden)
	if len(req.Overridden) > 0 && len(q.OverriddenKeys) == 0 {
		q.OverriddenKeys = []string{}
	}
	if q.Search = strings.TrimSpace(req.Search); q.Search != "" {
		for _, col := range req.Columns {
			if slices.Contains(searchableColumns, col) {
				q.SearchColumns = append(q.SearchColumns, col)
			}
		}
		q.SearchFieldKeys = keysContaining(req.Transaction, q.Search)
	}
	return q, errs
}

// parseNodeFilter validates the filters shared by list and options: transaction, node, Category
// ids and the createdAt range.
func parseNodeFilter(q *dto.OverrideQuery, tx string, categoryID *string, categories []string, createdAt json.RawMessage) []dto.FieldError {
	var errs []dto.FieldError
	if !slices.Contains(constants.TransactionOrder, tx) {
		errs = append(errs, dto.FieldError{Field: "transaction", Message: msgInvalidChoice})
	}
	if categoryID != nil && *categoryID != "" {
		id, ok := parseSnowflake(*categoryID)
		if !ok {
			errs = append(errs, dto.FieldError{Field: "categoryId", Message: msgInvalidID})
		}
		q.CategoryID = &id
	}
	for i, raw := range categories {
		id, ok := parseSnowflake(raw)
		if !ok {
			errs = append(errs, dto.FieldError{Field: "category[" + strconv.Itoa(i) + "]", Message: msgInvalidID})
			continue
		}
		q.CategoryIDs = append(q.CategoryIDs, id)
	}
	trimmed := bytes.TrimSpace(createdAt)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return errs
	}
	var r dto.DateRangeFilter
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.DisallowUnknownFields()
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &probe); err != nil || dec.Decode(&r) != nil || probe["from"] == nil || probe["to"] == nil || r.From > r.To {
		return append(errs, dto.FieldError{Field: "createdAt", Message: msgInvalidRange})
	}
	q.CreatedFrom, q.CreatedTo = &r.From, &r.To
	return errs
}

// keysForLabels maps display names to the field keys of the transaction.
func keysForLabels(tx string, labels []string) []string {
	var out []string
	for _, def := range constants.Catalogue[tx] {
		if slices.Contains(labels, constants.FieldLabels[def.Key]) {
			out = append(out, def.Key)
		}
	}
	return out
}

// keysContaining returns the field keys of the transaction whose display name contains text.
func keysContaining(tx, text string) []string {
	var out []string
	needle := strings.ToLower(text)
	for _, def := range constants.Catalogue[tx] {
		if strings.Contains(strings.ToLower(constants.FieldLabels[def.Key]), needle) {
			out = append(out, def.Key)
		}
	}
	return out
}

// applyVisibility restricts a caller whose rights come only from Group Roles to the rows inside
// those Groups (SD-50); empty = the caller sees no row.
func (s *Service) applyVisibility(ctx context.Context, clientID string, a CallerAccess, q *dto.OverrideQuery) (bool, error) {
	if a.ViewAll {
		return false, nil
	}
	if len(a.ViewGroupIDs) == 0 {
		return true, nil
	}
	if s.groupScope == nil {
		return false, ErrOverrideDirectoryUnavailable
	}
	groups, err := s.groupScope.ExpandGroups(ctx, clientID, a.ViewGroupIDs)
	if err != nil {
		return false, fmt.Errorf("expand view groups: %w", err)
	}
	if groups == nil {
		groups = []int64{}
	}
	q.VisibleGroupIDs = &groups
	return false, nil
}

// OverrideOptions returns the filter options of the overrides table (LLD options steps 2-4).
func (s *Service) OverrideOptions(ctx context.Context, in OverridesOptionsInput) (dto.OverridesOptionsData, error) {
	if s.overrides == nil {
		return dto.OverridesOptionsData{}, ErrOverrideStorageUnavailable
	}
	var q dto.OverrideQuery
	if errs := parseNodeFilter(&q, in.Request.Transaction, in.Request.CategoryID, in.Request.Category, in.Request.CreatedAt); len(errs) > 0 {
		return dto.OverridesOptionsData{}, &ValidationError{Errors: errs}
	}
	q.Transaction = in.Request.Transaction
	q.WantScope, q.WantOverridden, q.WantCreatedBy = in.Request.Scope, in.Request.Overridden, in.Request.CreatedBy
	var out dto.OverridesOptionsData
	empty, err := s.applyVisibility(ctx, in.ClientID, in.Access, &q)
	if err != nil {
		return out, err
	}
	var vals OverrideOptionValues
	if !empty {
		if vals, err = s.overrides.FindOverrideOptions(ctx, in.ClientID, q); err != nil {
			return out, err
		}
	}
	if q.WantScope {
		items := optionItems(scopeNames(vals.Scopes))
		out.Scope = &items
	}
	if q.WantOverridden {
		items := optionItems(fieldNames(vals.FieldKeys))
		out.Overridden = &items
	}
	if q.WantCreatedBy {
		items := optionItems(vals.CreatedBy)
		out.CreatedBy = &items
	}
	return out, nil
}

func scopeNames(scopes []string) []string {
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		out = append(out, constants.ScopeLabels[sc])
	}
	return out
}

func fieldNames(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		label, ok := constants.FieldLabels[k]
		if !ok {
			label = k
		}
		out = append(out, label)
	}
	return out
}

// optionItems returns the distinct names sorted, each as {label, value} with the name in both.
func optionItems(names []string) []dto.OptionItem {
	sorted := slices.Clone(names)
	sort.Strings(sorted)
	sorted = slices.Compact(sorted)
	out := make([]dto.OptionItem, 0, len(sorted))
	for _, n := range sorted {
		out = append(out, dto.OptionItem{Label: n, Value: n})
	}
	return out
}

// listLookups are the provider answers the rows of one page need.
type listLookups struct {
	removedTargets map[string]map[int64]bool // scope -> target id -> removed
	removedCats    map[int64]bool
	writeGroups    map[int64]bool // the caller's Update Groups with descendants
	company        fieldIndex
}

func (s *Service) buildListRows(ctx context.Context, in ListOverridesInput, recs []dto.OverrideListRecord) ([]dto.OverrideRow, error) {
	rows := make([]dto.OverrideRow, 0, len(recs))
	if len(recs) == 0 {
		return rows, nil
	}
	lk, err := s.loadListLookups(ctx, in, recs)
	if err != nil {
		return nil, err
	}
	for _, rec := range recs {
		rows = append(rows, listRow(rec, in, lk))
	}
	return rows, nil
}

func (s *Service) loadListLookups(ctx context.Context, in ListOverridesInput, recs []dto.OverrideListRecord) (listLookups, error) {
	lk := listLookups{removedTargets: map[string]map[int64]bool{}, removedCats: map[int64]bool{}, writeGroups: map[int64]bool{}}
	if s.targets == nil {
		return lk, ErrOverrideDirectoryUnavailable
	}
	idsByScope := map[string][]int64{}
	catIDs := map[int64]bool{}
	for _, rec := range recs {
		if id, ok := parseSnowflake(rec.TargetID); ok {
			idsByScope[rec.Scope] = append(idsByScope[rec.Scope], id)
		}
		if rec.CategoryID != nil {
			if id, ok := parseSnowflake(*rec.CategoryID); ok {
				catIDs[id] = true
			}
		}
	}
	for scope, ids := range idsByScope {
		list, err := s.targets.ResolveTargets(ctx, in.ClientID, scope, ids)
		if err != nil {
			return lk, fmt.Errorf("resolve override targets: %w", err)
		}
		alive := map[int64]bool{}
		for _, t := range list {
			alive[t.ID] = t.Exists
		}
		lk.removedTargets[scope] = map[int64]bool{}
		for _, id := range ids {
			lk.removedTargets[scope][id] = !alive[id]
		}
	}
	if len(catIDs) > 0 {
		if s.categories == nil {
			return lk, ErrOverrideDirectoryUnavailable
		}
		for id := range catIDs {
			info, err := s.categories.LookupCategory(ctx, in.ClientID, id)
			if err != nil {
				return lk, fmt.Errorf("look up category: %w", err)
			}
			// A deleted Category, or one that is not a leaf, leaves its rows not applied.
			lk.removedCats[id] = !info.Exists || !info.Leaf
		}
	}
	if in.Access.canWriteAny() && !in.Access.TotalControl && len(in.Access.WriteGroupIDs) > 0 {
		if s.groupScope == nil {
			return lk, ErrOverrideDirectoryUnavailable
		}
		groups, err := s.groupScope.ExpandGroups(ctx, in.ClientID, in.Access.WriteGroupIDs)
		if err != nil {
			return lk, fmt.Errorf("expand write groups: %w", err)
		}
		for _, g := range groups {
			lk.writeGroups[g] = true
		}
	}
	company, err := s.repo.FindNodeFields(ctx, in.ClientID, nil)
	if err != nil {
		return lk, fmt.Errorf("read company default fields: %w", err)
	}
	lk.company = newFieldIndex(company)
	return lk, nil
}

func listRow(rec dto.OverrideListRecord, in ListOverridesInput, lk listLookups) dto.OverrideRow {
	fields := decodeOverrideFields(rec.Fields)
	unit := overrideUnit([]map[string]any{fields}, nil, lk.company)
	row := dto.OverrideRow{
		ID: rec.ID, Scope: rec.Scope, Target: rec.Target, Category: rec.Category, Transaction: rec.Transaction,
		Overridden: []dto.OverriddenField{}, Applied: true, CreatedBy: rec.CreatedBy, CreatedAt: rec.CreatedAt,
	}
	gated := false
	for _, def := range overriddenDefs(rec.Transaction, fields) {
		row.Overridden = append(row.Overridden, dto.OverriddenField{
			Key: def.Key, Label: constants.FieldLabels[def.Key], Value: fields[def.Key],
			DisplayValue: logValue(def, constants.ModeOwn, fields[def.Key], unit),
		})
		gated = gated || fieldGated(rec.Transaction, def.Key, in.Plan)
	}
	targetID, _ := parseSnowflake(rec.TargetID)
	removed := lk.removedTargets[rec.Scope][targetID]
	if rec.CategoryID != nil {
		if id, ok := parseSnowflake(*rec.CategoryID); ok && lk.removedCats[id] {
			removed = true
		}
	}
	switch {
	case removed:
		reason := constants.NotAppliedRemoved
		row.Applied, row.NotAppliedReason = false, &reason
	case gated:
		reason := constants.NotAppliedPlan
		row.Applied, row.NotAppliedReason = false, &reason
	}
	row.CanDelete = canDeleteRow(in.Access, rec, lk)
	return row
}

// canDeleteRow is true when the caller holds an Update right and its scope covers the row (the
// list reads the snapshot Group of an Asset override).
func canDeleteRow(a CallerAccess, rec dto.OverrideListRecord, lk listLookups) bool {
	if !a.canWriteScope(rec.Scope) {
		return false
	}
	if a.TotalControl {
		return true
	}
	switch rec.Scope {
	case constants.ScopeAssetName:
		return a.AssetNameWrite
	case constants.ScopeGroup:
		id, _ := parseSnowflake(rec.TargetID)
		return lk.writeGroups[id]
	}
	if rec.GroupID == nil {
		return false
	}
	id, _ := parseSnowflake(*rec.GroupID)
	return lk.writeGroups[id]
}
