package query_builder

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oneLine collapses the builder's line breaks so the assertions compare the statement text.
func oneLine(query string) string {
	return strings.Join(strings.Fields(query), " ")
}

func TestBuildBadgeCountQueryTotalControlSkipsGroupPredicate(t *testing.T) {
	query, args, err := BuildBadgeCountQuery(true, nil)
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT COUNT(*) FROM damage_reports dr JOIN assets a ON a.id = dr.asset_id WHERE "dr"."status" IN ($1, $2) AND "dr"."missing_marker" IS NULL`,
		oneLine(query))
	assert.Equal(t, []any{"waiting_for_confirmation", "damaged"}, args)
}

func TestBuildBadgeCountQueryFiltersByActorGroups(t *testing.T) {
	query, args, err := BuildBadgeCountQuery(false, []int64{10, 20})
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT COUNT(*) FROM damage_reports dr JOIN assets a ON a.id = dr.asset_id WHERE "dr"."status" IN ($1, $2) AND "dr"."missing_marker" IS NULL AND "a"."group_id" IN ($3, $4)`,
		oneLine(query))
	assert.Equal(t, []any{"waiting_for_confirmation", "damaged", int64(10), int64(20)}, args)
}

func TestBuildBadgeCountQueryRejectsEmptyGroupList(t *testing.T) {
	_, _, err := BuildBadgeCountQuery(false, nil)
	require.Error(t, err)
}

func TestBuildAssetScopeQuery(t *testing.T) {
	query, args, err := BuildAssetScopeQuery(1924881519926425600)
	require.NoError(t, err)
	assert.Equal(t, `SELECT id,group_id,holder_id FROM assets WHERE "id" = $1`, oneLine(query))
	assert.Equal(t, []any{int64(1924881519926425600)}, args)
}

func TestBuildRunningRecordQueryFormatsWireLabels(t *testing.T) {
	query, args, err := BuildRunningRecordQuery(1924881519926425600)
	require.NoError(t, err)
	assert.Equal(t,
		`SELECT id,reported_at,CASE WHEN status = 'waiting_for_confirmation' THEN 'Waiting for Confirmation' WHEN status = 'damaged' THEN 'Damaged' WHEN status = 'reported_disposal' THEN 'Reported Disposal' WHEN status = 'in_disposal_process' THEN 'In Disposal Process' ELSE status END AS "status",CASE WHEN missing_marker = 'reported_missing' THEN 'Reported Missing' WHEN missing_marker = 'missing' THEN 'Missing' ELSE NULL END AS "missing_marker" FROM damage_reports WHERE "asset_id" = $1 AND "status" IN ($2, $3, $4, $5)`,
		oneLine(query))
	assert.Equal(t, []any{int64(1924881519926425600), "waiting_for_confirmation", "damaged", "reported_disposal", "in_disposal_process"}, args)
}
