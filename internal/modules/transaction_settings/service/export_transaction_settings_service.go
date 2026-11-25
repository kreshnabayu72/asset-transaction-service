package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// ExportFileName is the download name of the export workbook.
const ExportFileName = "Transaction-Rules.xlsx"

// objectTransactionRules is the Activity Log object of an export (LLD export Logs Written: "transaction rules").
const objectTransactionRules = "Transaction Rules"

const (
	exportTimeLayout = "2006-01-02 15:04:05"
	emptyCell        = "-"
)

// ExportInput carries the request-scoped inputs of an export.
type ExportInput struct {
	ClientID   string
	CategoryID *int64
	Plan       Plan
	ActorID    int64
	ActorName  string
	ExportedAt time.Time
}

// ExportTransactionSettings builds the Rules, Overrides and Info sheets of the node from the
// stored rows, renders them into one workbook and dispatches the Activity Log entry after the file is built.
func (s *Service) ExportTransactionSettings(ctx context.Context, in ExportInput) (*dto.ExportFile, error) {
	if s.renderer == nil {
		return nil, ErrExportRendererUnavailable
	}
	info, err := s.resolveLeafCategory(ctx, in.ClientID, in.CategoryID)
	if err != nil {
		return nil, err
	}
	category := categoryLabel(in.CategoryID, info)
	version, err := s.repo.FindSettingsVersion(ctx, in.ClientID)
	if err != nil {
		return nil, fmt.Errorf("read settings version: %w", err)
	}
	fields, err := s.repo.FindExportFields(ctx, in.ClientID, in.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("read export fields: %w", err)
	}
	overrides, err := s.repo.FindExportOverrides(ctx, in.ClientID, in.CategoryID)
	if err != nil {
		return nil, fmt.Errorf("read export overrides: %w", err)
	}
	sheets := []dto.Sheet{
		rulesSheet(category, in.CategoryID, in.Plan, fields),
		overridesSheet(in.Plan, overrides),
		infoSheet(category, in, version),
	}
	data, err := s.renderer.Render(sheets)
	if err != nil {
		return nil, fmt.Errorf("render transaction rules workbook: %w", err)
	}
	s.dispatchExportActivityLog(ctx, in, category)
	return &dto.ExportFile{FileName: ExportFileName, Data: data}, nil
}

// categoryLabel is the Category of an export row: "All categories" for a nil id, else the full path from the
// lookup (the id itself when the directory is unset).
func categoryLabel(id *int64, info CategoryInfo) string {
	switch {
	case id == nil:
		return constants.NodeAllCategories
	case info.Path != "":
		return info.Path
	default:
		return strconv.FormatInt(*id, 10)
	}
}

// rulesSheet lists one row per node x transaction x field for transactions available on the plan.
// A leaf export lists that Category; an All categories export lists the Company default, then every
// Category that has stored rows. A field without an own value at a Category shows the Company default.
func rulesSheet(category string, categoryID *int64, plan Plan, rows []model.TransactionRuleField) dto.Sheet {
	sheet := dto.Sheet{Name: "Rules", Headers: []string{"Category", "Transaction", "Field", "Value", "Unit", "Source"}, Rows: [][]any{}}
	var companyRows []model.TransactionRuleField
	byCategory := map[int64][]model.TransactionRuleField{}
	var order []int64
	for _, r := range rows {
		if r.CategoryID == nil {
			companyRows = append(companyRows, r)
			continue
		}
		if _, seen := byCategory[*r.CategoryID]; !seen {
			order = append(order, *r.CategoryID)
		}
		byCategory[*r.CategoryID] = append(byCategory[*r.CategoryID], r)
	}
	company := newFieldIndex(companyRows)
	if categoryID != nil {
		return appendRules(sheet, category, true, plan, company, newFieldIndex(byCategory[*categoryID]))
	}
	sheet = appendRules(sheet, category, false, plan, company, nil)
	for _, id := range order {
		sheet = appendRules(sheet, strconv.FormatInt(id, 10), true, plan, company, newFieldIndex(byCategory[id]))
	}
	return sheet
}

