package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

type fakeChangeLogs struct {
	calls    int
	entries  []ChangeLogEntry
	activity []ActivityLogEntry
	err      error
}

func (f *fakeChangeLogs) DispatchLogs(_ context.Context, _ string, e []ChangeLogEntry, a []ActivityLogEntry) error {
	f.calls++
	f.entries = append(f.entries, e...)
	f.activity = append(f.activity, a...)
	return f.err
}

type fakeCategories struct {
	infos  map[int64]CategoryInfo
	leaves []LeafCategory
	err    error
}

func (f *fakeCategories) LookupCategory(_ context.Context, _ string, id int64) (CategoryInfo, error) {
	return f.infos[id], f.err
}

func (f *fakeCategories) ListLeafCategories(_ context.Context, _ string) ([]LeafCategory, error) {
	return f.leaves, f.err
}

func sf(key, mode, value string) dto.SaveField {
	f := dto.SaveField{Key: key, Mode: mode}
	if value != "" {
		f.Value = json.RawMessage(value)
	}
	return f
}

func change(cat string, tx string, fields ...dto.SaveField) dto.SaveChange {
	c := dto.SaveChange{Transaction: tx, Fields: fields}
	if cat != "" {
		c.CategoryID = &cat
	}
	return c
}

func saveInput(plan Plan, changes ...dto.SaveChange) SaveInput {
	return SaveInput{ClientID: "c1", Request: dto.SaveSettingsRequest{Changes: changes}, Plan: plan, ActorID: 7, ActorName: "Rina Wulandari"}
}

func storedRow(cat *int64, tx, key, value string) model.TransactionRuleField {
	raw := json.RawMessage(value)
	return model.TransactionRuleField{CategoryID: cat, TransactionType: tx, FieldKey: key, Mode: constants.ModeOwn, Value: &raw}
}

func validationErrors(t *testing.T, err error) []dto.FieldError {
	t.Helper()
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Equal(t, "validation failed", ve.Error())
	return ve.Errors
}

func TestSave_CompanyDefaultChangeWritesBumpsAndLogs(t *testing.T) {
	repo := &fakeRepo{}
	logs := &fakeChangeLogs{}
	svc := NewService(repo, nil).WithChangeLogDispatcher(logs)
	res, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("", "borrow", sf("max_duration", "own", "25"), sf("allowed_borrower", "own", `"same_group_only"`))))
	require.NoError(t, err)
	require.True(t, res.Changed)
	require.Len(t, repo.tx.rows, 2)
	require.Equal(t, 1, repo.tx.bumps)
	require.Equal(t, "Rina Wulandari", repo.tx.bumpedBy)
	require.True(t, repo.tx.committed)
	require.True(t, repo.tx.closed)
	require.Equal(t, "25", string(*repo.tx.rows[0].Value))
	require.Equal(t, int64(7), *repo.tx.rows[0].ModifiedByID)
	require.Equal(t, 1, logs.calls, "one save dispatches its Changelog and Activity Log rows in one call")
	require.Equal(t, []ActivityLogEntry{
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionEdit, Object: "Borrow", ObjectName: "All categories", Field: "Max duration", OldValue: "14 days", NewValue: "25 days"},
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionEdit, Object: "Borrow", ObjectName: "All categories", Field: "Allowed borrower", OldValue: "Employee in any Group", NewValue: "Same Group only"},
	}, logs.activity)
	require.Equal(t, ChangeLogEntry{
		Action: "Edit", Entity: "All categories", ObjectPath: "Transaction Settings", Field: "Borrow (Max duration)",
		OldValue: "14 days", NewValue: "25 days", ModifiedBy: "Rina Wulandari", CreatedByID: 7, LogTable: LogTableTransactionSettings,
	}, logs.entries[0])
	require.Equal(t, "Employee in any Group", logs.entries[1].OldValue)
	require.Equal(t, "Same Group only", logs.entries[1].NewValue)
}

