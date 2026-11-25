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

var totalControl = CallerAccess{TotalControl: true, ViewAll: true, AssetNameWrite: true}

func cf(key, value string) dto.CreateOverrideField {
	f := dto.CreateOverrideField{Key: key}
	if value != "" {
		f.Value = json.RawMessage(value)
	}
	return f
}

func createReq(scope, tx string, targets []string, fields ...dto.CreateOverrideField) dto.CreateOverrideRequest {
	return dto.CreateOverrideRequest{Scope: scope, TargetIDs: targets, Transaction: tx, Fields: fields}
}

func createInput(req dto.CreateOverrideRequest) CreateOverrideInput {
	return CreateOverrideInput{ClientID: "c1", Request: req, Plan: FullPlan, ActorID: 7, ActorName: "Rina Wulandari", Access: totalControl}
}

func groupTarget(id int64, name string, chain ...int64) OverrideTarget {
	return OverrideTarget{ID: id, Exists: true, Name: name, GroupChain: chain}
}

// createService wires a repo, one Group target 10 and an activity sink.
func createService(repo *fakeOverrideRepo, targets ...OverrideTarget) (*Service, *fakeActivities) {
	acts := &fakeActivities{}
	svc := NewService(repo, nil).WithActivityEntryDispatcher(acts).WithOverrideTargetDirectory(&fakeTargets{list: targets})
	return svc, acts
}

func TestCreateOverrides_WritesRowsBumpsOnceAndLogs(t *testing.T) {
	repo := newOverrideRepo()
	svc, acts := createService(repo, groupTarget(10, "Pusat > Karawang", 10, 1), groupTarget(11, "Pusat > Bekasi", 11, 1))
	err := svc.CreateOverrides(context.Background(), createInput(createReq("group", "borrow", []string{"10", "11", "10"},
		cf("max_duration", "30"), cf("max_assets_per_borrower", "2"))))
	require.NoError(t, err)
	require.Len(t, repo.otx.inserted, 2)
	row := repo.otx.inserted[0]
	require.Equal(t, "group", row.Scope)
	require.Equal(t, int64(10), row.TargetID)
	require.Equal(t, "Pusat > Karawang", row.TargetName)
	require.Nil(t, row.CategoryID)
	require.Nil(t, row.CategoryPath)
	require.Equal(t, int64(10), *row.TargetGroupID)
	require.JSONEq(t, `{"max_duration":30,"max_assets_per_borrower":2}`, string(row.Fields))
	require.Equal(t, "Rina Wulandari", row.CreatedByName)
	require.Equal(t, 1, repo.otx.bumps)
	require.True(t, repo.otx.committed)
	require.True(t, repo.otx.closed)
	fields := "Max duration (30 days), Max assets per borrower (concurrent) (2)"
	require.Equal(t, []ActivityLogEntry{
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionCreate, Object: "Borrow", ObjectName: "Pusat > Karawang in All categories", Field: fields},
		{ActorID: 7, ActorName: "Rina Wulandari", LogTable: LogTableTransactionSettings, Action: actionCreate, Object: "Borrow", ObjectName: "Pusat > Bekasi in All categories", Field: fields},
	}, acts.entries)
}

