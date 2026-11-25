package query_builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCheckAvailabilityQueryWithoutExclusion(t *testing.T) {
	query, args, err := BuildCheckAvailabilityQuery(" Workshop ", nil)

	require.NoError(t, err)
	require.Contains(t, query, "COUNT(*) = 0 AS available")
	require.Contains(t, query, "name_key = LOWER(BTRIM($1))")
	require.NotContains(t, strings.ToLower(query), "is_active")
	require.Equal(t, []any{" Workshop "}, args)
}

func TestBuildCheckAvailabilityQueryExcludesMatchingID(t *testing.T) {
	excludeID := int64(14)
	query, args, err := BuildCheckAvailabilityQuery("Workshop", &excludeID)

	require.NoError(t, err)
	require.Contains(t, query, "id")
	require.Contains(t, query, "!= $2")
	require.Equal(t, []any{"Workshop", excludeID}, args)
}
