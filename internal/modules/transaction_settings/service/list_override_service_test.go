package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

func listReq() dto.ListOverridesRequest {
	return dto.ListOverridesRequest{Columns: []string{"scope", "target", "overridden", "createdBy", "category", "createdAt"}, Transaction: "borrow"}
}

func listIn(req dto.ListOverridesRequest) ListOverridesInput {
	return ListOverridesInput{ClientID: "c1", Request: req, Plan: FullPlan, Access: totalControl}
}

func ip(v int) *int       { return &v }
func sp(s string) *string { return &s }

func TestListOverrides_RequestValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(r *dto.ListOverridesRequest)
		field  string
	}{
		"columns missing":  {func(r *dto.ListOverridesRequest) { r.Columns = nil }, "columns"},
		"transaction":      {func(r *dto.ListOverridesRequest) { r.Transaction = "loan" }, "transaction"},
		"page zero":        {func(r *dto.ListOverridesRequest) { r.Page = ip(0) }, "page"},
		"limit seven":      {func(r *dto.ListOverridesRequest) { r.Limit = ip(7) }, "limit"},
		"sort by id":       {func(r *dto.ListOverridesRequest) { r.SortBy = sp("id") }, "sortBy"},
		"sort order two":   {func(r *dto.ListOverridesRequest) { r.SortOrder = ip(2) }, "sortOrder"},
		"bad scope":        {func(r *dto.ListOverridesRequest) { r.Scope = []string{"team"} }, "scope"},
		"bad category":     {func(r *dto.ListOverridesRequest) { r.CategoryID = sp("x") }, "categoryId"},
		"bad category ids": {func(r *dto.ListOverridesRequest) { r.Category = []string{"x"} }, "category[0]"},
		"reversed range":   {func(r *dto.ListOverridesRequest) { r.CreatedAt = json.RawMessage(`{"from":2,"to":1}`) }, "createdAt"},
		"malformed range":  {func(r *dto.ListOverridesRequest) { r.CreatedAt = json.RawMessage(`{"from":"x"}`) }, "createdAt"},
		"range not object": {func(r *dto.ListOverridesRequest) { r.CreatedAt = json.RawMessage(`5`) }, "createdAt"},
		"range missing to": {func(r *dto.ListOverridesRequest) { r.CreatedAt = json.RawMessage(`{"from":1}`) }, "createdAt"},
	} {
		t.Run(name, func(t *testing.T) {
			req := listReq()
			tc.mutate(&req)
			_, err := NewService(newOverrideRepo(), nil).ListOverrides(context.Background(), listIn(req))
			errs := validationErrors(t, err)
			require.Equal(t, tc.field, errs[0].Field)
		})
	}
}

func TestListOverrides_ResolvesFilters(t *testing.T) {
	repo := newOverrideRepo()
	svc := NewService(repo, nil)
	req := listReq()
	req.CategoryID = sp("42")
	req.Page, req.Limit, req.SortBy, req.SortOrder = ip(2), ip(25), sp("target"), ip(1)
	req.Search = "  max dur "
	req.Columns = []string{"overridden", "createdAt", "other"}
	req.Scope = []string{"group"}
	req.Overridden = []string{"Max duration"}
	req.CreatedBy = []string{"Rina"}
	req.Category = []string{"7"}
	req.CreatedAt = json.RawMessage(`{"from":1,"to":2}`)
	data, err := svc.ListOverrides(context.Background(), listIn(req))
	require.NoError(t, err)
	require.Empty(t, data.Data)
	q := repo.lastQuery
	require.Equal(t, 2, q.Page)
	require.Equal(t, 25, q.Limit)
	require.Equal(t, "target", q.SortBy)
	require.Equal(t, 1, q.SortOrder)
	require.Equal(t, int64(42), *q.CategoryID)
	require.Equal(t, "max dur", q.Search)
	require.Equal(t, []string{"overridden"}, q.SearchColumns)
	require.Equal(t, []string{"max_duration"}, q.SearchFieldKeys)
	require.Equal(t, []string{"max_duration"}, q.OverriddenKeys)
	require.Equal(t, []int64{7}, q.CategoryIDs)
	require.Equal(t, int64(1), *q.CreatedFrom)
	require.Nil(t, q.VisibleGroupIDs)

	// A label of no field of the transaction matches nothing (non-nil empty key set).
	req = listReq()
	req.Overridden = []string{"Nope"}
	_, err = svc.ListOverrides(context.Background(), listIn(req))
	require.NoError(t, err)
	require.NotNil(t, repo.lastQuery.OverriddenKeys)
	require.Empty(t, repo.lastQuery.OverriddenKeys)
	// Defaults.
	_, err = svc.ListOverrides(context.Background(), listIn(listReq()))
	require.NoError(t, err)
	require.Equal(t, "createdAt", repo.lastQuery.SortBy)
	require.Equal(t, -1, repo.lastQuery.SortOrder)
	require.Equal(t, 10, repo.lastQuery.Limit)
}

