package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

type fakeAssets struct {
	assets []AssetContext
	err    error
}

func (f *fakeAssets) ResolveAssets(context.Context, string, []int64) ([]AssetContext, error) {
	return f.assets, f.err
}

type fakeContacts struct {
	contact dto.DenyContact
	err     error
	groups  []int64
}

func (f *fakeContacts) ResolveDenyContact(_ context.Context, _ string, groups []int64) (dto.DenyContact, error) {
	f.groups = groups
	return f.contact, f.err
}

func override(scope string, target int64, cat *int64, tx, fields string) model.TransactionRuleOverride {
	return model.TransactionRuleOverride{Scope: scope, TargetID: target, CategoryID: cat, TransactionType: tx, Fields: []byte(fields)}
}

func effField(t *testing.T, a dto.EffectiveAsset, key string) dto.EffectiveField {
	t.Helper()
	for _, f := range a.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("field %s missing", key)
	return dto.EffectiveField{}
}

func effInput(tx string, plan Plan, ids ...int64) EffectiveInput {
	return EffectiveInput{ClientID: "c1", Transaction: tx, AssetIDs: ids, Plan: plan, Access: CallerAccess{TotalControl: true}}
}

func TestEffective_ClosestLevelWins(t *testing.T) {
	cat := int64(10)
	repo := &fakeRepo{
		version: 4,
		company: []model.TransactionRuleField{storedRow(nil, "borrow", "max_duration", "21")},
		leaf:    []model.TransactionRuleField{storedRow(&cat, "borrow", "max_duration", "14"), storedRow(&cat, "borrow", "max_extensions", "2")},
		overrides: []model.TransactionRuleOverride{
			override("asset_name", 5, nil, "borrow", `{"max_duration": 8, "max_assets_per_borrower": 9}`),
			override("group", 2, nil, "borrow", `{"max_duration": 9}`),
			override("group", 1, nil, "borrow", `{"max_duration": 10, "max_extensions": 4}`),
			override("asset", 100, &cat, "borrow", `{"max_duration": 3}`),
			override("asset", 100, nil, "borrow", `{"max_duration": 4, "max_assets_per_borrower": 6}`),
			override("asset", 999, nil, "borrow", `{"max_duration": 1}`),
			override("asset", 100, &cat, "assign", `{"max_duration": 77}`),
			override("group", 1, &[]int64{55}[0], "borrow", `{"max_extensions": 60}`),
		},
	}
	assets := &fakeAssets{assets: []AssetContext{{
		ID: 100, Label: "A-1 · Laptop", CategoryID: &cat, CategoryPath: "IT > Laptop", AssetNameID: 5, AssetName: "Laptop Pusat",
		Groups: []GroupRef{{ID: 1, Path: "HQ > IT"}, {ID: 2, Path: "HQ"}},
	}}}
	svc := NewService(repo, nil).WithAssetDirectory(assets)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 100))
	require.NoError(t, err)
	require.Len(t, out, 1)
	a := out[0]
	require.Equal(t, "100", a.AssetID)
	require.Equal(t, 4, a.SettingsVersion)
	require.True(t, a.Allowed)
	require.Nil(t, a.DenyContact)

	d := effField(t, a, "max_duration")
	require.Equal(t, 3, d.Value, "leaf-category asset override beats All categories asset override")
	require.Equal(t, dto.EffectiveField{Key: "max_duration", Value: 3, SourceLevel: "asset", SourceName: "A-1 · Laptop", SourceCategoryScope: "leaf"}, d)
	b := effField(t, a, "max_assets_per_borrower")
	require.Equal(t, 6, b.Value)
	require.Equal(t, "all", b.SourceCategoryScope)
	e := effField(t, a, "max_extensions")
	require.Equal(t, 4, e.Value)
	require.Equal(t, "group", e.SourceLevel)
	require.Equal(t, "HQ > IT", e.SourceName)
	require.Equal(t, "company_default", effField(t, a, "block_new_borrow_when_overdue").SourceLevel)
	require.Equal(t, "Company default", effField(t, a, "block_new_borrow_when_overdue").SourceName)
}

