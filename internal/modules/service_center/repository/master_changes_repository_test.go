package repository

import (
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"github.com/stretchr/testify/require"
)

func TestSameIDsIgnoresOrderButDetectsChanges(t *testing.T) {
	require.True(t, sameIDs([]int64{1, 2}, []int64{2, 1}))
	require.False(t, sameIDs([]int64{1, 2}, []int64{1, 3}))
}

func TestIsGroupRemoval(t *testing.T) {
	for _, action := range []string{"deleted", "deactivated", "archived"} {
		require.True(t, isGroupRemoval(dto.MasterChangesRequest{Type: dto.MasterChangeTypeGroup, Action: action}))
	}
	require.False(t, isGroupRemoval(dto.MasterChangesRequest{Type: dto.MasterChangeTypeGroup, Action: "moved"}))
	require.False(t, isGroupRemoval(dto.MasterChangesRequest{Type: dto.MasterChangeTypeCategory, Action: "deleted"}))
}
