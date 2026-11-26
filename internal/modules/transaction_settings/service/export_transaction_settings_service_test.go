package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

type fakeRenderer struct {
	sheets []dto.Sheet
	err    error
}

func (f *fakeRenderer) Render(sheets []dto.Sheet) ([]byte, error) {
	f.sheets = sheets
	return []byte("xlsx"), f.err
}

type fakeActivities struct {
	entries []ActivityLogEntry
	err     error
}

func (f *fakeActivities) DispatchActivityEntries(_ context.Context, _ string, entries []ActivityLogEntry) error {
	f.entries = append(f.entries, entries...)
	return f.err
}

func exportInput(categoryID *int64, plan Plan) ExportInput {
	return ExportInput{
		ClientID:   "client_1",
		CategoryID: categoryID,
		Plan:       plan,
		ActorID:    9,
		ActorName:  "Rina Wulandari",
		ExportedAt: time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC),
	}
}

func TestExportTransactionSettings_RendererMissing(t *testing.T) {
	file, err := NewService(&fakeRepo{}, nil).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.Nil(t, file)
	require.ErrorIs(t, err, ErrExportRendererUnavailable)
}

func TestExportTransactionSettings_AllCategories(t *testing.T) {
	renderer, activities := &fakeRenderer{}, &fakeActivities{}
	file, err := NewService(&fakeRepo{}, renderer).WithActivityEntryDispatcher(activities).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))

	require.NoError(t, err)
	require.Equal(t, ExportFileName, file.FileName)
	require.Equal(t, []byte("xlsx"), file.Data)
	require.Len(t, renderer.sheets, 3)
	rules, overrides, info := renderer.sheets[0], renderer.sheets[1], renderer.sheets[2]
	require.Equal(t, "Rules", rules.Name)
	require.Equal(t, []string{"Category", "Transaction", "Field", "Value", "Unit", "Source"}, rules.Headers)
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionBorrow, constants.FieldMaxDuration, 14, "days", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionBorrow, constants.FieldTransactionTimeout, 7, "days", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionDamagedMissing, constants.FieldAfterPhotoRequired, "Inactive", "-", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionBorrow, constants.FieldAllowBorrow, "Allow", "-", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionBorrow, constants.FieldApprovalType, "Any one approver", "-", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionAssign, constants.FieldNeedRecipientConfirmation, "Yes", "-", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionAudit, constants.FieldGracePeriod, 3, "days", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionMaintenance, constants.FieldRequireLocation, "Inactive", "-", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, constants.TransactionDisposal, constants.FieldAllowedDisposalTypes, "Sell, Scrap, Donate, Trade-in, Write-off", "-", constants.CompanyDefaultSource})
	require.Equal(t, "Overrides", overrides.Name)
	require.Empty(t, overrides.Rows)
	require.Len(t, overrides.Headers, 10)
	require.Equal(t, "Info", info.Name)
	require.Equal(t, [][]any{
		{"Export time", "2026-11-20 09:30:00"},
		{"Exported by", "Rina Wulandari"},
		{"Category", constants.NodeAllCategories},
		{"Settings version", int64(1)},
		{"Approval Workflow", "Available"},
		{"Feature Suite Borrowing", "Full"},
		{"Feature Suite Assignment", "Full"},
		{"Feature Suite Transfer", "Full"},
		{"Feature Suite Disposal", "Full"},
	}, info.Rows)
	require.Equal(t, []ActivityLogEntry{{ActorID: 9, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionExport, Object: objectTransactionRules, ObjectName: constants.NodeAllCategories}}, activities.entries)
}

func TestExportTransactionSettings_LeafWithPlanLevels(t *testing.T) {
	renderer, activities := &fakeRenderer{}, &fakeActivities{err: errors.New("down")}
	id := int64(42)
	plan := Plan{FeatureSuite: dto.FeatureSuite{Borrowing: constants.LevelNone, Assignment: constants.LevelBasic, Transfer: constants.LevelFull, Disposal: constants.LevelFull}}
	in := exportInput(&id, plan)
	in.ActorName = ""
	_, err := NewService(&fakeRepo{}, renderer).WithActivityEntryDispatcher(activities).ExportTransactionSettings(context.Background(), in)

	require.NoError(t, err)
	for _, row := range renderer.sheets[0].Rows {
		require.Equal(t, "42", row[0])
		require.NotEqual(t, constants.TransactionAudit, row[1])
		require.NotEqual(t, constants.FieldRequireLocation, row[2])
		require.NotEqual(t, constants.TransactionBorrow, row[1])
		require.NotEqual(t, constants.TransactionDamagedMissing, row[1])
		require.NotEqual(t, constants.FieldTransactionTimeout, row[2])
	}
	info := renderer.sheets[2].Rows
	require.Equal(t, []any{"Exported by", "-"}, info[1])
	require.Equal(t, []any{"Approval Workflow", "Not available"}, info[4])
	require.Equal(t, []any{"Feature Suite Borrowing", "None"}, info[5])
	require.Equal(t, []any{"Feature Suite Assignment", "Basic"}, info[6])
	require.Len(t, activities.entries, 1)
}

