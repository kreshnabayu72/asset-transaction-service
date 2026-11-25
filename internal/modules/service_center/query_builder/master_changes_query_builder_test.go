package query_builder

import (
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"github.com/stretchr/testify/require"
)

func TestBuildTargetCentersQueryIsDistinctAndOrdered(t *testing.T) {
	query, args, err := BuildTargetCentersQuery(dto.MasterChangeTypeGroup, []int64{20, 10})

	require.NoError(t, err)
	require.Contains(t, query, "DISTINCT service_center_id")
	require.Contains(t, query, `"group_id" IN ($1, $2)`)
	require.Contains(t, query, "ORDER BY service_center_id ASC")
	require.Equal(t, []any{int64(20), int64(10)}, args)
}

func TestBuildLockServiceCenterQueryUsesForUpdate(t *testing.T) {
	query, args, err := BuildLockServiceCenterQuery(42)

	require.NoError(t, err)
	require.Contains(t, query, "FOR UPDATE")
	require.Contains(t, query, `WHERE "id" = $1`)
	require.Equal(t, []any{int64(42)}, args)
}

func TestBuildCurrentLinksQueryUsesTheSelectedMasterTable(t *testing.T) {
	query, args, err := BuildCurrentLinksQuery(dto.MasterChangeTypeBrand, 52)

	require.NoError(t, err)
	require.Contains(t, query, "FROM service_center_brands")
	require.Contains(t, query, "brand_id AS id")
	require.Contains(t, query, `WHERE "service_center_id" = $1`)
	require.Equal(t, []any{int64(52)}, args)
}

func TestBuildDeleteLinksQueryUsesBothPredicates(t *testing.T) {
	query, args, err := BuildDeleteLinksQuery(dto.MasterChangeTypeCategory, 42, []int64{7, 8})

	require.NoError(t, err)
	require.Contains(t, query, "DELETE FROM service_center_categories")
	require.Contains(t, query, `"service_center_id" = $1`)
	require.Contains(t, query, `"category_id" IN ($2, $3)`)
	require.Equal(t, []any{int64(42), int64(7), int64(8)}, args)
}

func TestBuildServiceCenterUpdateAdvancesVersionAndCanDeactivate(t *testing.T) {
	query, args, err := BuildServiceCenterUpdateQuery(42, true)

	require.NoError(t, err)
	require.Contains(t, query, `"version" = version + 1`)
	require.Contains(t, query, `"is_active" = `)
	require.Contains(t, query, `"modified_by_id" = `)
	require.Contains(t, query, `"modified_by_name" = `)
	require.Contains(t, strings.ToLower(query), "updated_at")
	require.Len(t, args, 4)
	require.Contains(t, args, false)
	require.Contains(t, args, nil)
	require.Contains(t, args, "System")
	require.Contains(t, args, int64(42))
}

func TestBuildServiceCenterUpdateKeepsExistingStatusWhenNoDeactivationIsNeeded(t *testing.T) {
	query, _, err := BuildServiceCenterUpdateQuery(42, false)

	require.NoError(t, err)
	require.NotContains(t, query, `"is_active" =`)
}

func TestBuildCurrentLinksQueryRejectsUnknownType(t *testing.T) {
	_, _, err := BuildCurrentLinksQuery("unsupported", 42)

	require.Error(t, err)
}