func TestEffective_CategoryAssetNameAndCompanyLevels(t *testing.T) {
	cat := int64(10)
	repo := &fakeRepo{
		company:   []model.TransactionRuleField{storedRow(nil, "borrow", "max_duration", "21"), storedRow(nil, "borrow", "transaction_timeout", "9")},
		leaf:      []model.TransactionRuleField{storedRow(&cat, "borrow", "max_duration", "14")},
		overrides: []model.TransactionRuleOverride{override("asset_name", 5, nil, "borrow", `{"max_extensions": 8, "transaction_timeout": 2}`)},
	}
	assets := &fakeAssets{assets: []AssetContext{
		{ID: 1, CategoryID: &cat, CategoryPath: "IT > Laptop", AssetNameID: 5, AssetName: "Laptop Pusat"},
		{ID: 2, AssetNameID: 6},
	}}
	svc := NewService(repo, nil).WithAssetDirectory(assets)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1, 2))
	require.NoError(t, err)
	require.Equal(t, 14, effField(t, out[0], "max_duration").Value)
	require.Equal(t, "category", effField(t, out[0], "max_duration").SourceLevel)
	require.Equal(t, "leaf", effField(t, out[0], "max_duration").SourceCategoryScope)
	require.Equal(t, "IT > Laptop", effField(t, out[0], "max_duration").SourceName)
	require.Equal(t, 8, effField(t, out[0], "max_extensions").Value)
	require.Equal(t, "asset_name", effField(t, out[0], "max_extensions").SourceLevel)
	require.Equal(t, 9, effField(t, out[0], "transaction_timeout").Value, "All categories only fields ignore overrides")
	require.Equal(t, 21, effField(t, out[1], "max_duration").Value, "asset without a leaf category follows the Company default")
	require.Equal(t, "company_default", effField(t, out[1], "max_duration").SourceLevel)
}

func TestEffective_CategoryOverrideStopsWhenCategoryChanged(t *testing.T) {
	other, cat := int64(11), int64(10)
	repo := &fakeRepo{overrides: []model.TransactionRuleOverride{
		override("asset", 1, &other, "borrow", `{"max_duration": 3}`),
		override("asset", 2, &cat, "borrow", `{"max_duration": 3}`),
	}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, CategoryID: &cat}, {ID: 2}}}
	out, err := NewService(repo, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1, 2))
	require.NoError(t, err)
	require.Equal(t, 14, effField(t, out[0], "max_duration").Value)
	require.Equal(t, 14, effField(t, out[1], "max_duration").Value)
}

func TestEffective_DenyOverrideWinsAndNeedsContact(t *testing.T) {
	repo := &fakeRepo{overrides: []model.TransactionRuleOverride{
		override("group", 2, nil, "borrow", `{"allow_borrow": false}`),
		override("asset", 1, nil, "borrow", `{"allow_borrow": true}`),
	}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, Label: "A", Groups: []GroupRef{{ID: 3, Path: "HQ > IT > Ops"}, {ID: 2, Path: "HQ > IT"}}}}}
	contacts := &fakeContacts{contact: dto.DenyContact{Name: "Budi", Role: "Group Manager"}}
	svc := NewService(repo, nil).WithAssetDirectory(assets).WithDenyContactResolver(contacts)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1))
	require.NoError(t, err)
	require.False(t, out[0].Allowed)
	require.Equal(t, &dto.DenyContact{Name: "Budi", Role: "Group Manager"}, out[0].DenyContact)
	require.Equal(t, []int64{3, 2}, contacts.groups)
	f := effField(t, out[0], "allow_borrow")
	require.Equal(t, false, f.Value)
	require.Equal(t, "group", f.SourceLevel)

	svc = NewService(repo, nil).WithAssetDirectory(assets)
	_, err = svc.ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1))
	require.ErrorIs(t, err, ErrDenyContactUnavailable)

	svc = NewService(repo, nil).WithAssetDirectory(assets).WithDenyContactResolver(&fakeContacts{err: errors.New("iam down")})
	_, err = svc.ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1))
	require.ErrorContains(t, err, "resolve deny contact")
}

func TestEffective_CompanyDenyWithoutOverride(t *testing.T) {
	repo := &fakeRepo{company: []model.TransactionRuleField{storedRow(nil, "transfer", "allow_transfer", "false")}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, Groups: []GroupRef{{ID: 2}}}}}
	svc := NewService(repo, nil).WithAssetDirectory(assets).WithDenyContactResolver(&fakeContacts{contact: dto.DenyContact{Name: "T", Role: "Total Control"}})
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("transfer", FullPlan, 1))
	require.NoError(t, err)
	require.False(t, out[0].Allowed)
	require.Equal(t, "company_default", effField(t, out[0], "allow_transfer").SourceLevel)
}