func TestListOverrides_StorageAndRepoErrors(t *testing.T) {
	_, err := NewService(&fakeRepo{}, nil).ListOverrides(context.Background(), listIn(listReq()))
	require.ErrorIs(t, err, ErrOverrideStorageUnavailable)
	repo := newOverrideRepo()
	repo.listErr = errBoom
	_, err = NewService(repo, nil).ListOverrides(context.Background(), listIn(listReq()))
	require.ErrorIs(t, err, errBoom)
}

func TestListOverrides_Visibility(t *testing.T) {
	repo := newOverrideRepo()
	svc := NewService(repo, nil)
	in := listIn(listReq())
	in.Access = CallerAccess{}
	data, err := svc.ListOverrides(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, dto.ListOverridesData{Data: []dto.OverrideRow{}}, data)

	in.Access = CallerAccess{ViewGroupIDs: []int64{1}}
	_, err = svc.ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable)
	svc.WithGroupScopeDirectory(&fakeGroups{expandErr: errBoom})
	_, err = svc.ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, errBoom)
	svc.WithGroupScopeDirectory(&fakeGroups{expanded: []int64{1, 2}})
	_, err = svc.ListOverrides(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []int64{1, 2}, *repo.lastQuery.VisibleGroupIDs)
	svc.WithGroupScopeDirectory(&fakeGroups{})
	_, err = svc.ListOverrides(context.Background(), in)
	require.NoError(t, err)
	require.NotNil(t, repo.lastQuery.VisibleGroupIDs)
	require.Empty(t, *repo.lastQuery.VisibleGroupIDs)
}

func rec(id, scope, target, catID, fields string, group *string) dto.OverrideListRecord {
	r := dto.OverrideListRecord{
		ID: id, Scope: scope, Target: "T" + id, Category: "All categories", Transaction: "borrow", Fields: json.RawMessage(fields),
		TargetID: target, CreatedBy: "Rina", CreatedAt: time.Date(2026, 6, 15, 14, 5, 0, 0, time.UTC), GroupID: group,
	}
	if catID != "" {
		r.CategoryID = &catID
		r.Category = "Vehicle"
	}
	return r
}

