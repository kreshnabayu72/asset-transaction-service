package damaged

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/query_builder"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testCallerID is the X-User-ID every route test request carries.
const testCallerID int64 = 7

// routeRepo answers every read with fixed rows.
type routeRepo struct {
	count  int
	asset  *query_builder.AssetScopeRow
	record *query_builder.RunningRecordRow
}

func (r routeRepo) CountBadge(context.Context, string, bool, []int64) (int, error) {
	return r.count, nil
}

func (r routeRepo) FindAssetScope(context.Context, string, int64) (*query_builder.AssetScopeRow, error) {
	return r.asset, nil
}

func (r routeRepo) FindRunningRecord(context.Context, string, int64) (*query_builder.RunningRecordRow, error) {
	return r.record, nil
}

// routesApp mounts the routes. Roles are placed on the request only when given, standing in for
// tsMiddleware.LoadUserRoles.
func routesApp(repo service.Repository, roles *middleware.UserAccessCache) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if roles != nil {
			c.Locals(middleware.UserRolesKey, *roles)
		}
		return c.Next()
	})
	SetupRoutes(app, handler.NewHandler(service.NewService(repo)))
	return app
}

func get(t *testing.T, app *fiber.App, path string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Client-ID", "client01")
	req.Header.Set("X-User-ID", strconv.FormatInt(testCallerID, 10))
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func rolesPtr(flags middleware.ModuleFlags, grants map[enum.SystemRoleKey]middleware.Permissions[[]string]) *middleware.UserAccessCache {
	roles := middleware.UserAccessCache{
		ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{enum.ModuleAccessFixedAsset: flags},
	}
	roles.FixedAssetSystemRole.GroupRoles = grants
	return &roles
}

const (
	summaryPath       = "/v2/asset-transaction/damaged/summary"
	runningRecordPath = "/v2/asset-transaction/damaged/assets/1924881519926425600/running-record"
)

func TestSummaryRequiresRolesSnapshot(t *testing.T) {
	app := routesApp(routeRepo{}, nil)

	status, body := get(t, app, summaryPath)

	assert.Equal(t, http.StatusUnauthorized, status)
	assert.JSONEq(t, `{"message":"Unauthorized"}`, body)
}

func TestSummaryRequiresDamagedViewScope(t *testing.T) {
	cases := []struct {
		name   string
		grants map[enum.SystemRoleKey]middleware.Permissions[[]string]
		want   int
	}{
		{"manage asset and tag read", map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
		}, http.StatusOK},
		{"manage asset and tag update", map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"10"}},
		}, http.StatusOK},
		{"view asset value", map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyViewAssetValue: {View: []string{"10"}},
		}, http.StatusOK},
		{"manage tag only", map[enum.SystemRoleKey]middleware.Permissions[[]string]{
			enum.SystemRoleKeyManageTAG: {View: []string{"10"}},
		}, http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			roles := rolesPtr(middleware.ModuleFlags{}, tc.grants)

			status, _ := get(t, routesApp(routeRepo{}, roles), summaryPath)

			assert.Equal(t, tc.want, status)
		})
	}
}

func TestSummaryReturnsBadgeCount(t *testing.T) {
	roles := rolesPtr(middleware.ModuleFlags{IsTotalControl: true}, nil)
	app := routesApp(routeRepo{count: 7}, roles)

	status, body := get(t, app, summaryPath)

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"message":"Success","data":{"badgeCount":7}}`, body)
}

func TestSummaryReturnsNullBadgeWithoutUpdateGroups(t *testing.T) {
	roles := rolesPtr(middleware.ModuleFlags{IsReadOnly: true}, nil)
	app := routesApp(routeRepo{count: 7}, roles)

	status, body := get(t, app, summaryPath)

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"message":"Success","data":{"badgeCount":null}}`, body)
}

func TestRunningRecordUnknownAssetIsNotFound(t *testing.T) {
	roles := rolesPtr(middleware.ModuleFlags{IsTotalControl: true}, nil)
	app := routesApp(routeRepo{}, roles)

	status, body := get(t, app, runningRecordPath)

	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"message":"Asset not found"}`, body)
}

func TestRunningRecordNonNumericIDIsNotFound(t *testing.T) {
	roles := rolesPtr(middleware.ModuleFlags{IsTotalControl: true}, nil)
	app := routesApp(routeRepo{asset: &query_builder.AssetScopeRow{ID: 1}}, roles)

	status, body := get(t, app, "/v2/asset-transaction/damaged/assets/12a/running-record")

	assert.Equal(t, http.StatusNotFound, status)
	assert.JSONEq(t, `{"message":"Asset not found"}`, body)
}

func TestRunningRecordOutsideScopeIsForbiddenWithReason(t *testing.T) {
	group30 := int64(30)
	otherHolder := int64(99)
	roles := rolesPtr(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
	})
	app := routesApp(routeRepo{asset: &query_builder.AssetScopeRow{ID: 1, GroupID: &group30, HolderID: &otherHolder}}, roles)

	status, body := get(t, app, runningRecordPath)

	assert.Equal(t, http.StatusForbidden, status)
	assert.JSONEq(t, `{"message":"Forbidden","data":{"reason":"group_scope"}}`, body)
}

func TestRunningRecordWithoutRecordReturnsNullData(t *testing.T) {
	group10 := int64(10)
	roles := rolesPtr(middleware.ModuleFlags{}, map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"10"}},
	})
	app := routesApp(routeRepo{asset: &query_builder.AssetScopeRow{ID: 1, GroupID: &group10}}, roles)

	status, body := get(t, app, runningRecordPath)

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"message":"Success","data":null}`, body)
}

func TestRunningRecordReturnsEnvelope(t *testing.T) {
	group10 := int64(10)
	missing := "Missing"
	roles := rolesPtr(middleware.ModuleFlags{IsReadOnly: true}, nil)
	app := routesApp(routeRepo{
		asset: &query_builder.AssetScopeRow{ID: 1, GroupID: &group10},
		record: &query_builder.RunningRecordRow{
			ID:            1924881519926425901,
			Status:        "Damaged",
			MissingMarker: &missing,
			ReportedAt:    time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC),
		},
	}, roles)

	status, body := get(t, app, runningRecordPath)

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"message":"Success","data":{"_id":"1924881519926425901","status":"Damaged","missingMarker":"Missing","reportedAt":"2026-11-10T01:15:00Z"}}`, body)
}

func TestRunningRecordHolderIsAllowedOutsideGroupScope(t *testing.T) {
	group30 := int64(30)
	holder := testCallerID
	missing := "Missing"
	roles := rolesPtr(middleware.ModuleFlags{}, nil)
	app := routesApp(routeRepo{
		asset: &query_builder.AssetScopeRow{ID: 1, GroupID: &group30, HolderID: &holder},
		record: &query_builder.RunningRecordRow{
			ID:            1924881519926425901,
			Status:        "Damaged",
			MissingMarker: &missing,
			ReportedAt:    time.Date(2026, 11, 10, 1, 15, 0, 0, time.UTC),
		},
	}, roles)

	status, body := get(t, app, runningRecordPath)

	assert.Equal(t, http.StatusOK, status)
	assert.JSONEq(t, `{"message":"Success","data":{"_id":"1924881519926425901","status":"Damaged","missingMarker":"Missing","reportedAt":"2026-11-10T01:15:00Z"}}`, body)
}