func TestSave_NoDifferenceIsNoOp(t *testing.T) {
	repo := &fakeRepo{company: []model.TransactionRuleField{storedRow(nil, "borrow", "max_duration", "21")}}
	res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("", "borrow", sf("max_duration", "own", "21"), sf("max_extensions", "own", "1")),
		change("", "disposal", sf("allowed_disposal_types", "own", `["write_off","sell","scrap","donate","trade_in"]`))))
	require.NoError(t, err)
	require.False(t, res.Changed)
	require.Zero(t, repo.began)
}

func TestSave_CategoryInheritToOwnAndOwnToInherit(t *testing.T) {
	cat := int64(42)
	repo := &fakeRepo{leaf: []model.TransactionRuleField{storedRow(&cat, "borrow", "max_extensions", "2")}}
	logs := &fakeChangeLogs{}
	dir := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Path: "Vehicle > Pool car"}}}
	svc := NewService(repo, nil).WithChangeLogDispatcher(logs).WithCategoryDirectory(dir)
	res, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("42", "borrow", sf("max_duration", "own", "7"), sf("max_extensions", "inherit", ""))))
	require.NoError(t, err)
	require.True(t, res.Changed)
	require.Equal(t, "Vehicle > Pool car", logs.entries[0].Entity)
	require.Equal(t, "Inherit", logs.entries[0].OldValue)
	require.Equal(t, "7 days", logs.entries[0].NewValue)
	require.Equal(t, "Edit", logs.entries[1].Action)
	require.Equal(t, "2", logs.entries[1].OldValue)
	require.Equal(t, "Inherit", logs.entries[1].NewValue)
	require.Nil(t, repo.tx.rows[1].Value)
	require.Equal(t, constants.ModeInherit, repo.tx.rows[1].Mode)
}

func TestSave_CategoryNoEntityPathFallsBackToID(t *testing.T) {
	logs := &fakeChangeLogs{}
	svc := NewService(&fakeRepo{}, nil).WithChangeLogDispatcher(logs)
	_, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, change("42", "borrow", sf("max_duration", "own", "7"))))
	require.NoError(t, err)
	require.Equal(t, "42", logs.entries[0].Entity)
}

func TestSave_DurationUnitInLog(t *testing.T) {
	logs := &fakeChangeLogs{}
	svc := NewService(&fakeRepo{}, nil).WithChangeLogDispatcher(logs)
	_, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("", "borrow", sf("duration_unit", "own", `"hours"`), sf("max_duration", "own", "48"))))
	require.NoError(t, err)
	require.Equal(t, "Days", logs.entries[0].OldValue)
	require.Equal(t, "48 hours", logs.entries[1].NewValue)
}

func TestSave_ValidationMessages(t *testing.T) {
	cases := []struct {
		name  string
		field dto.SaveField
		tx    string
		want  string
	}{
		{"missing", sf("max_duration", "own", ""), "borrow", msgValueRequired},
		{"null", sf("max_duration", "own", "null"), "borrow", msgValueRequired},
		{"empty string", sf("max_duration", "own", `""`), "borrow", msgValueRequired},
		{"zero min one", sf("max_duration", "own", "0"), "borrow", msgMinOne},
		{"negative min zero", sf("max_extensions", "own", "-1"), "borrow", msgMinZero},
		{"fraction", sf("max_duration", "own", "1.5"), "borrow", msgWholeNumber},
		{"string for number", sf("max_duration", "own", `"abc"`), "borrow", msgWholeNumber},
		{"bad json", sf("max_duration", "own", `{`), "borrow", msgInvalidOption},
		{"bool wrong type", sf("self_service", "own", "1"), "borrow", msgBoolean},
		{"unknown token", sf("allowed_borrower", "own", `"everyone"`), "borrow", msgInvalidOption},
		{"choice wrong type", sf("allowed_borrower", "own", "3"), "borrow", msgInvalidOption},
		{"empty list", sf("allowed_disposal_types", "own", "[]"), "disposal", msgNoDisposalType},
		{"duplicate list", sf("allowed_disposal_types", "own", `["sell","sell"]`), "disposal", msgInvalidOption},
		{"unknown list token", sf("allowed_disposal_types", "own", `["lost"]`), "disposal", msgInvalidOption},
		{"list wrong type", sf("allowed_disposal_types", "own", `"sell"`), "disposal", msgInvalidOption},
		{"list item wrong type", sf("allowed_disposal_types", "own", `[1]`), "disposal", msgInvalidOption},
		{"unknown key", sf("nope", "own", "1"), "borrow", msgUnknownField},
		{"inherit at company", sf("max_duration", "inherit", ""), "borrow", msgInheritNotAllowed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan, change("", tc.tx, tc.field)))
			errs := validationErrors(t, err)
			require.Len(t, errs, 1)
			require.Equal(t, tc.want, errs[0].Message)
			require.Zero(t, repo.began)
		})
	}
}