func TestExportTransactionSettings_InactiveAddonRowsDropped(t *testing.T) {
	renderer := &fakeRenderer{}
	plan := FullPlan
	plan.Addons = dto.Addons{}
	_, err := NewService(&fakeRepo{}, renderer).ExportTransactionSettings(context.Background(), exportInput(nil, plan))
	require.NoError(t, err)
	for _, row := range renderer.sheets[0].Rows {
		require.NotEqual(t, constants.TransactionAudit, row[1])
		require.NotEqual(t, constants.TransactionMaintenance, row[1])
	}
}

func TestExportTransactionSettings_RenderError(t *testing.T) {
	activities := &fakeActivities{}
	_, err := NewService(&fakeRepo{}, &fakeRenderer{err: errors.New("boom")}).WithActivityEntryDispatcher(activities).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.ErrorContains(t, err, "boom")
	require.Empty(t, activities.entries)
}

func TestExportTransactionSettings_NoDispatcher(t *testing.T) {
	file, err := NewService(&fakeRepo{}, &fakeRenderer{}).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.NoError(t, err)
	require.NotNil(t, file)
}

func TestExportCell(t *testing.T) {
	require.Equal(t, "Active", exportCell(true))
	require.Equal(t, "Inactive", exportCell(false))
	require.Equal(t, "-", exportCell([]string{}))
	require.Equal(t, "a, b", exportCell([]string{"a", "b"}))
	require.Equal(t, "-", exportCell(""))
	require.Equal(t, "Allow", exportCell("Allow"))
	require.Equal(t, "-", exportCell(nil))
	require.Equal(t, 0, exportCell(0))
}

func TestExportTransactionSettings_StoredRowsAndOverrides(t *testing.T) {
	catID := int64(42)
	path := "Electronics / Laptops"
	created := time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		version: 9,
		export: []model.TransactionRuleField{
			ownRow("borrow", constants.FieldMaxDuration, "30"),
			{CategoryID: &catID, TransactionType: "borrow", FieldKey: constants.FieldMaxDuration, Mode: constants.ModeOwn, Value: raw("21")},
		},
		overrides: []model.TransactionRuleOverride{
			{Scope: "group", TargetName: "Finance", CategoryID: &catID, CategoryPath: &path, TransactionType: "borrow", Fields: []byte(`{"max_duration":5,"auto_approve":false}`), CreatedByName: "Rina", CreatedAt: created},
			{Scope: "asset", TargetName: "Laptop 1", TransactionType: "transfer", Fields: []byte(`{"in_transit_overdue_after":3}`), CreatedByName: "Budi", CreatedAt: created},
			{Scope: "asset", TargetName: "Hidden", TransactionType: "disposal", Fields: []byte(`{"allow_disposal":false}`), CreatedByName: "Budi", CreatedAt: created},
		},
	}
	plan := FullPlan
	plan.FeatureSuite.Disposal = constants.LevelNone
	renderer := &fakeRenderer{}
	dir := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true, Path: path}}, leaves: []LeafCategory{{ID: 42, Path: path}}}
	_, err := NewService(repo, renderer).WithCategoryDirectory(dir).ExportTransactionSettings(context.Background(), exportInput(nil, plan))
	require.NoError(t, err)

	rules, overrides, info := renderer.sheets[0], renderer.sheets[1], renderer.sheets[2]
	require.Contains(t, rules.Rows, []any{constants.NodeAllCategories, "Borrow", "Max duration", 30, "days", constants.CompanyDefaultSource})
	require.Contains(t, rules.Rows, []any{path, "Borrow", "Max duration", 21, "days", path})
	require.Contains(t, rules.Rows, []any{path, "Borrow", "Max extensions", 1, "-", constants.CompanyDefaultSource})
	require.Equal(t, [][]any{
		{"Group", "Finance", path, "Borrow", "Max duration", 5, "days", "Applied", "Rina", "2026-11-01 08:00:00"},
		{"Group", "Finance", path, "Borrow", "Auto-approve", "Inactive", "-", "Applied", "Rina", "2026-11-01 08:00:00"},
		{"Asset", "Laptop 1", constants.NodeAllCategories, "Transfer", "In-transit overdue after", 3, "days", "Applied", "Budi", "2026-11-01 08:00:00"},
	}, overrides.Rows)
	require.Equal(t, []any{"Settings version", int64(9)}, info.Rows[3])
}

