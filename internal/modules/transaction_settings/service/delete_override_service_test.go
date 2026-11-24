package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

func ov(id int64, scope, tx string, target int64, fields string) model.TransactionRuleOverride {
	path := "Vehicle"
	return model.TransactionRuleOverride{
		ID: id, Scope: scope, TransactionType: tx, TargetID: target, TargetName: "T" + itoa(int(id)), Fields: json.RawMessage(fields),
		CategoryPath: nil, TargetGroupID: int64p(3), CreatedByName: path,
	}
}

func delIn(access CallerAccess, ids ...string) DeleteOverridesInput {
	return DeleteOverridesInput{ClientID: "c1", Request: dto.DeleteOverridesRequest{IDs: ids}, Plan: FullPlan, ActorID: 7, ActorName: "Rina Wulandari", Access: access}
}

func TestDeleteOverrides_DeletesBumpsOnceAndLogs(t *testing.T) {
	repo := newOverrideRepo()
	repo.byID = []model.TransactionRuleOverride{ov(1, "group", "borrow", 10, `{"max_duration":3}`), ov(2, "asset_name", "borrow", 20, `{"max_duration":3}`)}
	repo.otx.deleteN = 2
	acts := &fakeActivities{}
	svc := NewService(repo, nil).WithActivityEntryDispatcher(acts)
	res, err := svc.DeleteOverrides(context.Background(), delIn(totalControl, "1", "2", "1", "9"))
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2, 9}, res.DeletedIDs) // 9 is already gone: reported deleted
	require.Empty(t, res.Skipped)
	require.Equal(t, []int64{1, 2}, repo.otx.deleted)
	require.Equal(t, 1, repo.otx.bumps)
	require.True(t, repo.otx.committed)
	require.Equal(t, []ActivityLogEntry{
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionDelete, Object: "Borrow", ObjectName: "T1 in All categories"},
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionDelete, Object: "Borrow", ObjectName: "T2 in All categories"},
	}, acts.entries)
}

func TestDeleteOverrides_NothingRemovedDoesNotBump(t *testing.T) {
	repo := newOverrideRepo()
	repo.byID = []model.TransactionRuleOverride{ov(1, "group", "borrow", 10, `{"max_duration":3}`)}
	repo.otx.deleteN = 0 // removed by someone else meanwhile
	res, err := NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.NoError(t, err)
	require.Equal(t, []int64{1}, res.DeletedIDs)
	require.Zero(t, repo.otx.bumps)
	require.True(t, repo.otx.committed)
}

func TestDeleteOverrides_Validation(t *testing.T) {
	svc := NewService(newOverrideRepo(), nil)
	_, err := svc.DeleteOverrides(context.Background(), delIn(totalControl))
	require.Equal(t, []dto.FieldError{{Field: "ids", Message: msgPickIDs}}, validationErrors(t, err))
	_, err = svc.DeleteOverrides(context.Background(), delIn(totalControl, "1", "x"))
	require.Equal(t, []dto.FieldError{{Field: "ids[1]", Message: msgInvalidID}}, validationErrors(t, err))
	_, err = svc.DeleteOverrides(context.Background(), delIn(CallerAccess{ReadOnly: true, TotalControl: true}, "1"))
	require.ErrorIs(t, err, ErrOverrideForbidden)
	_, err = NewService(&fakeRepo{}, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.ErrorIs(t, err, ErrOverrideStorageUnavailable)
}

func TestDeleteOverrides_NoneExistAnswers204Shape(t *testing.T) {
	repo := newOverrideRepo()
	res, err := NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "5", "6"))
	require.NoError(t, err)
	require.True(t, res.NoneExist)
	require.Empty(t, repo.otx.deleted)
	require.False(t, repo.otx.committed)
}