func TestSave_FieldIndexRunsAcrossChanges(t *testing.T) {
	_, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("", "borrow", sf("max_duration", "own", "5")),
		change("", "disposal", sf("min_asset_age_warning", "own", "0"))))
	require.Equal(t, "fields[1].value", validationErrors(t, err)[0].Field)
}

func TestSave_CategoryRules(t *testing.T) {
	cases := []struct {
		name   string
		change dto.SaveChange
		field  string
		want   string
	}{
		{"all categories only field", change("5", "borrow", sf("transaction_timeout", "own", "3")), "fields[0].value", msgOwnNotAllowed},
		{"all categories only inherit", change("5", "borrow", sf("send_email_notifications", "inherit", "")), "fields[0].value", msgOwnNotAllowed},
		{"audit at category", change("5", "audit", sf("grace_period", "own", "3")), "changes[0].transaction", msgAllCategoriesOnlyTx},
		{"damaged at category", change("5", "damaged_missing", sf("long_missing_after", "own", "3")), "changes[0].transaction", msgAllCategoriesOnlyTx},
		{"require location at category", change("5", "maintenance", sf("require_location", "own", "true")), "fields[0].value", msgOwnNotAllowed},
		{"malformed id", change("abc", "borrow", sf("max_duration", "own", "3")), "changes[0].categoryId", msgInvalidCategoryID},
		{"zero id", change("0", "borrow", sf("max_duration", "own", "3")), "changes[0].categoryId", msgInvalidCategoryID},
		{"padded id", change("007", "borrow", sf("max_duration", "own", "3")), "changes[0].categoryId", msgInvalidCategoryID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan, tc.change))
			errs := validationErrors(t, err)
			require.Equal(t, tc.field, errs[0].Field)
			require.Equal(t, tc.want, errs[0].Message)
		})
	}
}

func TestSave_MaintenanceInheritAtCategoryAccepted(t *testing.T) {
	cat := int64(5)
	repo := &fakeRepo{leaf: []model.TransactionRuleField{storedRow(&cat, "maintenance", "reminder_before_due", "5")}}
	res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("5", "maintenance", sf("reminder_before_due", "inherit", ""))))
	require.NoError(t, err)
	require.True(t, res.Changed)
}

func TestSave_CategoryLookup(t *testing.T) {
	c := change("5", "borrow", sf("max_duration", "own", "3"))
	// An other-entity, unknown or deleted Category answers 404 after the validation list; nothing is written.
	for name, info := range map[string]CategoryInfo{"unknown, deleted or other entity": {}} {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := NewService(repo, nil).WithCategoryDirectory(&fakeCategories{infos: map[int64]CategoryInfo{5: info}})
			_, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, c))
			require.ErrorIs(t, err, ErrCategoryNotFound)
			require.Zero(t, repo.began)
		})
	}
	t.Run("validation list is answered before not found", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, nil).WithCategoryDirectory(&fakeCategories{})
		_, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, change("5", "borrow", sf("max_duration", "own", "0"))))
		errs := validationErrors(t, err)
		require.Equal(t, msgMinOne, errs[0].Message)
	})
	t.Run("parent category is accepted", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := NewService(repo, nil).WithCategoryDirectory(&fakeCategories{infos: map[int64]CategoryInfo{5: {Exists: true}}})
		// A second change of the same Category is looked up once.
		res, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, c, change("5", "borrow", sf("max_extensions", "own", "3"))))
		require.NoError(t, err)
		require.True(t, res.Changed)
		require.Equal(t, 1, repo.began)
	})
	t.Run("provider error", func(t *testing.T) {
		svc := NewService(&fakeRepo{}, nil).WithCategoryDirectory(&fakeCategories{err: errors.New("down")})
		_, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, c))
		require.Error(t, err)
	})
}

