package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
	iamhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/iam/http"
	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

func TestCategoryDirectoryReportsLeafParentAndMissing(t *testing.T) {
	o := &owners{categories: []marhttp.CategoryLookupItem{
		{ID: "5", Found: true, FullPath: ptr("Root > Leaf"), IsLeaf: ptr(true)},
		{ID: "6", Found: true, FullPath: ptr("Root"), IsLeaf: ptr(false)},
		{ID: "7", Found: false},
	}}
	mar, _, _ := o.clients(t)
	dir := CategoryDirectory{mar: mar}

	leaf, err := dir.LookupCategory(context.Background(), testClientID, 5)
	require.NoError(t, err)
	require.Equal(t, service.CategoryInfo{Exists: true, Path: "Root > Leaf", Leaf: true}, leaf)

	parent, err := dir.LookupCategory(context.Background(), testClientID, 6)
	require.NoError(t, err)
	require.Equal(t, service.CategoryInfo{Exists: true, Path: "Root", Leaf: false}, parent)

	for _, id := range []int64{7, 9} {
		missing, err := dir.LookupCategory(context.Background(), testClientID, id)
		require.NoError(t, err)
		require.Equal(t, service.CategoryInfo{}, missing)
	}

	o.failMAR = true
	_, err = dir.LookupCategory(context.Background(), testClientID, 5)
	require.ErrorContains(t, err, "lookup category")

	o.failMAR = false
	o.categories = []marhttp.CategoryLookupItem{{ID: "x", Found: true}}
	_, err = dir.LookupCategory(context.Background(), testClientID, 5)
	require.ErrorContains(t, err, "lookup category")
}

func TestAssetDirectoryResolvesInRequestOrderWithGroupChains(t *testing.T) {
	o := &owners{
		assets: []marhttp.AssetResolveItem{
			{ID: "10", Found: true, DisplayName: ptr("Laptop - 1"), AssetNameID: ptr("3"), AssetNameName: ptr("Laptop"),
				CategoryID: ptr("5"), CategoryPath: ptr("IT > Laptop"), CategoryIsLeaf: ptr(true), GroupID: ptr("2")},
			{ID: "11", Found: true, DisplayName: ptr("Chair"), AssetNameID: ptr("4"), AssetNameName: ptr("Chair"),
				CategoryID: ptr("6"), CategoryPath: ptr("Furniture"), CategoryIsLeaf: ptr(false)},
		},
		groups: []emshttp.GroupItem{
			{ID: 1, FullPath: "Root"},
			{ID: 2, FullPath: "Root > Child", ParentID: ptr(int64(1))},
		},
	}
	mar, _, iam := o.clients(t)
	dir := AssetDirectory{mar: mar, iam: iam}

	out, err := dir.ResolveAssets(context.Background(), testClientID, []int64{11, 10})
	require.NoError(t, err)
	require.Equal(t, []service.AssetContext{
		{ID: 11, Label: "Chair", CategoryPath: "Furniture", AssetNameID: 4, AssetName: "Chair"},
		{
			ID: 10, Label: "Laptop - 1", CategoryID: ptr(int64(5)), CategoryPath: "IT > Laptop",
			AssetNameID: 3, AssetName: "Laptop",
			Groups: []service.GroupRef{{ID: 2, Path: "Root > Child"}, {ID: 1, Path: "Root"}},
		},
	}, out)

	empty, err := dir.ResolveAssets(context.Background(), testClientID, nil)
	require.NoError(t, err)
	require.Nil(t, empty)

	_, err = dir.ResolveAssets(context.Background(), testClientID, []int64{10, 99})
	require.ErrorIs(t, err, service.ErrAssetNotFound)

	o.failMAR = true
	_, err = dir.ResolveAssets(context.Background(), testClientID, []int64{10})
	require.ErrorContains(t, err, "resolve assets")
	o.failMAR = false

	o.failIAM = true
	_, err = dir.ResolveAssets(context.Background(), testClientID, []int64{10})
	require.ErrorContains(t, err, "read groups")
	o.failIAM = false

	for _, bad := range []marhttp.AssetResolveItem{
		{ID: "10", Found: true, AssetNameID: ptr("x")},
		{ID: "10", Found: true},
		{ID: "10", Found: true, AssetNameID: ptr("3"), CategoryIsLeaf: ptr(true), CategoryID: ptr("x")},
		{ID: "10", Found: true, AssetNameID: ptr("3"), GroupID: ptr("x")},
	} {
		o.assets = []marhttp.AssetResolveItem{bad}
		_, err = dir.ResolveAssets(context.Background(), testClientID, []int64{10})
		require.Error(t, err)
	}
	o.assets = []marhttp.AssetResolveItem{{ID: "x", Found: true}}
	_, err = dir.ResolveAssets(context.Background(), testClientID, []int64{10})
	require.Error(t, err)
}