func TestDeleteOverrides_ScopeSkips(t *testing.T) {
	repo := newOverrideRepo()
	repo.byID = []model.TransactionRuleOverride{
		ov(1, "group", "borrow", 10, `{"max_duration":3}`),
		ov(2, "asset_name", "borrow", 20, `{"max_duration":3}`),
		ov(3, "asset", "borrow", 30, `{"max_duration":3}`),
		ov(4, "asset", "borrow", 31, `{"max_duration":3}`),
		ov(5, "group", "borrow", 11, `{"max_duration":3}`),
	}
	repo.byID[3].TargetGroupID = int64p(99)
	groups := &fakeGroups{expanded: []int64{3, 10}, current: map[int64]int64{30: 3}}
	svc := NewService(repo, nil).WithGroupScopeDirectory(groups)
	access := CallerAccess{WriteGroupIDs: []int64{3}}
	// Row 1 (Group 10 in subtree) and 3 (asset now in Group 3) go; 2 (Asset Name), 4 (snapshot Group 99) and 5 (Group 11) are out of scope.
	res, err := svc.DeleteOverrides(context.Background(), delIn(access, "1", "2", "3", "4", "5"))
	require.NoError(t, err)
	require.Equal(t, []int64{1, 3}, res.DeletedIDs)
	require.Equal(t, []SkippedRow{{2, "out_of_scope"}, {4, "out_of_scope"}, {5, "out_of_scope"}}, res.Skipped)

	// Only out-of-scope rows: 403. Manage transaction settings covers Asset Name rows.
	_, err = svc.DeleteOverrides(context.Background(), delIn(access, "2"))
	require.ErrorIs(t, err, ErrOverrideForbidden)
	res, err = svc.DeleteOverrides(context.Background(), delIn(CallerAccess{AssetNameWrite: true}, "2"))
	require.NoError(t, err)
	require.Equal(t, []int64{2}, res.DeletedIDs)
	// An asset that no longer exists uses the snapshot Group (3 -> covered).
	groups.current = map[int64]int64{}
	res, err = svc.DeleteOverrides(context.Background(), delIn(access, "3"))
	require.NoError(t, err)
	require.Equal(t, []int64{3}, res.DeletedIDs)
}

func TestDeleteOverrides_ScopeProviderErrors(t *testing.T) {
	repo := newOverrideRepo()
	repo.byID = []model.TransactionRuleOverride{ov(1, "group", "borrow", 10, `{"max_duration":3}`), ov(3, "asset", "borrow", 30, `{"max_duration":3}`)}
	access := CallerAccess{WriteGroupIDs: []int64{3}}
	_, err := NewService(repo, nil).DeleteOverrides(context.Background(), delIn(access, "1"))
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable)
	_, err = NewService(repo, nil).WithGroupScopeDirectory(&fakeGroups{expandErr: errBoom}).DeleteOverrides(context.Background(), delIn(access, "1"))
	require.ErrorIs(t, err, errBoom)
	_, err = NewService(repo, nil).WithGroupScopeDirectory(&fakeGroups{expanded: []int64{3}, currErr: errBoom}).DeleteOverrides(context.Background(), delIn(access, "3"))
	require.ErrorIs(t, err, errBoom)
	// A caller with no covering right on a group row skips it without any directory call.
	_, err = NewService(repo, nil).DeleteOverrides(context.Background(), delIn(CallerAccess{AssetNameWrite: true}, "1"))
	require.ErrorIs(t, err, ErrOverrideForbidden)
	repo.byIDErr = errBoom
	_, err = NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.ErrorIs(t, err, errBoom)
}

func TestDeleteOverrides_AssigneeGuard(t *testing.T) {
	// Override 1 (All categories, Employee ON) keeps override 2 (Employee OFF) from both OFF;
	// Company default Division is OFF, so deleting 1 would leave 2 with both OFF.
	repo := newOverrideRepo()
	one := ov(1, "group", "assign", 10, `{"allowed_assignee_employee":true}`)
	two := ov(2, "group", "assign", 10, `{"allowed_assignee_employee":false}`)
	repo.byID = []model.TransactionRuleOverride{one, two, ov(3, "group", "borrow", 11, `{"max_duration":3}`)}
	repo.forTargets = []model.TransactionRuleOverride{one, two}
	repo.company = []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_division", "false")}
	svc := NewService(repo, nil)
	_, err := svc.DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	var guard *GuardSkipError
	require.ErrorAs(t, err, &guard)
	require.Equal(t, []SkippedRow{{1, "assignee_guard"}}, guard.Skipped)
	require.Equal(t, "at least one assignee type must be enabled", guard.Error())
	// Mixed: the guard row is skipped, the other row deleted -> 200 with skipped.
	res, err := svc.DeleteOverrides(context.Background(), delIn(totalControl, "1", "3"))
	require.NoError(t, err)
	require.Equal(t, []int64{3}, res.DeletedIDs)
	require.Equal(t, []SkippedRow{{1, "assignee_guard"}}, res.Skipped)
	// Override 2 itself is deletable: nothing remains with both OFF.
	res, err = svc.DeleteOverrides(context.Background(), delIn(totalControl, "2"))
	require.NoError(t, err)
	require.Equal(t, []int64{2}, res.DeletedIDs)
	// The guard also reports when out_of_scope rows are present, but never for them.
	repo.byID = append(repo.byID, ov(4, "asset_name", "assign", 20, `{}`))
	_, err = svc.DeleteOverrides(context.Background(), delIn(CallerAccess{WriteGroupIDs: []int64{3}, TotalControl: false}, "4"))
	require.Error(t, err)
}