func appendRules(sheet dto.Sheet, label string, atLeaf bool, plan Plan, company, own fieldIndex) dto.Sheet {
	for _, tx := range constants.TransactionOrder {
		if (atLeaf && slices.Contains(constants.AllCategoriesOnlyTransactions, tx)) || transactionLevel(tx, plan) == constants.LevelNone {
			continue
		}
		durationUnit := nodeDurationUnit(tx, company, own)
		for _, f := range constants.Catalogue[tx] {
			if atLeaf && f.AllCategoriesOnly {
				continue
			}
			value, source := companyValue(company, tx, f), constants.CompanyDefaultSource
			if v, ok := own.own(tx, f.Key); ok {
				value, source = v, label
			}
			sheet.Rows = append(sheet.Rows, []any{label, tx, f.Key, fieldCell(f, value), exportUnit(resolveUnit(f.Unit, durationUnit)), source})
		}
	}
	return sheet
}

// nodeDurationUnit is the Duration unit token of tx at a node: its own value, else the Company default.
func nodeDurationUnit(tx string, company, own fieldIndex) string {
	if v, ok := own.own(tx, constants.FieldDurationUnit); ok {
		if u, isStr := v.(string); isStr {
			return u
		}
	}
	if v, ok := company.own(tx, constants.FieldDurationUnit); ok {
		if u, isStr := v.(string); isStr {
			return u
		}
	}
	return constants.ValueDays
}

// resolveUnit turns the Duration-bound sentinel into the node's Duration unit token.
func resolveUnit(unit, durationUnit string) string {
	if unit == constants.DurationUnit {
		return durationUnit
	}
	return unit
}

// fieldCell formats a value with the field's own labels (Allow/Deny, Yes/No, choice labels).
func fieldCell(f constants.FieldDef, v any) any {
	if b, ok := v.(bool); ok && f.BoolLabels[0] != "" {
		if b {
			return f.BoolLabels[0]
		}
		return f.BoolLabels[1]
	}
	switch val := v.(type) {
	case string:
		return exportCell(valueLabel(val))
	case []string:
		labels := make([]string, len(val))
		for i, item := range val {
			labels[i] = valueLabel(item)
		}
		return exportCell(labels)
	}
	return exportCell(v)
}

func fieldDef(tx, key string) constants.FieldDef {
	for _, f := range constants.Catalogue[tx] {
		if f.Key == key {
			return f
		}
	}
	return constants.FieldDef{Key: key}
}

// overridesSheet lists one row per changed field of each stored override, omitting transactions
// whose Feature Suite level is None. The not-applied status (LLD export step 4: a deleted target or
// Category, or a plan-gated field) is not computed yet, so every row reads Applied (open gap).
func overridesSheet(plan Plan, overrides []model.TransactionRuleOverride) dto.Sheet {
	sheet := dto.Sheet{
		Name:    "Overrides",
		Headers: []string{"Scope", "Target", "Category", "Transaction", "Field", "Value", "Unit", "Status", "Created By", "Created At"},
		Rows:    [][]any{},
	}
	for _, o := range overrides {
		if transactionLevel(o.TransactionType, plan) == constants.LevelNone {
			continue
		}
		var values map[string]json.RawMessage
		if err := json.Unmarshal(o.Fields, &values); err != nil {
			continue
		}
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			def := fieldDef(o.TransactionType, k)
			sheet.Rows = append(sheet.Rows, []any{
				scopeLabel(o.Scope), exportCell(o.TargetName), overrideCategory(o), o.TransactionType, k,
				fieldCell(def, decodeStored(values[k])), exportUnit(resolveUnit(def.Unit, overrideDurationUnit(values))), "Applied",
				exportCell(o.CreatedByName), o.CreatedAt.Format(exportTimeLayout),
			})
		}
	}
	return sheet
}