func TestSave_PlanGating(t *testing.T) {
	basicAssign := FullPlan
	basicAssign.FeatureSuite.Assignment = constants.LevelBasic
	noWorkflow := FullPlan
	noWorkflow.ApprovalWorkflowAvailable = false
	noAudit := FullPlan
	noAudit.Addons.Audit = false
	noBorrow := FullPlan
	noBorrow.FeatureSuite.Borrowing = constants.LevelNone

	cases := []struct {
		name string
		plan Plan
		c    dto.SaveChange
	}{
		{"assignee at basic", basicAssign, change("", "assign", sf("allowed_assignee_division", "own", "false"))},
		{"approval without workflow", noWorkflow, change("", "borrow", sf("require_approval", "own", "false"))},
		{"inactive addon", noAudit, change("", "audit", sf("grace_period", "own", "9"))},
		{"suite none", noBorrow, change("", "borrow", sf("max_duration", "own", "9"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{}
			_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(tc.plan, tc.c))
			require.Equal(t, msgFieldGated, validationErrors(t, err)[0].Message)
			require.Zero(t, repo.began)
		})
	}
	t.Run("unchanged gated value is not a difference", func(t *testing.T) {
		res, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(noWorkflow,
			change("", "borrow", sf("require_approval", "own", "true"))))
		require.NoError(t, err)
		require.False(t, res.Changed)
	})
}

func TestSave_AssigneeGuard(t *testing.T) {
	both := change("", "assign", sf("allowed_assignee_employee", "own", "false"), sf("allowed_assignee_division", "own", "false"))
	t.Run("both off rejected", func(t *testing.T) {
		_, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan, both))
		errs := validationErrors(t, err)
		require.Len(t, errs, 2)
		require.Equal(t, "fields[0].value", errs[0].Field)
		require.Equal(t, "fields[1].value", errs[1].Field)
		require.Equal(t, msgNoAssigneeType, errs[0].Message)
	})
	t.Run("one field sent uses stored other", func(t *testing.T) {
		repo := &fakeRepo{company: []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_division", "false")}}
		_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "assign", sf("allowed_assignee_employee", "own", "false"), sf("max_assets_per_assignee", "own", "2"))))
		errs := validationErrors(t, err)
		require.Equal(t, "fields[0].value", errs[0].Field)
		require.Len(t, errs, 1)
	})
	t.Run("deny skips guard", func(t *testing.T) {
		res, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "assign", sf("allow_assign", "own", "false"), sf("allowed_assignee_employee", "own", "false"), sf("allowed_assignee_division", "own", "false"))))
		require.NoError(t, err)
		require.True(t, res.Changed)
	})
	t.Run("basic skips guard", func(t *testing.T) {
		plan := FullPlan
		plan.FeatureSuite.Assignment = constants.LevelBasic
		_, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(plan,
			change("", "assign", sf("max_assets_per_assignee", "own", "2"))))
		require.NoError(t, err)
	})
	t.Run("leaf reads company effective", func(t *testing.T) {
		repo := &fakeRepo{company: []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_division", "false")}}
		_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("9", "assign", sf("allowed_assignee_employee", "own", "false"))))
		require.Len(t, validationErrors(t, err), 1)
	})
}

func TestSave_OverrideAssigneeGuard(t *testing.T) {
	t.Run("override row becoming both OFF answers one entry", func(t *testing.T) {
		repo := &fakeRepo{
			company:   []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_employee", "true"), storedRow(nil, "assign", "allowed_assignee_division", "true")},
			overrides: []model.TransactionRuleOverride{{TransactionType: "assign", Scope: "group", TargetID: 7, Fields: json.RawMessage(`{"allowed_assignee_employee": false}`)}},
		}
		_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "assign", sf("allowed_assignee_division", "own", "false"))))
		errs := validationErrors(t, err)
		require.Len(t, errs, 1)
		require.Equal(t, "fields[0].value", errs[0].Field)
		require.Equal(t, msgNoAssigneeType, errs[0].Message)
	})
	t.Run("row already both OFF before the save is not answered", func(t *testing.T) {
		repo := &fakeRepo{
			company:   []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_employee", "true"), storedRow(nil, "assign", "allowed_assignee_division", "false")},
			overrides: []model.TransactionRuleOverride{{TransactionType: "assign", Scope: "group", TargetID: 7, Fields: json.RawMessage(`{"allowed_assignee_employee": false}`)}},
		}
		res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "assign", sf("max_assets_per_assignee", "own", "2"))))
		require.NoError(t, err)
		require.True(t, res.Changed)
	})
}