func TestDeleteOverrides_GuardEvaluation(t *testing.T) {
	leaf := int64p(42)
	cases := map[string]struct {
		plan    Plan
		company []model.TransactionRuleField
		cat     []model.TransactionRuleField
		others  []model.TransactionRuleOverride
		guarded bool
	}{
		"effective deny remaining": {FullPlan, []model.TransactionRuleField{storedRow(nil, "assign", "allow_assign", "false")},
			nil, []model.TransactionRuleOverride{{ID: 2, Scope: "group", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_employee":false,"allowed_assignee_division":false}`)}}, false},
		"remaining both off": {FullPlan, nil, nil,
			[]model.TransactionRuleOverride{{ID: 2, Scope: "group", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_employee":false,"allowed_assignee_division":false}`)}}, true},
		"leaf remaining falls to all categories sibling": {FullPlan, nil,
			[]model.TransactionRuleField{storedRow(leaf, "assign", "allowed_assignee_employee", "false")},
			[]model.TransactionRuleOverride{
				{ID: 2, Scope: "group", TargetID: 10, CategoryID: leaf, Fields: json.RawMessage(`{}`)},
				{ID: 3, Scope: "group", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_division":true}`)},
				{ID: 4, Scope: "asset", TargetID: 10, Fields: json.RawMessage(`{}`)},
			}, false},
		"assignment basic": {Plan{ApprovalWorkflowAvailable: true, FeatureSuite: dto.FeatureSuite{Assignment: constants.LevelBasic}}, nil, nil,
			[]model.TransactionRuleOverride{{ID: 2, Scope: "group", TargetID: 10, Fields: json.RawMessage(`{"allowed_assignee_employee":false,"allowed_assignee_division":false}`)}}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := newOverrideRepo()
			first := ov(1, "group", "assign", 10, `{}`)
			repo.byID = []model.TransactionRuleOverride{first}
			repo.forTargets = append([]model.TransactionRuleOverride{first}, tc.others...)
			repo.company, repo.leaf = tc.company, tc.cat
			in := delIn(totalControl, "1")
			in.Plan = tc.plan
			_, err := NewService(repo, nil).DeleteOverrides(context.Background(), in)
			if tc.guarded {
				require.ErrorAs(t, err, new(*GuardSkipError))
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestDeleteOverrides_Failures(t *testing.T) {
	base := func() *fakeOverrideRepo {
		repo := newOverrideRepo()
		repo.byID = []model.TransactionRuleOverride{ov(1, "group", "borrow", 10, `{"max_duration":3}`)}
		return repo
	}
	for name, mutate := range map[string]func(r *fakeOverrideRepo){
		"begin":    func(r *fakeOverrideRepo) { r.beginErr = errBoom },
		"delete":   func(r *fakeOverrideRepo) { r.otx.deleteErr = errBoom },
		"bump":     func(r *fakeOverrideRepo) { r.otx.bumpErr = errBoom },
		"commit":   func(r *fakeOverrideRepo) { r.otx.commitErr = errBoom },
		"rollback": func(r *fakeOverrideRepo) { r.otx.deleteErr, r.otx.rollbackErr = errBoom, errBoom },
	} {
		t.Run(name, func(t *testing.T) {
			repo := base()
			mutate(repo)
			_, err := NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
			require.ErrorIs(t, err, errBoom)
		})
	}
	acts := &fakeActivities{err: errBoom}
	_, err := NewService(base(), nil).WithActivityEntryDispatcher(acts).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.NoError(t, err)
}

func TestDeleteOverrides_GuardProviderFailures(t *testing.T) {
	repo := newOverrideRepo()
	first := ov(1, "group", "assign", 10, `{}`)
	repo.byID = []model.TransactionRuleOverride{first}
	repo.forTargets = []model.TransactionRuleOverride{first, {ID: 2, Scope: "group", TargetID: 10, CategoryID: int64p(42), Fields: json.RawMessage(`{}`)}}
	repo.forErr = errBoom
	_, err := NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.ErrorIs(t, err, errBoom)
	repo.forErr = nil
	repo.err = errBoom // company read
	_, err = NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.ErrorIs(t, err, errBoom)
	repo.err = nil
	repo.leafErr = errBoom // category read of the remaining override
	_, err = NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.ErrorIs(t, err, errBoom)
	// Category fields are read once per category (cached) and an unreadable second find fails.
	repo.leafErr = nil
	repo.forTargets = append(repo.forTargets, model.TransactionRuleOverride{ID: 3, Scope: "group", TargetID: 10, CategoryID: int64p(42), Fields: json.RawMessage(`{}`)})
	_, err = NewService(repo, nil).DeleteOverrides(context.Background(), delIn(totalControl, "1"))
	require.NoError(t, err)
}
