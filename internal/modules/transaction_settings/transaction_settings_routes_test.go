package transaction_settings

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/handler"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

func routesApp(roles middleware.UserAccessCache) *fiber.App {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(middleware.UserRolesKey, roles)
		return c.Next()
	})
	SetupRoutes(app, handler.NewHandler(nil, nil))
	return app
}

func call(t *testing.T, app *fiber.App, method, path string) int {
	t.Helper()
	req := httptest.NewRequest(method, "/v2/asset-transaction/fixed-asset/transaction-settings"+path, strings.NewReader("{"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(constant.HEADER_CLIENT_ID, "c1")
	req.Header.Set(constant.HEADER_USER_ID, "7")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func TestSaveRoute_GatedPerRoute(t *testing.T) {
	totalControl := middleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{enum.ModuleAccessFixedAsset: {IsTotalControl: true}}}
	readOnly := middleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{enum.ModuleAccessFixedAsset: {IsReadOnly: true}}}

	// Total Control passes the gate and reaches the handler, which rejects the malformed body.
	require.Equal(t, http.StatusBadRequest, call(t, routesApp(totalControl), http.MethodPatch, "/"))
	// Read Only reads but cannot save.
	require.Equal(t, http.StatusForbidden, call(t, routesApp(readOnly), http.MethodPatch, "/"))
	require.Equal(t, http.StatusForbidden, call(t, routesApp(middleware.UserAccessCache{}), http.MethodPatch, "/"))
}

func TestEffectiveRulesRoute_NeedsNoSettingsCapability(t *testing.T) {
	// No capability needed: the request reaches the handler, which rejects the malformed body.
	require.Equal(t, http.StatusBadRequest, call(t, routesApp(middleware.UserAccessCache{}), http.MethodPost, "/effective-rules"))
}

func TestOverrideRoutes_Gates(t *testing.T) {
	fa := enum.ModuleAccessFixedAsset
	totalControl := middleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{fa: {IsTotalControl: true}}}
	readOnly := middleware.UserAccessCache{ModuleAccess: map[enum.ModuleAccess]middleware.ModuleFlags{fa: {IsReadOnly: true}}}
	// Read Only with an Update role: the access level wins.
	readOnlyUpdater := middleware.UserAccessCache{ModuleAccess: readOnly.ModuleAccess}
	readOnlyUpdater.FixedAssetSystemRole.GlobalRole = map[enum.SystemRoleKey]middleware.Permissions[bool]{
		enum.SystemRoleKeyManageTransactionSettings: {Edit: true, View: true},
	}
	manageUpdate := middleware.UserAccessCache{}
	manageUpdate.FixedAssetSystemRole.GlobalRole = readOnlyUpdater.FixedAssetSystemRole.GlobalRole
	groupUpdate := middleware.UserAccessCache{}
	groupUpdate.FixedAssetSystemRole.GroupRoles = map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {Edit: []string{"3"}, View: []string{"3"}},
	}
	groupView := middleware.UserAccessCache{}
	groupView.FixedAssetSystemRole.GroupRoles = map[enum.SystemRoleKey]middleware.Permissions[[]string]{
		enum.SystemRoleKeyManageAssetAndTAG: {View: []string{"3"}},
	}

	// The malformed body reaches the handler (400) when the gate passes.
	for _, route := range []struct{ method, path string }{{http.MethodPost, "/overrides"}, {http.MethodDelete, "/overrides"}} {
		for name, tc := range map[string]struct {
			roles middleware.UserAccessCache
			want  int
		}{
			"total control":           {totalControl, http.StatusBadRequest},
			"manage settings update":  {manageUpdate, http.StatusBadRequest},
			"manage asset tag update": {groupUpdate, http.StatusBadRequest},
			"read only":               {readOnly, http.StatusForbidden},
			"read only with update":   {readOnlyUpdater, http.StatusForbidden},
			"manage asset tag read":   {groupView, http.StatusForbidden},
			"no roles":                {middleware.UserAccessCache{}, http.StatusForbidden},
		} {
			require.Equal(t, tc.want, call(t, routesApp(tc.roles), route.method, route.path), route.method+" "+route.path+" "+name)
		}
	}
	for _, path := range []string{"/overrides/list", "/overrides/options"} {
		for name, tc := range map[string]struct {
			roles middleware.UserAccessCache
			want  int
		}{
			"total control":         {totalControl, http.StatusBadRequest},
			"read only":             {readOnly, http.StatusBadRequest},
			"manage settings":       {manageUpdate, http.StatusBadRequest},
			"manage asset tag read": {groupView, http.StatusBadRequest},
			"no roles":              {middleware.UserAccessCache{}, http.StatusForbidden},
		} {
			require.Equal(t, tc.want, call(t, routesApp(tc.roles), http.MethodPost, path), path+" "+name)
		}
	}
}
