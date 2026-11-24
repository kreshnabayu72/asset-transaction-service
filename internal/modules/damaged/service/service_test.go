package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/query_builder"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeRepo is an in-memory Repository that records how each query was called.
type fakeRepo struct {
	count        int
	countErr     error
	countCalls   int
	gotAllGroups bool
	gotGroupIDs  []int64

	asset       *query_builder.AssetScopeRow
	assetErr    error
	record      *query_builder.RunningRecordRow
	recordCalls int
}

func (f *fakeRepo) CountBadge(_ context.Context, _ string, allGroups bool, groupIDs []int64) (int, error) {
	f.countCalls++
	f.gotAllGroups = allGroups
	f.gotGroupIDs = groupIDs
	return f.count, f.countErr
}

func (f *fakeRepo) FindAssetScope(_ context.Context, _ string, _ int64) (*query_builder.AssetScopeRow, error) {
	return f.asset, f.assetErr
}

func (f *fakeRepo) FindRunningRecord(_ context.Context, _ string, _ int64) (*query_builder.RunningRecordRow, error) {
	f.recordCalls++
	return f.record, nil
}

// rolesWith builds a snapshot with the given module flags and per-key group grants.
// testCallerID is the user the running-record tests act as.
const testCallerID int64 = 7

func rolesWith(flags middleware.ModuleFlags, grants map[enum.SystemRoleKey]middleware.Permissions[[]string]) middleware.UserAccessCache {
	roles := middleware.UserAccessCache{
		ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{enum.ModuleAccessFixedAsset: flags},
	}
	roles.FixedAssetSystemRole.GroupRoles = grants
	return roles
}

// rolesWithGroupManagers is rolesWith plus the caller's Group Manager group ids.
func rolesWithGroupManagers(grants map[enum.SystemRoleKey]middleware.Permissions[[]string], ids []string) middleware.UserAccessCache {
	roles := rolesWith(middleware.ModuleFlags{}, grants)
	roles.GroupManagerGroupIDs = ids
	return roles
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestSummaryTotalControlCountsEveryGroup(t *testing.T) {
	repo := &fakeRepo{count: 7}
	roles := rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil)

	data, err := NewService(repo).Summary(context.Background(), "client01", roles)

	require.NoError(t, err)
	require.NotNil(t, data.BadgeCount)
	assert.Equal(t, 7, *data.BadgeCount)
	assert.True(t, repo.gotAllGroups)
	assert.Nil(t, repo.gotGroupIDs)
}

func TestSummaryCountsOnlyUpdateGroups(t *testing.T) {
	repo := &fakeRepo{count: 3}
	roles := rolesWith(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"10", "20"}, View: []string{"30"}},
	})

	data, err := NewService(repo).Summary(context.Background(), "client01", roles)

	require.NoError(t, err)
	require.NotNil(t, data.BadgeCount)
	assert.Equal(t, 3, *data.BadgeCount)
	assert.False(t, repo.gotAllGroups)
	assert.Equal(t, []int64{10, 20}, repo.gotGroupIDs)
}

func TestSummaryCountsGroupManagerGroups(t *testing.T) {
	repo := &fakeRepo{count: 4}
	roles := rolesWithGroupManagers(map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"10"}, View: []string{"30"}},
	}, []string{"20"})

	data, err := NewService(repo).Summary(context.Background(), "client01", roles)

	require.NoError(t, err)
	require.NotNil(t, data.BadgeCount)
	assert.Equal(t, 4, *data.BadgeCount)
	assert.False(t, repo.gotAllGroups)
	assert.Equal(t, []int64{10, 20}, repo.gotGroupIDs)
}