func TestExportTransactionSettings_LeafRulesListOnlyTheLeaf(t *testing.T) {
	catID := int64(42)
	repo := &fakeRepo{export: []model.TransactionRuleField{
		ownRow("borrow", constants.FieldMaxDuration, "30"),
		{CategoryID: &catID, TransactionType: "borrow", FieldKey: constants.FieldMaxExtensions, Mode: constants.ModeOwn, Value: raw("0")},
	}}
	renderer := &fakeRenderer{}
	_, err := NewService(repo, renderer).ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
	require.NoError(t, err)
	require.Contains(t, renderer.sheets[0].Rows, []any{"42", "Borrow", "Max extensions", 0, "-", "42"})
	require.Contains(t, renderer.sheets[0].Rows, []any{"42", "Borrow", "Max duration", 30, "days", constants.CompanyDefaultSource})
}

func TestExportTransactionSettings_AllCategoriesListEveryLeaf(t *testing.T) {
	pool, truck, parent := int64(42), int64(43), int64(44)
	repo := &fakeRepo{export: []model.TransactionRuleField{
		ownRow("borrow", constants.FieldMaxDuration, "30"),
		{CategoryID: &pool, TransactionType: "borrow", FieldKey: constants.FieldMaxDuration, Mode: constants.ModeOwn, Value: raw("21")},
		{CategoryID: &parent, TransactionType: "borrow", FieldKey: constants.FieldMaxDuration, Mode: constants.ModeOwn, Value: raw("7")},
	}}
	dir := &fakeCategories{leaves: []LeafCategory{{ID: pool, Path: "Vehicle / Pool car"}, {ID: truck, Path: "Vehicle / Truck"}}}
	renderer := &fakeRenderer{}
	_, err := NewService(repo, renderer).WithCategoryDirectory(dir).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.NoError(t, err)
	rules := renderer.sheets[0].Rows
	require.Contains(t, rules, []any{"Vehicle / Pool car", "Borrow", "Max duration", 21, "days", "Vehicle / Pool car"})
	require.Contains(t, rules, []any{"Vehicle / Truck", "Borrow", "Max duration", 30, "days", constants.CompanyDefaultSource})
	for _, row := range rules {
		require.NotEqual(t, "44", row[0], "a parent Category's stored values leave Rules")
	}
}

func TestExportTransactionSettings_RepositoryErrors(t *testing.T) {
	activities := &fakeActivities{}
	_, err := NewService(&fakeRepo{err: errors.New("db down")}, &fakeRenderer{}).WithActivityEntryDispatcher(activities).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.ErrorContains(t, err, "db down")
	require.Empty(t, activities.entries)
}

func TestExportTransactionSettings_CategoryLookup(t *testing.T) {
	catID := int64(42)
	t.Run("a leaf is labelled by its full path", func(t *testing.T) {
		renderer, activities := &fakeRenderer{}, &fakeActivities{}
		dir := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true, Path: "Vehicle > Pool car"}}}
		svc := NewService(&fakeRepo{}, renderer).WithCategoryDirectory(dir).WithActivityEntryDispatcher(activities)
		_, err := svc.ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
		require.NoError(t, err)
		require.Contains(t, renderer.sheets[2].Rows, []any{"Category", "Vehicle > Pool car"})
		require.Equal(t, "Vehicle > Pool car", activities.entries[0].ObjectName)
	})
	t.Run("an unknown, deleted, other-entity or parent Category is not found", func(t *testing.T) {
		for name, info := range map[string]CategoryInfo{"unknown, deleted or other entity": {}, "parent": {Exists: true}} {
			renderer, activities := &fakeRenderer{}, &fakeActivities{}
			dir := &fakeCategories{infos: map[int64]CategoryInfo{42: info}}
			svc := NewService(&fakeRepo{}, renderer).WithCategoryDirectory(dir).WithActivityEntryDispatcher(activities)
			_, err := svc.ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
			require.ErrorIs(t, err, ErrCategoryNotFound, name)
			require.Empty(t, renderer.sheets, name)
			require.Empty(t, activities.entries, name)
		}
	})
	t.Run("a directory failure is an error", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, &fakeRenderer{}).WithCategoryDirectory(&fakeCategories{err: errors.New("mar down")})
		_, err := svc.ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
		require.ErrorContains(t, err, "mar down")
	})
	t.Run("an unset directory accepts the id and labels it by id", func(t *testing.T) {
		renderer := &fakeRenderer{}
		_, err := NewService(&fakeRepo{}, renderer).ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
		require.NoError(t, err)
		require.Contains(t, renderer.sheets[2].Rows, []any{"Category", "42"})
	})
}