func TestCreateOverrides_SnapshotsPerScopeAndCategory(t *testing.T) {
	group := int64(3)
	repo := newOverrideRepo()
	acts := &fakeActivities{err: errBoom}
	cats := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true, Path: "Vehicle > Pool car"}, 43: {Exists: true, Leaf: true}}}
	svc := NewService(repo, nil).WithActivityEntryDispatcher(acts).WithCategoryDirectory(cats).WithOverrideTargetDirectory(&fakeTargets{list: []OverrideTarget{
		{ID: 20, Exists: true, Name: "Laptop", GroupID: &group},
		{ID: 21, Exists: true, Name: "A-1 · Laptop", GroupID: &group, GroupChain: []int64{3}},
	}})
	cat := "42"
	req := createReq("asset_name", "borrow", []string{"20"}, cf("duration_unit", `"hours"`), cf("max_duration", "6"))
	req.CategoryID = &cat
	require.NoError(t, svc.CreateOverrides(context.Background(), createInput(req)))
	row := repo.otx.inserted[0]
	require.Equal(t, int64(42), *row.CategoryID)
	require.Equal(t, "Vehicle > Pool car", *row.CategoryPath)
	require.Nil(t, row.TargetGroupID)
	require.Contains(t, acts.entries[0].ObjectName, "in Vehicle > Pool car")
	require.Equal(t, "Duration unit (Hours), Max duration (6 hours)", acts.entries[0].Field)

	repo2 := newOverrideRepo()
	svc2 := NewService(repo2, nil).WithCategoryDirectory(cats).WithOverrideTargetDirectory(&fakeTargets{list: []OverrideTarget{{ID: 21, Exists: true, Name: "A-1", GroupID: &group}}})
	cat = "43"
	req = createReq("asset", "borrow", []string{"21"}, cf("self_service", "true"))
	req.CategoryID = &cat
	require.NoError(t, svc2.CreateOverrides(context.Background(), createInput(req)))
	require.Equal(t, "43", *repo2.otx.inserted[0].CategoryPath)
	require.Equal(t, group, *repo2.otx.inserted[0].TargetGroupID)
}

func TestCreateOverrides_Authorization(t *testing.T) {
	targets := []OverrideTarget{groupTarget(10, "G", 10, 1), {ID: 11}}
	req := createReq("group", "borrow", []string{"10"}, cf("max_duration", "30"))
	groupRole := CallerAccess{WriteGroupIDs: []int64{1}}
	for name, tc := range map[string]struct {
		access CallerAccess
		req    dto.CreateOverrideRequest
	}{
		"read only":           {CallerAccess{ReadOnly: true, TotalControl: true}, req},
		"no rights":           {CallerAccess{}, req},
		"asset name scope":    {groupRole, createReq("asset_name", "borrow", []string{"10"}, cf("max_duration", "30"))},
		"group outside scope": {CallerAccess{WriteGroupIDs: []int64{99}}, req},
	} {
		t.Run(name, func(t *testing.T) {
			repo := newOverrideRepo()
			svc, _ := createService(repo, targets...)
			in := createInput(tc.req)
			in.Access = tc.access
			require.ErrorIs(t, svc.CreateOverrides(context.Background(), in), ErrOverrideForbidden)
			require.Empty(t, repo.otx.inserted)
		})
	}
	// Another entity's target is not found (404) whatever the caller holds, and nothing is written.
	foreignRepo := newOverrideRepo()
	foreignSvc, _ := createService(foreignRepo, targets...)
	foreignReq := createInput(createReq("group", "borrow", []string{"11"}, cf("max_duration", "30")))
	require.ErrorIs(t, foreignSvc.CreateOverrides(context.Background(), foreignReq), ErrTargetNotFound)
	require.Empty(t, foreignRepo.otx.inserted)
	// A Group Role holder covering an ancestor of the Group is allowed; a manage-settings holder
	// covers Asset Name scope.
	repo := newOverrideRepo()
	svc, _ := createService(repo, targets...)
	in := createInput(req)
	in.Access = groupRole
	require.NoError(t, svc.CreateOverrides(context.Background(), in))
	in = createInput(createReq("asset_name", "borrow", []string{"10"}, cf("max_duration", "30")))
	in.Access = CallerAccess{AssetNameWrite: true}
	require.NoError(t, svc.CreateOverrides(context.Background(), in))
}

