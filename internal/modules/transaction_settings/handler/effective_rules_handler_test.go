package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

func effectiveApp(repo *fakeRepo, id IdentityReader, assets *fakeAssets) *fiber.App {
	svc := service.NewService(repo, nil)
	if assets != nil {
		svc.WithAssetDirectory(assets)
	}
	app := fiber.New()
	app.Post("/effective", NewHandler(svc, fullPlan).WithIdentity(id).ResolveEffectiveRules)
	return app
}

func TestEffectiveHandler_Success(t *testing.T) {
	assets := &fakeAssets{}
	body := `{"transaction":"borrow","assetIds":["10","20","10"]}`
	code, resp := send(t, effectiveApp(&fakeRepo{}, okIdentity, assets), http.MethodPost, "/effective", body)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, []int64{10, 20}, assets.ids)
	require.Contains(t, resp, `"assetId":"10"`)
	require.Contains(t, resp, `"settingsVersion":1`)
}

func TestEffectiveHandler_Unauthorized(t *testing.T) {
	id := func(fiber.Ctx) (Identity, error) { return Identity{}, nil }
	code, _ := send(t, effectiveApp(&fakeRepo{}, id, &fakeAssets{}), http.MethodPost, "/effective", `{}`)
	require.Equal(t, http.StatusUnauthorized, code)
}

func TestEffectiveHandler_BadRequests(t *testing.T) {
	app := effectiveApp(&fakeRepo{}, okIdentity, &fakeAssets{})
	code, _ := send(t, app, http.MethodPost, "/effective", `{"assetIds": 3}`)
	require.Equal(t, http.StatusBadRequest, code)
	for _, body := range []string{
		`{"transaction":"borrow","assetIds":[]}`,
		`{"transaction":"nope","assetIds":["1"]}`,
		`{"transaction":"borrow","assetIds":["abc"]}`,
		`{"transaction":"borrow","assetIds":["0"]}`,
		`{"transaction":"borrow","assetIds":["01"]}`,
	} {
		code, _ = send(t, app, http.MethodPost, "/effective", body)
		require.Equal(t, http.StatusUnprocessableEntity, code, body)
	}
}

// groupedAssets answers every asset inside Group 9.
type groupedAssets struct{}

func (groupedAssets) ResolveAssets(_ context.Context, _ string, ids []int64) ([]service.AssetContext, error) {
	out := make([]service.AssetContext, 0, len(ids))
	for _, id := range ids {
		out = append(out, service.AssetContext{ID: id, Groups: []service.GroupRef{{ID: 9}}})
	}
	return out, nil
}

// TestEffectiveHandler_GroupScope passes the caller's Group scope to the service: an asset outside the
// view Groups is 404 for the whole request.
func TestEffectiveHandler_GroupScope(t *testing.T) {
	svc := service.NewService(&fakeRepo{}, nil).WithAssetDirectory(groupedAssets{})
	body := `{"transaction":"borrow","assetIds":["1"]}`
	outside := func(fiber.Ctx) (Identity, error) {
		return Identity{ClientID: "c1", UserID: 7, Access: service.CallerAccess{ViewGroupIDs: []int64{2}}}, nil
	}
	inside := func(fiber.Ctx) (Identity, error) {
		return Identity{ClientID: "c1", UserID: 7, Access: service.CallerAccess{ViewGroupIDs: []int64{9}}}, nil
	}
	app := fiber.New()
	app.Post("/effective", NewHandler(svc, fullPlan).WithIdentity(outside).ResolveEffectiveRules)
	code, _ := send(t, app, http.MethodPost, "/effective", body)
	require.Equal(t, http.StatusNotFound, code)
	app = fiber.New()
	app.Post("/effective", NewHandler(svc, fullPlan).WithIdentity(inside).ResolveEffectiveRules)
	code, _ = send(t, app, http.MethodPost, "/effective", body)
	require.Equal(t, http.StatusOK, code)
}

func TestEffectiveHandler_ProviderErrors(t *testing.T) {
	body := `{"transaction":"borrow","assetIds":["1"]}`
	cases := map[string]struct {
		app  *fiber.App
		want int
	}{
		"not found":   {effectiveApp(&fakeRepo{}, okIdentity, &fakeAssets{err: service.ErrAssetNotFound}), http.StatusNotFound},
		"unwired":     {effectiveApp(&fakeRepo{}, okIdentity, nil), http.StatusInternalServerError},
		"repo failed": {effectiveApp(&fakeRepo{fail: true}, okIdentity, &fakeAssets{}), http.StatusInternalServerError},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			code, _ := send(t, tc.app, http.MethodPost, "/effective", body)
			require.Equal(t, tc.want, code)
		})
	}
}

func TestEffectiveHandler_PlanFailure(t *testing.T) {
	app := fiber.New()
	svc := service.NewService(&fakeRepo{}, nil).WithAssetDirectory(&fakeAssets{})
	app.Post("/effective", NewHandler(svc, failingPlan).WithIdentity(okIdentity).ResolveEffectiveRules)
	code, resp := send(t, app, http.MethodPost, "/effective", `{"transaction":"borrow","assetIds":["1"]}`)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, resp, "Internal server error")
}
