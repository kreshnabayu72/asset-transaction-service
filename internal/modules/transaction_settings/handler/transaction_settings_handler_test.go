package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
)

func strPtr(s string) *string { return &s }

func fullPlan(fiber.Ctx) (service.Plan, error) { return service.FullPlan, nil }

// failingPlan is a plan provider that fails; every handler that reads the plan answers it as 500.
func failingPlan(fiber.Ctx) (service.Plan, error) {
	return service.Plan{}, errors.New("plan provider down")
}

func TestNewHandler(t *testing.T) {
	svc := service.NewService(&fakeRepo{}, nil)
	h := NewHandler(svc, fullPlan)
	require.Same(t, svc, h.svc)
	require.NotNil(t, h.plan)
}

func TestParseCategoryID(t *testing.T) {
	id, err := parseCategoryID(nil)
	require.NoError(t, err)
	require.Nil(t, id)

	id, err = parseCategoryID(strPtr(""))
	require.NoError(t, err)
	require.Nil(t, id)

	id, err = parseCategoryID(strPtr("1924881519926425600"))
	require.NoError(t, err)
	require.Equal(t, int64(1924881519926425600), *id)

	for _, bad := range []string{"abc", "-1", "0", "99999999999999999999", "1a"} {
		_, err = parseCategoryID(strPtr(bad))
		require.ErrorIs(t, err, errMalformedCategoryID, bad)
	}
}

func TestPlanFromProvider(t *testing.T) {
	var seen string
	read := func(_ context.Context, clientID string) (service.Plan, error) {
		seen = clientID
		return service.FullPlan, nil
	}
	app := fiber.New()
	app.Get("/plan", func(c fiber.Ctx) error {
		plan, err := PlanFromProvider(read)(c)
		require.NoError(t, err)
		require.Equal(t, service.FullPlan, plan)
		return c.SendStatus(fiber.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/plan", nil)
	req.Header.Set(constant.HEADER_CLIENT_ID, "c1")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "c1", seen)
}