func TestSummaryIsNullWithoutUpdateGroups(t *testing.T) {
	cases := map[string]middleware.UserAccessCache{
		"read only": rolesWith(middleware.ModuleFlags{IsReadOnly: true}, nil),
		"view asset value only": rolesWith(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyViewAssetValue: {View: []string{"10"}},
		}),
		"manage asset and tag read only": rolesWith(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
		}),
		"nil group manager ids":   rolesWithGroupManagers(nil, nil),
		"empty group manager ids": rolesWithGroupManagers(nil, []string{}),
	}
	for name, roles := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{count: 9}

			data, err := NewService(repo).Summary(context.Background(), "client01", roles)

			require.NoError(t, err)
			assert.Nil(t, data.BadgeCount)
			assert.Equal(t, 0, repo.countCalls)
		})
	}
}

func TestSummaryReturnsCountError(t *testing.T) {
	repo := &fakeRepo{countErr: errors.New("db down")}
	roles := rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil)

	_, err := NewService(repo).Summary(context.Background(), "client01", roles)

	require.Error(t, err)
	assert.ErrorContains(t, err, "db down")
}

func TestRunningRecordUnknownAssetIsNotFound(t *testing.T) {
	repo := &fakeRepo{}
	roles := rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil)

	_, err := NewService(repo).RunningRecord(context.Background(), "client01", roles, testCallerID, 1)

	assert.ErrorIs(t, err, ErrAssetNotFound)
	assert.Equal(t, 0, repo.recordCalls)
}

func TestRunningRecordScopeRules(t *testing.T) {
	viewGroup10 := map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
	}
	editGroup10 := map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"10"}},
	}
	valueGroup10 := map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyViewAssetValue: {View: []string{"10"}},
	}
	cases := []struct {
		name       string
		roles      middleware.UserAccessCache
		assetGroup *int64
		wantErr    error
	}{
		{"manage asset and tag read grants its group", rolesWith(middleware.ModuleFlags{}, viewGroup10), int64Ptr(10), nil},
		{"manage asset and tag update grants its group", rolesWith(middleware.ModuleFlags{}, editGroup10), int64Ptr(10), nil},
		{"view asset value grants its group", rolesWith(middleware.ModuleFlags{}, valueGroup10), int64Ptr(10), nil},
		{"other group is outside scope", rolesWith(middleware.ModuleFlags{}, viewGroup10), int64Ptr(30), ErrGroupScope},
		{"asset without group needs all-group access", rolesWith(middleware.ModuleFlags{}, viewGroup10), nil, ErrGroupScope},
		{"total control sees every group", rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil), int64Ptr(99), nil},
		{"read only sees every group", rolesWith(middleware.ModuleFlags{IsReadOnly: true}, nil), int64Ptr(99), nil},
		{"total control sees asset without group", rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil), nil, nil},
		{"no grants are outside scope", rolesWith(middleware.ModuleFlags{}, nil), int64Ptr(10), ErrGroupScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{
				asset:  &query_builder.AssetScopeRow{ID: 1, GroupID: tc.assetGroup},
				record: &query_builder.RunningRecordRow{ID: 2, Status: "Damaged"},
			}

			data, err := NewService(repo).RunningRecord(context.Background(), "client01", tc.roles, testCallerID, 1)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, 0, repo.recordCalls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, data)
			assert.Equal(t, int64(2), data.ID)
		})
	}
}