func TestEffective_TransactionsWithoutOverrideLevel(t *testing.T) {
	cat := int64(10)
	repo := &fakeRepo{
		company: []model.TransactionRuleField{storedRow(nil, "maintenance", "reminder_before_due", "5")},
		leaf:    []model.TransactionRuleField{storedRow(&cat, "maintenance", "reminder_before_due", "2")},
	}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, CategoryID: &cat, CategoryPath: "Vehicle"}, {ID: 2}}}
	svc := NewService(repo, nil).WithAssetDirectory(assets)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("maintenance", FullPlan, 1, 2))
	require.NoError(t, err)
	require.Equal(t, 2, effField(t, out[0], "reminder_before_due").Value)
	require.Equal(t, "category", effField(t, out[0], "reminder_before_due").SourceLevel)
	require.Equal(t, 5, effField(t, out[1], "reminder_before_due").Value)
	require.True(t, out[0].Allowed)
	require.Nil(t, out[0].DenyContact)

	out, err = svc.ResolveEffectiveRules(context.Background(), effInput("audit", FullPlan, 1))
	require.NoError(t, err)
	require.True(t, out[0].Allowed)
	require.Equal(t, "company_default", effField(t, out[0], "grace_period").SourceLevel)
}

func TestEffective_PlanGating(t *testing.T) {
	repo := &fakeRepo{company: []model.TransactionRuleField{
		storedRow(nil, "borrow", "duration_unit", `"hours"`),
		storedRow(nil, "borrow", "max_duration", "49"),
		storedRow(nil, "borrow", "auto_approve_max_duration", "24"),
		storedRow(nil, "borrow", "self_service", "true"),
	}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1}}}
	basic := FullPlan
	basic.FeatureSuite.Borrowing = constants.LevelBasic
	basic.ApprovalWorkflowAvailable = false
	out, err := NewService(repo, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), effInput("borrow", basic, 1))
	require.NoError(t, err)
	a := out[0]
	require.Equal(t, 3, effField(t, a, "max_duration").Value, "49 hours = ceil(49/24) days")
	require.False(t, effField(t, a, "max_duration").Gated)
	require.Equal(t, 1, effField(t, a, "auto_approve_max_duration").Value)
	require.True(t, effField(t, a, "auto_approve_max_duration").Gated)
	require.Equal(t, "days", effField(t, a, "duration_unit").Value)
	require.Equal(t, false, effField(t, a, "self_service").Value)
	require.Equal(t, false, effField(t, a, "require_approval").Value)
	require.True(t, effField(t, a, "require_approval").Gated)
	require.Equal(t, "any_one_approver", effField(t, a, "approval_type").Value)
	require.True(t, effField(t, a, "approval_type").Gated)
}

func TestEffective_AssignAndTransferBasic(t *testing.T) {
	repo := &fakeRepo{company: []model.TransactionRuleField{
		storedRow(nil, "assign", "allowed_assignee_employee", "false"),
		storedRow(nil, "assign", "allowed_assignee_division", "true"),
	}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1}}}
	plan := FullPlan
	plan.FeatureSuite.Assignment = constants.LevelBasic
	plan.FeatureSuite.Transfer = constants.LevelBasic
	svc := NewService(repo, nil).WithAssetDirectory(assets)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("assign", plan, 1))
	require.NoError(t, err)
	require.Equal(t, true, effField(t, out[0], "allowed_assignee_employee").Value)
	require.Equal(t, false, effField(t, out[0], "allowed_assignee_division").Value)
	out, err = svc.ResolveEffectiveRules(context.Background(), effInput("transfer", plan, 1))
	require.NoError(t, err)
	require.Equal(t, "origin_only", effField(t, out[0], "approval_scope").Value)
	require.True(t, effField(t, out[0], "in_transit_overdue_after").Gated)
	require.Equal(t, 7, effField(t, out[0], "in_transit_overdue_after").Value)

	plan.ApprovalWorkflowAvailable = false
	out, err = svc.ResolveEffectiveRules(context.Background(), effInput("transfer", plan, 1))
	require.NoError(t, err)
	require.Equal(t, "both", effField(t, out[0], "approval_scope").Value, "scope is only forced while Approval Workflow is available")
}

func TestEffective_SuiteNoneAndInactiveAddon(t *testing.T) {
	repo := &fakeRepo{company: []model.TransactionRuleField{storedRow(nil, "disposal", "min_asset_age_warning", "20")}}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1}}}
	plan := FullPlan
	plan.FeatureSuite.Disposal = constants.LevelNone
	plan.Addons.Maintenance = false
	svc := NewService(repo, nil).WithAssetDirectory(assets)
	out, err := svc.ResolveEffectiveRules(context.Background(), effInput("disposal", plan, 1))
	require.NoError(t, err)
	require.False(t, out[0].Allowed)
	require.Nil(t, out[0].DenyContact)
	f := effField(t, out[0], "min_asset_age_warning")
	require.True(t, f.Gated)
	require.Equal(t, 20, f.Value)

	out, err = svc.ResolveEffectiveRules(context.Background(), effInput("maintenance", plan, 1))
	require.NoError(t, err)
	require.True(t, out[0].Allowed)
	require.True(t, effField(t, out[0], "photo_after").Gated)
}