func TestSave_HiddenFieldsKeepStoredValue(t *testing.T) {
	t.Run("deny drops invalid zero", func(t *testing.T) {
		repo := &fakeRepo{}
		res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "borrow", sf("allow_borrow", "own", "false"), sf("max_duration", "own", "0"))))
		require.NoError(t, err)
		require.True(t, res.Changed)
		require.Len(t, repo.tx.rows, 1)
		require.Equal(t, "allow_borrow", repo.tx.rows[0].FieldKey)
	})
	t.Run("approval off drops auto approve duration", func(t *testing.T) {
		repo := &fakeRepo{}
		res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "borrow", sf("require_approval", "own", "false"), sf("auto_approve_max_duration", "own", "0"))))
		require.NoError(t, err)
		require.True(t, res.Changed)
		require.Len(t, repo.tx.rows, 1)
	})
	t.Run("visible zero still rejected", func(t *testing.T) {
		_, err := NewService(&fakeRepo{}, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
			change("", "borrow", sf("auto_approve_max_duration", "own", "0"))))
		require.Equal(t, msgMinOne, validationErrors(t, err)[0].Message)
	})
}

func TestSave_LastDuplicateWins(t *testing.T) {
	repo := &fakeRepo{}
	_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("", "borrow", sf("max_duration", "own", "5"), sf("max_duration", "own", "9"))))
	require.NoError(t, err)
	require.Len(t, repo.tx.rows, 1)
	require.Equal(t, "9", string(*repo.tx.rows[0].Value))
}

func TestSave_StorageFailures(t *testing.T) {
	in := saveInput(FullPlan, change("", "borrow", sf("max_duration", "own", "5")))
	boom := errors.New("boom")
	cases := []struct {
		name string
		repo *fakeRepo
		want string
	}{
		{"read", &fakeRepo{err: boom}, "read stored fields"},
		{"begin", &fakeRepo{beginErr: boom}, "boom"},
		{"upsert", &fakeRepo{tx: &fakeTx{upsertErr: boom}}, "boom"},
		{"bump", &fakeRepo{tx: &fakeTx{bumpErr: boom}}, "boom"},
		{"commit", &fakeRepo{tx: &fakeTx{commitErr: boom}}, "commit settings save"},
		{"rollback", &fakeRepo{tx: &fakeTx{upsertErr: boom, rollbackErr: errors.New("rb")}}, "rollback settings save"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewService(tc.repo, nil).SaveTransactionSettings(context.Background(), in)
			require.ErrorContains(t, err, tc.want)
			if tc.repo.tx != nil && tc.repo.tx.upsertErr != nil {
				require.True(t, tc.repo.tx.rolledBack)
			}
		})
	}
}

func TestSave_LogDispatchFailuresDoNotFailSave(t *testing.T) {
	svc := NewService(&fakeRepo{}, nil).WithChangeLogDispatcher(&fakeChangeLogs{err: errors.New("y")})
	res, err := svc.SaveTransactionSettings(context.Background(), saveInput(FullPlan, change("", "borrow", sf("max_duration", "own", "5"))))
	require.NoError(t, err)
	require.True(t, res.Changed)
}