func TestCreateOverrides_ValidationMessages(t *testing.T) {
	ok := []string{"10"}
	for name, tc := range map[string]struct {
		req   dto.CreateOverrideRequest
		field string
		msg   string
	}{
		"scope missing":        {createReq("", "borrow", ok, cf("max_duration", "30")), "scope", "You must pick a scope"},
		"group missing":        {createReq("group", "borrow", nil, cf("max_duration", "30")), "targetIds", "You must pick a group"},
		"asset name missing":   {createReq("asset_name", "borrow", nil, cf("max_duration", "30")), "targetIds", "You must pick an asset name"},
		"asset missing":        {createReq("asset", "borrow", nil, cf("max_duration", "30")), "targetIds", "You must pick an asset"},
		"bad target id":        {createReq("group", "borrow", []string{"x"}, cf("max_duration", "30")), "targetIds[0]", msgInvalidID},
		"field missing":        {createReq("group", "borrow", ok), "fields", "You must pick a field"},
		"field key empty":      {createReq("group", "borrow", ok, cf("", "1")), "fields[0].value", "You must pick a field"},
		"value missing":        {createReq("group", "borrow", ok, cf("max_duration", "")), "fields[0].value", "You must enter a value"},
		"choice value missing": {createReq("group", "borrow", ok, cf("allowed_borrower", `""`)), "fields[0].value", "You must pick a value"},
		"min one zero":         {createReq("group", "borrow", ok, cf("max_duration", "0")), "fields[0].value", "Value must be at least 1"},
		"auto approve zero":    {createReq("group", "borrow", ok, cf("auto_approve_max_duration", "0")), "fields[0].value", "Value must be at least 1"},
		"unknown field":        {createReq("group", "borrow", ok, cf("nope", "1")), "fields[0].value", msgUnknownField},
		"reserved field":       {createReq("group", "borrow", ok, cf("transaction_timeout", "5")), "fields[0].value", msgUnknownField},
		"multi select field":   {createReq("disposal", "disposal", ok, cf("allowed_disposal_types", `["sell"]`)), "fields[0].value", msgUnknownField},
		"duplicate field":      {createReq("group", "borrow", ok, cf("max_duration", "3"), cf("max_duration", "4")), "fields[1].value", msgDuplicateField},
		"audit transaction":    {createReq("group", "audit", ok, cf("grace_period", "3")), "transaction", msgInvalidChoice},
		"bad category":         {dto.CreateOverrideRequest{Scope: "group", TargetIDs: ok, Transaction: "borrow", Fields: []dto.CreateOverrideField{cf("max_duration", "3")}, CategoryID: strp("abc")}, "categoryId", msgInvalidID},
	} {
		t.Run(name, func(t *testing.T) {
			svc, _ := createService(newOverrideRepo())
			errs := validationErrors(t, svc.CreateOverrides(context.Background(), createInput(tc.req)))
			require.Contains(t, errs, dto.FieldError{Field: tc.field, Message: tc.msg})
		})
	}
}

func strp(s string) *string { return &s }

func TestCreateOverrides_ZeroAcceptedForMinZeroFields(t *testing.T) {
	repo := newOverrideRepo()
	svc, _ := createService(repo, groupTarget(10, "G", 10))
	require.NoError(t, svc.CreateOverrides(context.Background(), createInput(createReq("group", "borrow", []string{"10"}, cf("max_extensions", "0")))))
	require.JSONEq(t, `{"max_extensions":0}`, string(repo.otx.inserted[0].Fields))
}

func TestCreateOverrides_CategoryChecks(t *testing.T) {
	cat := "42"
	req := createReq("group", "borrow", []string{"10"}, cf("max_duration", "3"))
	req.CategoryID = &cat
	run := func(dir CategoryDirectory) error {
		svc, _ := createService(newOverrideRepo(), groupTarget(10, "G", 10))
		if dir != nil {
			svc.WithCategoryDirectory(dir)
		}
		return svc.CreateOverrides(context.Background(), createInput(req))
	}
	require.ErrorIs(t, run(nil), ErrOverrideDirectoryUnavailable)
	// Another entity's Category is answered like an unknown one: not found.
	require.ErrorIs(t, run(&fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: false}}}), ErrCategoryNotFound)
	require.ErrorIs(t, run(&fakeCategories{err: errBoom}), errBoom)
	// An unknown or deleted Category (Exists false) is 404; a parent Category is a 422 entry.
	require.ErrorIs(t, run(&fakeCategories{}), ErrCategoryNotFound)
	require.ErrorIs(t, run(&fakeCategories{infos: map[int64]CategoryInfo{42: {Leaf: true}}}), ErrCategoryNotFound)
	errs := validationErrors(t, run(&fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true}}}))
	require.Equal(t, []dto.FieldError{{Field: "categoryId", Message: msgNotLeaf}}, errs)
}