func TestEffective_Errors(t *testing.T) {
	in := effInput("borrow", FullPlan, 1)
	_, err := NewService(&fakeRepo{}, nil).ResolveEffectiveRules(context.Background(), in)
	require.ErrorIs(t, err, ErrAssetDirectoryUnavailable)

	for _, want := range []error{ErrAssetNotFound} {
		_, err = NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{err: want}).ResolveEffectiveRules(context.Background(), in)
		require.ErrorIs(t, err, ErrAssetNotFound)
	}

	cat := int64(1)
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, CategoryID: &cat}}}
	_, err = NewService(&fakeRepo{err: errors.New("db")}, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), in)
	require.ErrorContains(t, err, "read settings version")

	bad := &fakeRepo{overrides: []model.TransactionRuleOverride{override("asset", 1, nil, "borrow", `[1]`)}}
	_, err = NewService(bad, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), in)
	require.ErrorContains(t, err, "decode override")
}

func TestEffective_ScopeHidesWholeRequest(t *testing.T) {
	inScope := AssetContext{ID: 1, Groups: []GroupRef{{ID: 5}, {ID: 2}}}
	outside := AssetContext{ID: 2, Groups: []GroupRef{{ID: 9}}}
	// One asset outside the view Groups rejects the whole request, and nothing is resolved.
	in := effInput("borrow", FullPlan, 1, 2)
	in.Access = CallerAccess{ViewGroupIDs: []int64{2}}
	repo := &fakeRepo{}
	_, err := NewService(repo, nil).WithAssetDirectory(&fakeAssets{assets: []AssetContext{inScope, outside}}).ResolveEffectiveRules(context.Background(), in)
	require.ErrorIs(t, err, ErrAssetNotFound)
	require.Zero(t, repo.began)
	// A Group-role holder resolves the assets whose Group chain holds one of its view Groups.
	in = effInput("borrow", FullPlan, 1)
	in.Access = CallerAccess{ViewGroupIDs: []int64{2}}
	out, err := NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{assets: []AssetContext{inScope}}).ResolveEffectiveRules(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, out, 1)
	// Read-all and Total Control rights see every asset.
	for _, access := range []CallerAccess{{ViewAll: true}, {TotalControl: true}} {
		in = effInput("borrow", FullPlan, 2)
		in.Access = access
		_, err = NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{assets: []AssetContext{outside}}).ResolveEffectiveRules(context.Background(), in)
		require.NoError(t, err)
	}
	// A caller holding no Group Role has no Group scope to narrow the answer, and a Group-role holder
	// does not see an asset whose Group chain misses its view Groups.
	in = effInput("borrow", FullPlan, 2)
	in.Access = CallerAccess{}
	_, err = NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{assets: []AssetContext{outside}}).ResolveEffectiveRules(context.Background(), in)
	require.NoError(t, err)
	in.Access = CallerAccess{ViewGroupIDs: []int64{5}}
	_, err = NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{assets: []AssetContext{outside}}).ResolveEffectiveRules(context.Background(), in)
	require.ErrorIs(t, err, ErrAssetNotFound)
}

// failingRepo fails the n-th repository read.
type failingRepo struct {
	fakeRepo
	failFields int
	calls      int
	failOver   bool
}

func (f *failingRepo) FindNodeFields(ctx context.Context, c string, cat *int64) ([]model.TransactionRuleField, error) {
	f.calls++
	if f.calls == f.failFields {
		return nil, errors.New("fields down")
	}
	return f.fakeRepo.FindNodeFields(ctx, c, cat)
}

func (f *failingRepo) FindExportOverrides(ctx context.Context, c string, cat *int64) ([]model.TransactionRuleOverride, error) {
	if f.failOver {
		return nil, errors.New("overrides down")
	}
	return f.fakeRepo.FindExportOverrides(ctx, c, cat)
}

func TestEffective_ReadFailures(t *testing.T) {
	cat := int64(1)
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, CategoryID: &cat}}}
	in := effInput("borrow", FullPlan, 1)
	for name, repo := range map[string]*failingRepo{
		"company":   {failFields: 1},
		"leaf":      {failFields: 2},
		"overrides": {failOver: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewService(repo, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), in)
			require.Error(t, err)
		})
	}
}

func TestEffective_LeafCachedPerCategory(t *testing.T) {
	cat := int64(1)
	repo := &failingRepo{}
	assets := &fakeAssets{assets: []AssetContext{{ID: 1, CategoryID: &cat}, {ID: 2, CategoryID: &cat}}}
	_, err := NewService(repo, nil).WithAssetDirectory(assets).ResolveEffectiveRules(context.Background(), effInput("borrow", FullPlan, 1, 2))
	require.NoError(t, err)
	require.Equal(t, 2, repo.calls, "company once, leaf once")
}