func scopeLabel(scope string) string {
	switch scope {
	case "group":
		return "Group"
	case "asset_name":
		return "Asset Name"
	case "asset":
		return "Asset"
	default:
		return scope
	}
}

func overrideCategory(o model.TransactionRuleOverride) string {
	switch {
	case o.CategoryID == nil:
		return constants.NodeAllCategories
	case o.CategoryPath != nil && *o.CategoryPath != "":
		return *o.CategoryPath
	default:
		return strconv.FormatInt(*o.CategoryID, 10)
	}
}

// overrideDurationUnit is the Duration unit stored with the override (Days when it changes none).
func overrideDurationUnit(values map[string]json.RawMessage) string {
	if raw, ok := values[constants.FieldDurationUnit]; ok {
		if u, isStr := decodeStored(raw).(string); isStr {
			return u
		}
	}
	return constants.ValueDays
}

// infoSheet lists the export metadata rows, always all present, in the fixed order.
func infoSheet(category string, in ExportInput, version int64) dto.Sheet {
	approval := "Not available"
	if in.Plan.ApprovalWorkflowAvailable {
		approval = "Available"
	}
	return dto.Sheet{
		Name:    "Info",
		Headers: []string{"Item", "Value"},
		Rows: [][]any{
			{"Export time", in.ExportedAt.Format(exportTimeLayout)},
			{"Exported by", exportCell(in.ActorName)},
			{"Category", category},
			{"Settings version", version},
			{"Approval Workflow", approval},
			{"Feature Suite Borrowing", levelLabel(in.Plan.FeatureSuite.Borrowing)},
			{"Feature Suite Assignment", levelLabel(in.Plan.FeatureSuite.Assignment)},
			{"Feature Suite Transfer", levelLabel(in.Plan.FeatureSuite.Transfer)},
			{"Feature Suite Disposal", levelLabel(in.Plan.FeatureSuite.Disposal)},
		},
	}
}

// exportCell formats a value: numbers stay numeric, switches Active/Inactive, lists joined by ", ", empty "-".
func exportCell(v any) any {
	switch val := v.(type) {
	case bool:
		if val {
			return "Active"
		}
		return "Inactive"
	case []string:
		if len(val) == 0 {
			return emptyCell
		}
		return strings.Join(val, ", ")
	case string:
		if val == "" {
			return emptyCell
		}
		return val
	case nil:
		return emptyCell
	default:
		return val
	}
}

// valueLabel is the display label of a choice token; any other text stays as written.
func valueLabel(token string) string {
	if label, ok := constants.ValueLabels[token]; ok {
		return label
	}
	return token
}

func exportUnit(unit string) string {
	if unit == "" {
		return emptyCell
	}
	return unit
}

// levelLabel renders a Feature Suite level as None, Basic or Full.
func levelLabel(level string) string {
	switch level {
	case constants.LevelNone:
		return "None"
	case constants.LevelBasic:
		return "Basic"
	default:
		return "Full"
	}
}

// dispatchExportActivityLog sends the export entry of LLD export Logs Written ("[Name] exported transaction rules
// for [Category]"), best-effort.
func (s *Service) dispatchExportActivityLog(ctx context.Context, in ExportInput, category string) {
	if s.activityEntries == nil {
		return
	}
	entry := ActivityLogEntry{
		ActorID: in.ActorID, ActorName: in.ActorName, LogTable: LogTableTransactionSettings,
		Action: actionExport, Object: objectTransactionRules, ObjectName: category,
	}
	if err := s.activityEntries.DispatchActivityEntries(context.WithoutCancel(ctx), in.ClientID, []ActivityLogEntry{entry}); err != nil {
		log.Printf("transaction settings export: activity log dispatch failed: %v", err)
	}
}