func TestOverrideTargetsForEachScope(t *testing.T) {
	o := &owners{
		groups: []emshttp.GroupItem{
			{ID: 1, FullPath: "Root"},
			{ID: 2, FullPath: "Root > Child", ParentID: ptr(int64(1))},
		},
		assetNames: []marhttp.AssetNameLookupItem{
			{ID: "4", Found: true, Name: ptr("Laptop")},
			{ID: "5", Found: false},
		},
		assets: []marhttp.AssetResolveItem{
			{ID: "10", Found: true, DisplayName: ptr("Laptop - 1"), GroupID: ptr("2")},
			{ID: "11", Found: true, DisplayName: ptr("Chair")},
		},
	}
	mar, _, iam := o.clients(t)
	dir := OverrideTargetDirectory{mar: mar, iam: iam}
	ctx := context.Background()

	groups, err := dir.ResolveTargets(ctx, testClientID, constants.ScopeGroup, []int64{2, 9})
	require.NoError(t, err)
	require.Equal(t, []service.OverrideTarget{{ID: 2, Exists: true, Name: "Root > Child", GroupChain: []int64{2, 1}}}, groups)

	names, err := dir.ResolveTargets(ctx, testClientID, constants.ScopeAssetName, []int64{5, 4})
	require.NoError(t, err)
	require.Equal(t, []service.OverrideTarget{{ID: 4, Exists: true, Name: "Laptop"}}, names)

	assets, err := dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, []int64{10, 11, 12})
	require.NoError(t, err)
	require.Equal(t, []service.OverrideTarget{
		{ID: 10, Exists: true, Name: "Laptop - 1", GroupID: ptr(int64(2)), GroupChain: []int64{2, 1}},
		{ID: 11, Exists: true, Name: "Chair"},
	}, assets)

	none, err := dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, nil)
	require.NoError(t, err)
	require.Empty(t, none)

	_, err = dir.ResolveTargets(ctx, testClientID, "unknown", []int64{1})
	require.ErrorIs(t, err, errUnknownScope)

	o.failIAM = true
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeGroup, []int64{1})
	require.ErrorContains(t, err, "read groups")
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, []int64{10})
	require.ErrorContains(t, err, "read groups")
	o.failIAM = false

	o.failMAR = true
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAssetName, []int64{4})
	require.Error(t, err)
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, []int64{10})
	require.Error(t, err)
	o.failMAR = false

	o.assetNames = []marhttp.AssetNameLookupItem{{ID: "x", Found: true}}
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAssetName, []int64{4})
	require.Error(t, err)
	o.assetNames = nil
	o.assets = []marhttp.AssetResolveItem{{ID: "x", Found: true}}
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, []int64{10})
	require.Error(t, err)
	o.assets = []marhttp.AssetResolveItem{{ID: "10", Found: true, GroupID: ptr("x")}}
	_, err = dir.ResolveTargets(ctx, testClientID, constants.ScopeAsset, []int64{10})
	require.Error(t, err)
}

func TestAssetTargetsSkipTheGroupReadWithoutAssets(t *testing.T) {
	o := &owners{}
	mar, _, iam := o.clients(t)
	dir := OverrideTargetDirectory{mar: mar, iam: iam}
	_, err := dir.ResolveTargets(context.Background(), testClientID, constants.ScopeAsset, []int64{10})
	require.NoError(t, err)
	require.Zero(t, o.groupCalls.Load())
}

func TestGroupScopeExpandsAndReadsCurrentGroups(t *testing.T) {
	o := &owners{
		groups: []emshttp.GroupItem{
			{ID: 1, FullPath: "Root"},
			{ID: 2, FullPath: "Root > Child", ParentID: ptr(int64(1))},
			{ID: 3, FullPath: "Root > Child > Leaf", ParentID: ptr(int64(2))},
		},
		assets: []marhttp.AssetResolveItem{
			{ID: "10", Found: true, GroupID: ptr("2")},
			{ID: "11", Found: true},
			{ID: "12", Found: false, GroupID: ptr("3")},
		},
	}
	mar, _, iam := o.clients(t)
	dir := GroupScopeDirectory{mar: mar, iam: iam}
	ctx := context.Background()

	expanded, err := dir.ExpandGroups(ctx, testClientID, []int64{2})
	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, expanded)

	none, err := dir.ExpandGroups(ctx, testClientID, nil)
	require.NoError(t, err)
	require.Nil(t, none)

	groups, err := dir.CurrentAssetGroups(ctx, testClientID, []int64{10, 11, 12})
	require.NoError(t, err)
	require.Equal(t, map[int64]int64{10: 2}, groups)

	o.failIAM = true
	_, err = dir.ExpandGroups(ctx, testClientID, []int64{2})
	require.ErrorContains(t, err, "read groups")
	o.failIAM = false

	o.failMAR = true
	_, err = dir.CurrentAssetGroups(ctx, testClientID, []int64{10})
	require.ErrorContains(t, err, "read asset groups")
	o.failMAR = false

	o.assets = []marhttp.AssetResolveItem{{ID: "x", Found: true, GroupID: ptr("2")}}
	_, err = dir.CurrentAssetGroups(ctx, testClientID, []int64{10})
	require.Error(t, err)
	o.assets = []marhttp.AssetResolveItem{{ID: "10", Found: true, GroupID: ptr("x")}}
	_, err = dir.CurrentAssetGroups(ctx, testClientID, []int64{10})
	require.Error(t, err)
}

func TestDenyContactAsksFromTheAssetGroup(t *testing.T) {
	o := &owners{contact: iamhttp.ManagerContactResponse{Name: "Rina Wulandari", Role: "Group Manager"}}
	_, _, iam := o.clients(t)
	dir := DenyContactResolver{iam: iam}
	ctx := context.Background()

	contact, err := dir.ResolveDenyContact(ctx, testClientID, []int64{7, 1})
	require.NoError(t, err)
	require.Equal(t, "Rina Wulandari", contact.Name)
	require.Equal(t, "Group Manager", contact.Role)
	require.EqualValues(t, 7, o.lastContact.Load())

	_, err = dir.ResolveDenyContact(ctx, testClientID, nil)
	require.ErrorIs(t, err, errNoContactGroup)

	o.failIAM = true
	_, err = dir.ResolveDenyContact(ctx, testClientID, []int64{7})
	require.ErrorContains(t, err, "manager contact")
}