func TestCreateOverrides_TargetChecks(t *testing.T) {
	req := createReq("group", "borrow", []string{"10", "11"}, cf("max_duration", "3"))
	// A deleted target is answered like an unknown one: the providers return found=false for both, so both are 404.
	svc, _ := createService(newOverrideRepo(), groupTarget(10, "G", 10), OverrideTarget{ID: 11, Exists: false})
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), ErrTargetNotFound)

	svc = NewService(newOverrideRepo(), nil)
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), ErrOverrideDirectoryUnavailable)
	svc.WithOverrideTargetDirectory(&fakeTargets{err: errBoom})
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), errBoom)

	// An id the provider does not return is unknown: 404, and nothing is created.
	repo := newOverrideRepo()
	svc, _ = createService(repo)
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), ErrTargetNotFound)
	require.Empty(t, repo.otx.inserted)

	// Another entity's target is answered like an unknown one: 404.
	svc, _ = createService(newOverrideRepo(), groupTarget(10, "G", 10), OverrideTarget{ID: 11})
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), ErrTargetNotFound)

	// A deleted or disposed Asset, and a deleted Asset Name, are 404 too: no 422 entry is produced.
	svc, _ = createService(newOverrideRepo(), OverrideTarget{ID: 10, Exists: false})
	assetReq := createReq("asset", "borrow", []string{"10"}, cf("max_duration", "3"))
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(assetReq)), ErrTargetNotFound)
	svc, _ = createService(newOverrideRepo(), OverrideTarget{ID: 10, Exists: false})
	nameReq := createReq("asset_name", "borrow", []string{"10"}, cf("max_duration", "3"))
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(nameReq)), ErrTargetNotFound)

	// A parent Category (422) is listed before the not-found answer of a missing target.
	svc, _ = createService(newOverrideRepo())
	svc.WithCategoryDirectory(&fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true}}})
	parent := "42"
	parentReq := createReq("group", "borrow", []string{"10"}, cf("max_duration", "3"))
	parentReq.CategoryID = &parent
	errs := validationErrors(t, svc.CreateOverrides(context.Background(), createInput(parentReq)))
	require.Equal(t, []dto.FieldError{{Field: "categoryId", Message: msgNotLeaf}}, errs)
}

func TestCreateOverrides_PlanGate(t *testing.T) {
	svc, _ := createService(newOverrideRepo(), groupTarget(10, "G", 10))
	in := createInput(createReq("group", "borrow", []string{"10"}, cf("max_duration", "3"), cf("self_service", "true")))
	in.Plan.FeatureSuite.Borrowing = constants.LevelBasic
	errs := validationErrors(t, svc.CreateOverrides(context.Background(), in))
	require.Equal(t, []dto.FieldError{{Field: "fields[1].value", Message: msgFieldGated}}, errs)
	in.Plan.FeatureSuite.Borrowing = constants.LevelNone
	errs = validationErrors(t, svc.CreateOverrides(context.Background(), in))
	require.Len(t, errs, 2)
}