func TestRunningRecordGroupManagerScope(t *testing.T) {
	viewGroup10 := map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
	}
	cases := []struct {
		name       string
		roles      middleware.UserAccessCache
		assetGroup *int64
		wantErr    error
	}{
		{"group manager sees its group without a manage grant", rolesWithGroupManagers(nil, []string{"10"}), int64Ptr(10), nil},
		{"group manager is outside its groups", rolesWithGroupManagers(nil, []string{"10"}), int64Ptr(30), ErrGroupScope},
		{"nil group manager ids keep view grants", rolesWithGroupManagers(viewGroup10, nil), int64Ptr(10), nil},
		{"empty group manager ids keep view grants", rolesWithGroupManagers(viewGroup10, []string{}), int64Ptr(10), nil},
		{"nil group manager ids add no access", rolesWithGroupManagers(nil, nil), int64Ptr(10), ErrGroupScope},
		{"empty group manager ids add no access", rolesWithGroupManagers(nil, []string{}), int64Ptr(10), ErrGroupScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{
				asset:  &query_builder.AssetScopeRow{ID: 1, GroupID: tc.assetGroup},
				record: &query_builder.RunningRecordRow{ID: 2, Status: "Damaged"},
			}

			data, err := NewService(repo).RunningRecord(context.Background(), "client01", tc.roles, testCallerID, 1)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, 0, repo.recordCalls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, data)
			assert.Equal(t, int64(2), data.ID)
		})
	}
}

func TestRunningRecordWithoutRecordIsNil(t *testing.T) {
	repo := &fakeRepo{asset: &query_builder.AssetScopeRow{ID: 1, GroupID: int64Ptr(10)}}
	roles := rolesWith(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
	})

	data, err := NewService(repo).RunningRecord(context.Background(), "client01", roles, testCallerID, 1)

	require.NoError(t, err)
	assert.Nil(t, data)
	assert.Equal(t, 1, repo.recordCalls)
}

func TestRunningRecordMapsWireFields(t *testing.T) {
	reportedAt := time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC)
	missing := "Reported Missing"
	repo := &fakeRepo{
		asset: &query_builder.AssetScopeRow{ID: 1, GroupID: int64Ptr(10)},
		record: &query_builder.RunningRecordRow{
			ID:            1924881519926425901,
			Status:        "Waiting for Confirmation",
			MissingMarker: &missing,
			ReportedAt:    reportedAt,
		},
	}
	roles := rolesWith(middleware.ModuleFlags{IsTotalControl: true}, nil)

	data, err := NewService(repo).RunningRecord(context.Background(), "client01", roles, testCallerID, 1)

	require.NoError(t, err)
	require.NotNil(t, data)
	assert.Equal(t, int64(1924881519926425901), data.ID)
	assert.Equal(t, "Waiting for Confirmation", data.Status)
	require.NotNil(t, data.MissingMarker)
	assert.Equal(t, "Reported Missing", *data.MissingMarker)
	assert.Equal(t, reportedAt, data.ReportedAt)
}

func TestRunningRecordHolderAccess(t *testing.T) {
	cases := []struct {
		name        string
		assetGroup  *int64
		assetHolder *int64
		wantErr     error
	}{
		{"holder outside every group grant is allowed", int64Ptr(30), int64Ptr(testCallerID), nil},
		{"holder of an asset without group is allowed", nil, int64Ptr(testCallerID), nil},
		{"other holder outside group is outside scope", int64Ptr(30), int64Ptr(99), ErrGroupScope},
		{"unheld asset outside group is outside scope", int64Ptr(30), nil, ErrGroupScope},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{
				asset:  &query_builder.AssetScopeRow{ID: 1, GroupID: tc.assetGroup, HolderID: tc.assetHolder},
				record: &query_builder.RunningRecordRow{ID: 2, Status: "Damaged"},
			}
			roles := rolesWith(middleware.ModuleFlags{}, nil)

			data, err := NewService(repo).RunningRecord(context.Background(), "client01", roles, testCallerID, 1)

			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
				assert.Equal(t, 0, repo.recordCalls)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, data)
			assert.Equal(t, int64(2), data.ID)
		})
	}
}

func TestRunningRecordUnknownAssetBeatsScopeDenial(t *testing.T) {
	repo := &fakeRepo{}
	roles := rolesWith(middleware.ModuleFlags{}, nil)

	_, err := NewService(repo).RunningRecord(context.Background(), "client01", roles, testCallerID, 1)

	assert.ErrorIs(t, err, ErrAssetNotFound)
	assert.Equal(t, 0, repo.recordCalls)
}