func TestListOverrides_RowsAppliedAndCanDelete(t *testing.T) {
	g := "3"
	repo := newOverrideRepo()
	repo.records = []dto.OverrideListRecord{
		rec("1", "group", "10", "", `{"max_duration":14,"max_extensions":0,"duration_unit":"hours"}`, &g),
		rec("2", "asset_name", "20", "", `{"self_service":true}`, nil),
		rec("3", "asset", "30", "42", `{"max_duration":3}`, &g),
		rec("4", "group", "11", "43", `{"max_duration":3}`, nil),
		rec("5", "asset", "31", "", `{"max_duration":3}`, nil),
	}
	repo.total = 5
	repo.company = []model.TransactionRuleField{storedRow(nil, "borrow", "duration_unit", `"hours"`)}
	targets := &fakeTargets{list: []OverrideTarget{{ID: 10, Exists: true}, {ID: 20, Exists: true}, {ID: 30, Exists: false}, {ID: 11, Exists: true}, {ID: 31, Exists: true}}}
	cats := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true}, 43: {Exists: false}}}
	svc := NewService(repo, nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(cats).
		WithGroupScopeDirectory(&fakeGroups{expanded: []int64{3, 10}})
	in := listIn(listReq())
	in.Plan.FeatureSuite.Borrowing = constants.LevelBasic
	in.Access = CallerAccess{ViewAll: true, WriteGroupIDs: []int64{3}}
	data, err := svc.ListOverrides(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, 5, data.TotalRecords)
	rows := data.Data
	require.Len(t, rows, 5)
	require.Equal(t, []dto.OverriddenField{
		{Key: "duration_unit", Label: "Duration unit", Value: "hours", DisplayValue: "Hours"},
		{Key: "max_duration", Label: "Max duration", Value: 14, DisplayValue: "14 hours"},
		{Key: "max_extensions", Label: "Max extensions", Value: 0, DisplayValue: "No extension"},
	}, rows[0].Overridden)
	// Basic Borrowing gates duration_unit, so row 1 is not applied for the plan; row 2 for self service.
	require.False(t, rows[0].Applied)
	require.Equal(t, constants.NotAppliedPlan, *rows[0].NotAppliedReason)
	require.True(t, rows[0].CanDelete)  // Group 10 inside the expanded subtree
	require.False(t, rows[1].CanDelete) // Asset Name needs Total Control or Manage transaction settings
	require.Equal(t, constants.NotAppliedPlan, *rows[1].NotAppliedReason)
	require.Equal(t, "14 hours", rows[0].Overridden[1].DisplayValue)
	require.Equal(t, constants.NotAppliedRemoved, *rows[2].NotAppliedReason) // target gone
	require.True(t, rows[2].CanDelete)                                       // snapshot group 3
	require.Equal(t, constants.NotAppliedRemoved, *rows[3].NotAppliedReason) // category gone
	require.False(t, rows[3].CanDelete)                                      // group 11 outside subtree
	require.True(t, rows[4].Applied)
	require.Nil(t, rows[4].NotAppliedReason)
	require.False(t, rows[4].CanDelete)                             // no snapshot group
	require.Equal(t, "3 hours", rows[4].Overridden[0].DisplayValue) // company unit applies

	// Total Control deletes everything; Read Only nothing; a Manage settings holder Asset Name rows.
	in.Access = totalControl
	data, err = svc.ListOverrides(context.Background(), in)
	require.NoError(t, err)
	require.True(t, data.Data[1].CanDelete)
	in.Access = CallerAccess{ViewAll: true, AssetNameWrite: true}
	data, _ = svc.ListOverrides(context.Background(), in)
	require.True(t, data.Data[1].CanDelete)
	require.False(t, data.Data[0].CanDelete)
	in.Access = CallerAccess{ViewAll: true, ReadOnly: true}
	data, _ = svc.ListOverrides(context.Background(), in)
	require.False(t, data.Data[1].CanDelete)
}

func TestListOverrides_ParentCategoryRowIsNotApplied(t *testing.T) {
	repo := newOverrideRepo()
	repo.records = []dto.OverrideListRecord{rec("9", "group", "10", "44", `{"max_duration":3}`, nil)}
	repo.total = 1
	targets := &fakeTargets{list: []OverrideTarget{{ID: 10, Exists: true}}}
	cats := &fakeCategories{infos: map[int64]CategoryInfo{44: {Exists: true}}}
	svc := NewService(repo, nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(cats)
	data, err := svc.ListOverrides(context.Background(), listIn(listReq()))
	require.NoError(t, err)
	require.False(t, data.Data[0].Applied)
	require.Equal(t, constants.NotAppliedRemoved, *data.Data[0].NotAppliedReason)
}

func TestListOverrides_ProviderFailures(t *testing.T) {
	g := "3"
	one := func() *fakeOverrideRepo {
		repo := newOverrideRepo()
		repo.records = []dto.OverrideListRecord{rec("1", "group", "10", "42", `{"max_duration":3}`, &g)}
		return repo
	}
	in := listIn(listReq())
	in.Access = CallerAccess{ViewAll: true, WriteGroupIDs: []int64{3}}
	targets := &fakeTargets{list: []OverrideTarget{{ID: 10, Exists: true}}}
	cats := &fakeCategories{infos: map[int64]CategoryInfo{42: {Exists: true, Leaf: true}}}
	groups := &fakeGroups{expanded: []int64{3}}
	ok := func() *Service {
		return NewService(one(), nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(cats).WithGroupScopeDirectory(groups)
	}
	_, err := ok().ListOverrides(context.Background(), in)
	require.NoError(t, err)

	_, err = NewService(one(), nil).ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable)
	svc := NewService(one(), nil).WithOverrideTargetDirectory(targets)
	_, err = svc.ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable) // no category directory
	svc.WithCategoryDirectory(cats)
	_, err = svc.ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable) // no group directory
	_, err = NewService(one(), nil).WithOverrideTargetDirectory(&fakeTargets{err: errBoom}).ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, errBoom)
	_, err = NewService(one(), nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(&fakeCategories{err: errBoom}).ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, errBoom)
	_, err = NewService(one(), nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(cats).
		WithGroupScopeDirectory(&fakeGroups{expandErr: errBoom}).ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, errBoom)
	bad := one()
	bad.err = errBoom
	_, err = NewService(bad, nil).WithOverrideTargetDirectory(targets).WithCategoryDirectory(cats).
		WithGroupScopeDirectory(groups).ListOverrides(context.Background(), in)
	require.ErrorIs(t, err, errBoom)
}