func TestCreateOverrides_AssigneeGuard(t *testing.T) {
	tgt := groupTarget(10, "G", 10)
	run := func(repo *fakeOverrideRepo, plan Plan, fields ...dto.CreateOverrideField) error {
		svc, _ := createService(repo, tgt)
		in := createInput(createReq("group", "assign", []string{"10"}, fields...))
		in.Plan = plan
		return svc.CreateOverrides(context.Background(), in)
	}
	repo := newOverrideRepo()
	errs := validationErrors(t, run(repo, FullPlan, cf("allowed_assignee_employee", "false"), cf("allowed_assignee_division", "false")))
	require.Equal(t, []dto.FieldError{
		{Field: "fields[0].value", Message: msgNoAssigneeType}, {Field: "fields[1].value", Message: msgNoAssigneeType},
	}, errs)
	// Employee OFF alone does not trip while Division stays ON.
	require.NoError(t, run(newOverrideRepo(), FullPlan, cf("allowed_assignee_employee", "false")))
	// Company default Division OFF + Employee OFF set here: only the row of this request is named.
	repo = newOverrideRepo()
	repo.company = []model.TransactionRuleField{storedRow(nil, "assign", "allowed_assignee_division", "false")}
	errs = validationErrors(t, run(repo, FullPlan, cf("allowed_assignee_employee", "false")))
	require.Equal(t, []dto.FieldError{{Field: "fields[0].value", Message: msgNoAssigneeType}}, errs)
	// An Allow Assign row is the entry (Allow opening an asset under a Deny).
	repo = newOverrideRepo()
	repo.company = []model.TransactionRuleField{
		storedRow(nil, "assign", "allow_assign", "false"), storedRow(nil, "assign", "allowed_assignee_employee", "false"),
		storedRow(nil, "assign", "allowed_assignee_division", "false"),
	}
	errs = validationErrors(t, run(repo, FullPlan, cf("max_assets_per_assignee", "2"), cf("allow_assign", "true")))
	require.Equal(t, []dto.FieldError{{Field: "fields[1].value", Message: msgNoAssigneeType}}, errs)
	// Effective Allow Assign Deny (stored at the Company default) skips the guard.
	require.NoError(t, run(repo, FullPlan, cf("max_assets_per_assignee", "2")))
	// ... and so does a Deny set by the override itself.
	require.NoError(t, run(newOverrideRepo(), FullPlan, cf("allow_assign", "false"),
		cf("allowed_assignee_employee", "false"), cf("allowed_assignee_division", "false")))
	// No assignee row in the request: the entry names the fields list.
	repo = newOverrideRepo()
	repo.company = []model.TransactionRuleField{
		storedRow(nil, "assign", "allowed_assignee_employee", "false"), storedRow(nil, "assign", "allowed_assignee_division", "false"),
	}
	errs = validationErrors(t, run(repo, FullPlan, cf("max_assets_per_assignee", "2")))
	require.Equal(t, []dto.FieldError{{Field: "fields", Message: msgNoAssigneeType}}, errs)
	// Assignment Basic never triggers the guard.
	basic := FullPlan
	basic.FeatureSuite.Assignment = constants.LevelBasic
	require.NoError(t, run(repo, basic, cf("max_assets_per_assignee", "2")))
}

func TestCreateOverrides_GuardReadsAllCategoriesSiblingAndCategory(t *testing.T) {
	cat := "42"
	repo := newOverrideRepo()
	repo.forTargets = []model.TransactionRuleOverride{
		{ID: 1, Scope: "group", TargetID: 10, TransactionType: "assign", Fields: json.RawMessage(`{"allowed_assignee_employee":false}`)},
		{ID: 2, Scope: "group", TargetID: 10, CategoryID: int64p(5), TransactionType: "assign", Fields: json.RawMessage(`{}`)},
		{ID: 3, Scope: "asset", TargetID: 10, TransactionType: "assign", Fields: json.RawMessage(`{}`)},
	}
	svc, _ := createService(repo, groupTarget(10, "G", 10))
	svc.WithCategoryDirectory(&fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true}}})
	req := createReq("group", "assign", []string{"10"}, cf("allowed_assignee_division", "false"))
	req.CategoryID = &cat
	errs := validationErrors(t, svc.CreateOverrides(context.Background(), createInput(req)))
	require.Equal(t, []dto.FieldError{{Field: "fields[0].value", Message: msgNoAssigneeType}}, errs)
}

func int64p(v int64) *int64 { return &v }