// exportFieldsFailRepo fails the export read of the stored rules.
type exportFieldsFailRepo struct{ fakeRepo }

func (f *exportFieldsFailRepo) FindExportFields(context.Context, string, *int64) ([]model.TransactionRuleField, error) {
	return nil, errBoom
}

// ruleUnit is the Unit cell of the Rules row of one transaction and field.
func ruleUnit(t *testing.T, rows [][]any, tx, field string) any {
	t.Helper()
	for _, r := range rows {
		if r[1] == tx && r[2] == field {
			return r[4]
		}
	}
	t.Fatalf("no Rules row for %s %s", tx, field)
	return nil
}

func TestExportTransactionSettings_ReadFailures(t *testing.T) {
	_, err := NewService(&exportFieldsFailRepo{}, &fakeRenderer{}).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.ErrorContains(t, err, "read export fields")
	_, err = NewService(&failingRepo{failOver: true}, &fakeRenderer{}).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.ErrorContains(t, err, "read export overrides")
}

func TestExportTransactionSettings_DurationUnits(t *testing.T) {
	catID := int64(42)
	company := storedRow(nil, constants.TransactionBorrow, constants.FieldDurationUnit, `"hours"`)

	// All categories: the stored Company default unit is the unit of its rows.
	renderer := &fakeRenderer{}
	_, err := NewService(&fakeRepo{export: []model.TransactionRuleField{company}}, renderer).ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.NoError(t, err)
	require.Equal(t, "hours", ruleUnit(t, renderer.sheets[0].Rows, constants.TransactionBorrow, constants.FieldMaxDuration))

	// A leaf without its own Duration unit takes the Company default's.
	renderer = &fakeRenderer{}
	_, err = NewService(&fakeRepo{export: []model.TransactionRuleField{company}}, renderer).ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
	require.NoError(t, err)
	require.Equal(t, "hours", ruleUnit(t, renderer.sheets[0].Rows, constants.TransactionBorrow, constants.FieldMaxDuration))

	// A leaf's own stored Duration unit is the unit of its rows, over the Company default (days).
	renderer = &fakeRenderer{}
	own := storedRow(&catID, constants.TransactionBorrow, constants.FieldDurationUnit, `"hours"`)
	_, err = NewService(&fakeRepo{export: []model.TransactionRuleField{own}}, renderer).ExportTransactionSettings(context.Background(), exportInput(&catID, FullPlan))
	require.NoError(t, err)
	require.Equal(t, "hours", ruleUnit(t, renderer.sheets[0].Rows, constants.TransactionBorrow, constants.FieldMaxDuration))

	// An override carries the Duration unit it stores. Override fields are listed in key order:
	// duration_unit (no unit of its own), then max_duration in hours.
	renderer = &fakeRenderer{}
	_, err = NewService(&fakeRepo{overrides: []model.TransactionRuleOverride{override("group", 1, nil, constants.TransactionBorrow, `{"duration_unit":"hours","max_duration":3}`)}}, renderer).
		ExportTransactionSettings(context.Background(), exportInput(nil, FullPlan))
	require.NoError(t, err)
	require.Equal(t, "hours", renderer.sheets[1].Rows[1][6])
}

func TestValueLabel(t *testing.T) {
	require.Equal(t, "Any one approver", valueLabel("any_one_approver"))
	require.Equal(t, "custom text", valueLabel("custom text"))
}

func TestOverrideHelpers(t *testing.T) {
	require.Equal(t, "Asset Name", scopeLabel("asset_name"))
	require.Equal(t, "other", scopeLabel("other"))
	id := int64(3)
	require.Equal(t, "3", overrideCategory(model.TransactionRuleOverride{CategoryID: &id}))
	require.Equal(t, "", fieldDef("borrow", "unknown").Unit)
	require.Equal(t, "hours", resolveUnit(constants.DurationUnit, "hours"))
	require.Equal(t, "Deny", fieldCell(fieldDef("borrow", constants.FieldAllowBorrow), false))
	sheet := overridesSheet(FullPlan, []model.TransactionRuleOverride{{TransactionType: "borrow", Fields: []byte(`not-json`)}}, exportLookups{})
	require.Empty(t, sheet.Rows)
}