func TestLogValue(t *testing.T) {
	def := func(tx, key string) constants.FieldDef { d, _ := findDef(tx, key); return d }
	cases := []struct {
		name  string
		def   constants.FieldDef
		mode  string
		value any
		unit  string
		want  string
	}{
		{"inherit", def("borrow", "max_duration"), constants.ModeInherit, nil, "", "Inherit"},
		{"on", def("borrow", "photo_at_start"), constants.ModeOwn, true, "", "ON"},
		{"off", def("borrow", "photo_at_start"), constants.ModeOwn, false, "", "OFF"},
		{"allow", def("borrow", "allow_borrow"), constants.ModeOwn, true, "", "Allow"},
		{"deny", def("borrow", "allow_borrow"), constants.ModeOwn, false, "", "Deny"},
		{"yes", def("assign", "need_recipient_confirmation"), constants.ModeOwn, true, "", "Yes"},
		{"choice label", def("borrow", "approval_type"), constants.ModeOwn, "all_approvers", "", "All approvers (AND)"},
		{"choice unknown token", def("borrow", "approval_type"), constants.ModeOwn, "zzz", "", "zzz"},
		{"list option order", def("disposal", "allowed_disposal_types"), constants.ModeOwn, []string{"write_off", "sell", "trade_in"}, "", "Sell, Trade-in, Write-off"},
		{"duration", def("borrow", "max_duration"), constants.ModeOwn, 14, "days", "14 days"},
		{"days unit", def("borrow", "transaction_timeout"), constants.ModeOwn, 7, "", "7 days"},
		{"no unit", def("borrow", "max_extensions"), constants.ModeOwn, 3, "", "3"},
		{"percent", def("disposal", "max_deviation_from_book_value"), constants.ModeOwn, 30, "", "30%"},
		{"months", def("disposal", "min_asset_age_warning"), constants.ModeOwn, 12, "", "12 months"},
		{"zero extensions", def("borrow", "max_extensions"), constants.ModeOwn, 0, "", "No extension"},
		{"zero unlimited", def("assign", "max_assets_per_assignee"), constants.ModeOwn, 0, "", "Unlimited"},
		{"zero off", def("damaged_missing", "long_missing_after"), constants.ModeOwn, 0, "", "Off"},
		{"zero grace", def("audit", "grace_period"), constants.ModeOwn, 0, "", "No grace period"},
		{"zero reminder", def("maintenance", "reminder_before_due"), constants.ModeOwn, 0, "", "0 days"},
		{"fallback", def("borrow", "max_duration"), constants.ModeOwn, 1.5, "", "1.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { require.Equal(t, tc.want, logValue(tc.def, tc.mode, tc.value, tc.unit)) })
	}
}

func TestSameValueAndOptionOrder(t *testing.T) {
	require.True(t, sameValue([]string{"a", "b"}, []string{"b", "a"}))
	require.False(t, sameValue([]string{"a"}, []string{"b"}))
	require.False(t, sameValue([]string{"a"}, "a"))
	require.True(t, sameValue(3, 3))
	require.Equal(t, []string{"sell", "donate"}, optionOrder([]string{"donate", "sell"}))
}

func TestSave_StoredInheritRowIsInherit(t *testing.T) {
	cat := int64(5)
	inherit := model.TransactionRuleField{CategoryID: &cat, TransactionType: "borrow", FieldKey: "max_duration", Mode: constants.ModeInherit}
	repo := &fakeRepo{leaf: []model.TransactionRuleField{inherit}}
	res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("5", "borrow", sf("max_duration", "inherit", ""))))
	require.NoError(t, err)
	require.False(t, res.Changed)
}

func TestSave_PendingInheritFallsBackToCompanyInGuard(t *testing.T) {
	cat := int64(9)
	repo := &fakeRepo{
		company: []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_division", "false")},
		leaf:    []model.TransactionRuleField{storedRow(&cat, "assign", "allowed_assignee_employee", "false")},
	}
	res, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("9", "assign", sf("allowed_assignee_employee", "inherit", ""))))
	require.NoError(t, err)
	require.True(t, res.Changed)
}

func TestSave_GuardReadsStoredLeafValue(t *testing.T) {
	cat := int64(9)
	repo := &fakeRepo{leaf: []model.TransactionRuleField{storedRow(&cat, "assign", "allowed_assignee_employee", "false")}}
	_, err := NewService(repo, nil).SaveTransactionSettings(context.Background(), saveInput(FullPlan,
		change("9", "assign", sf("allowed_assignee_division", "own", "false"))))
	require.Len(t, validationErrors(t, err), 1)
}
