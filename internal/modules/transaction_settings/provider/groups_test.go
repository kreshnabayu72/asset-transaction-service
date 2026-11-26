package provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// sampleTree is Root (1) > Child (2) > Leaf (3), plus a second root Other (4).
func sampleTree() groupTree {
	return newGroupTree([]emshttp.GroupItem{
		{ID: 1, FullPath: "Root"},
		{ID: 2, FullPath: "Root > Child", ParentID: ptr(int64(1))},
		{ID: 3, FullPath: "Root > Child > Leaf", ParentID: ptr(int64(2))},
		{ID: 4, FullPath: "Other"},
	})
}

func TestGroupTreeChainAndPaths(t *testing.T) {
	tree := sampleTree()
	require.True(t, tree.has(3))
	require.False(t, tree.has(99))
	require.Equal(t, "Root > Child", tree.fullPath(2))
	require.Equal(t, []int64{3, 2, 1}, tree.chain(3))
	require.Nil(t, tree.chain(99))
	require.Equal(t, []service.GroupRef{{ID: 2, Path: "Root > Child"}, {ID: 1, Path: "Root"}}, tree.refs(tree.chain(2)))
	require.Nil(t, tree.refs(nil))
}

func TestGroupTreeChainStopsAtAParentLoop(t *testing.T) {
	loop := newGroupTree([]emshttp.GroupItem{
		{ID: 1, ParentID: ptr(int64(2))},
		{ID: 2, ParentID: ptr(int64(1))},
	})
	require.Equal(t, []int64{1, 2}, loop.chain(1))
}

func TestGroupTreeExpandReturnsDescendantsSorted(t *testing.T) {
	tree := sampleTree()
	require.Equal(t, []int64{1, 2, 3}, tree.expand([]int64{1}))
	require.Equal(t, []int64{1, 2, 3, 4}, tree.expand([]int64{4, 1, 1}))
	require.Equal(t, []int64{99}, tree.expand([]int64{99}))
	require.Nil(t, tree.expand(nil))
}

func TestLoadGroupsReadsTheTreeAndSurfacesFailure(t *testing.T) {
	o := &owners{groups: []emshttp.GroupItem{{ID: 1, FullPath: "Root"}}}
	_, _, iam := o.clients(t)
	tree, err := loadGroups(context.Background(), iam, testClientID)
	require.NoError(t, err)
	require.True(t, tree.has(1))

	o.failIAM = true
	_, err = loadGroups(context.Background(), iam, testClientID)
	require.ErrorContains(t, err, "read groups")
}