func TestCreateOverrides_Duplicates(t *testing.T) {
	req := createReq("group", "borrow", []string{"10", "11", "12"}, cf("max_duration", "3"))
	targets := []OverrideTarget{groupTarget(10, "A", 10), groupTarget(11, "B", 11), groupTarget(12, "C", 12)}
	repo := newOverrideRepo()
	repo.dups = [][]int64{{12, 10}}
	svc, _ := createService(repo, targets...)
	var dup *DuplicateOverrideError
	require.ErrorAs(t, svc.CreateOverrides(context.Background(), createInput(req)), &dup)
	require.Equal(t, []int64{10, 12}, dup.TargetIDs)
	require.Equal(t, "active override already exists", dup.Error())
	require.Empty(t, repo.otx.inserted)

	// A racing submit: the pre-check is clean, the insert hits the unique key, the rollback runs
	// and the 409 lists the targets found afterwards.
	repo = newOverrideRepo()
	repo.dups = [][]int64{nil, {11}}
	repo.otx.insertErr = ErrDuplicateOverride
	svc, _ = createService(repo, targets...)
	require.ErrorAs(t, svc.CreateOverrides(context.Background(), createInput(req)), &dup)
	require.Equal(t, []int64{11}, dup.TargetIDs)
	require.True(t, repo.otx.rolledBack)

	// When the second read finds nothing or fails, every target is named.
	repo = newOverrideRepo()
	repo.dups = [][]int64{nil, nil}
	repo.otx.insertErr = ErrDuplicateOverride
	svc, _ = createService(repo, targets...)
	require.ErrorAs(t, svc.CreateOverrides(context.Background(), createInput(req)), &dup)
	require.Equal(t, []int64{10, 11, 12}, dup.TargetIDs)
}

func TestCreateOverrides_Failures(t *testing.T) {
	req := createReq("group", "borrow", []string{"10"}, cf("max_duration", "3"))
	tgt := groupTarget(10, "A", 10)
	for name, tc := range map[string]func(r *fakeOverrideRepo){
		"company read":  func(r *fakeOverrideRepo) { r.err = errBoom },
		"dup read":      func(r *fakeOverrideRepo) { r.dupErr = errBoom },
		"begin":         func(r *fakeOverrideRepo) { r.beginErr = errBoom },
		"insert":        func(r *fakeOverrideRepo) { r.otx.insertErr = errBoom },
		"bump":          func(r *fakeOverrideRepo) { r.otx.bumpErr = errBoom },
		"commit":        func(r *fakeOverrideRepo) { r.otx.commitErr = errBoom },
		"rollback also": func(r *fakeOverrideRepo) { r.otx.insertErr, r.otx.rollbackErr = errBoom, errBoom },
	} {
		t.Run(name, func(t *testing.T) {
			repo := newOverrideRepo()
			tc(repo)
			svc, _ := createService(repo, tgt)
			require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(req)), errBoom)
		})
	}
	// A failed Category field read and a failed sibling read.
	cat := "42"
	withCat := req
	withCat.CategoryID = &cat
	cats := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true}}}
	repo := newOverrideRepo()
	repo.leafErr = errBoom
	svc, _ := createService(repo, tgt)
	svc.WithCategoryDirectory(cats)
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(withCat)), errBoom)
	repo = newOverrideRepo()
	repo.forErr = errBoom
	svc, _ = createService(repo, tgt)
	svc.WithCategoryDirectory(cats)
	assign := createReq("group", "assign", []string{"10"}, cf("max_assets_per_assignee", "2"))
	assign.CategoryID = &cat
	require.ErrorIs(t, svc.CreateOverrides(context.Background(), createInput(assign)), errBoom)

	// Without override storage the operation cannot run.
	plain := NewService(&fakeRepo{}, nil)
	require.ErrorIs(t, plain.CreateOverrides(context.Background(), createInput(req)), ErrOverrideStorageUnavailable)
}

func TestCreateOverrides_NilActivitiesAndFailingSink(t *testing.T) {
	repo := newOverrideRepo()
	svc := NewService(repo, nil).WithOverrideTargetDirectory(&fakeTargets{list: []OverrideTarget{groupTarget(10, "A", 10)}})
	require.NoError(t, svc.CreateOverrides(context.Background(), createInput(createReq("group", "borrow", []string{"10"}, cf("max_duration", "3")))))
	acts := &fakeActivities{err: errBoom}
	svc = NewService(newOverrideRepo(), nil).WithActivityEntryDispatcher(acts).WithOverrideTargetDirectory(&fakeTargets{list: []OverrideTarget{groupTarget(10, "A", 10)}})
	require.NoError(t, svc.CreateOverrides(context.Background(), createInput(createReq("group", "borrow", []string{"10"}, cf("max_duration", "3")))))
	require.Len(t, acts.entries, 1)
}