func TestOverrideOptions(t *testing.T) {
	repo := newOverrideRepo()
	repo.opts = OverrideOptionValues{
		Scopes: []string{"asset", "group"}, FieldKeys: []string{"max_duration", "unknown_key", "max_extensions"},
		CreatedBy: []string{"Siti", "Rina", "Rina"},
	}
	svc := NewService(repo, nil)
	in := OverridesOptionsInput{ClientID: "c1", Access: totalControl, Request: dto.OverridesOptionsRequest{
		Transaction: "borrow", Scope: true, Overridden: true, CreatedBy: true, Category: []string{"7"},
		CreatedAt: json.RawMessage(`{"from":1,"to":2}`),
	}}
	data, err := svc.OverrideOptions(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []dto.OptionItem{{Label: "Asset", Value: "Asset"}, {Label: "Group", Value: "Group"}}, *data.Scope)
	require.Equal(t, []dto.OptionItem{{Label: "Max duration", Value: "Max duration"}, {Label: "Max extensions", Value: "Max extensions"}, {Label: "unknown_key", Value: "unknown_key"}}, *data.Overridden)
	require.Equal(t, []dto.OptionItem{{Label: "Rina", Value: "Rina"}, {Label: "Siti", Value: "Siti"}}, *data.CreatedBy)
	require.True(t, repo.lastQuery.WantScope && repo.lastQuery.WantOverridden && repo.lastQuery.WantCreatedBy)
	require.Equal(t, "borrow", repo.lastQuery.Transaction)

	// Only the requested arrays are returned; an empty row set gives [].
	repo.opts = OverrideOptionValues{}
	in.Request = dto.OverridesOptionsRequest{Transaction: "borrow", Scope: true}
	data, err = svc.OverrideOptions(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []dto.OptionItem{}, *data.Scope)
	require.Nil(t, data.Overridden)
	require.Nil(t, data.CreatedBy)
}

func TestOverrideOptions_ErrorsAndVisibility(t *testing.T) {
	in := OverridesOptionsInput{ClientID: "c1", Access: totalControl, Request: dto.OverridesOptionsRequest{Transaction: "loan"}}
	_, err := NewService(newOverrideRepo(), nil).OverrideOptions(context.Background(), in)
	require.Equal(t, "transaction", validationErrors(t, err)[0].Field)
	_, err = NewService(&fakeRepo{}, nil).OverrideOptions(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideStorageUnavailable)

	in.Request = dto.OverridesOptionsRequest{Transaction: "borrow", Scope: true, CreatedBy: true}
	repo := newOverrideRepo()
	repo.optsErr = errBoom
	_, err = NewService(repo, nil).OverrideOptions(context.Background(), in)
	require.ErrorIs(t, err, errBoom)

	// A caller without any visible Group gets empty arrays without a read.
	repo = newOverrideRepo()
	repo.optsErr = errBoom
	in.Access = CallerAccess{}
	data, err := NewService(repo, nil).OverrideOptions(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, []dto.OptionItem{}, *data.Scope)
	// A Group-only caller needs the Group directory.
	in.Access = CallerAccess{ViewGroupIDs: []int64{1}}
	_, err = NewService(newOverrideRepo(), nil).OverrideOptions(context.Background(), in)
	require.ErrorIs(t, err, ErrOverrideDirectoryUnavailable)
}
