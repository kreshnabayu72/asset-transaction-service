package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

func readApp() *fiber.App {
	app := fiber.New()
	app.Get("/settings", NewHandler(service.NewService(&fakeRepo{}, nil), fullPlan).ReadTransactionSettings)
	return app
}

func TestReadTransactionSettingsHandler_AllCategories(t *testing.T) {
	resp, err := readApp().Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Message string `json:"message"`
		Data    struct {
			SettingsVersion int             `json:"settingsVersion"`
			CategoryID      *string         `json:"categoryId"`
			Transactions    json.RawMessage `json:"transactions"`
		} `json:"data"`
	}
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &body))
	require.Equal(t, "Success", body.Message)
	require.Equal(t, 1, body.Data.SettingsVersion)
	require.Nil(t, body.Data.CategoryID)
	require.Contains(t, string(body.Data.Transactions), `"damaged_missing"`)
}

func TestReadTransactionSettingsHandler_Leaf(t *testing.T) {
	resp, err := readApp().Test(httptest.NewRequest(http.MethodGet, "/settings?categoryId=42", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"categoryId":"42"`)
	require.NotContains(t, string(raw), `"damaged_missing"`)
}

func TestReadTransactionSettingsHandler_MalformedCategoryID(t *testing.T) {
	resp, err := readApp().Test(httptest.NewRequest(http.MethodGet, "/settings?categoryId=abc", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(raw), "Invalid request body")
}

func TestReadTransactionSettingsHandler_RepositoryFailure(t *testing.T) {
	app := fiber.New()
	app.Get("/settings", NewHandler(service.NewService(&fakeRepo{fail: true}, nil), fullPlan).ReadTransactionSettings)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestReadTransactionSettingsHandler_CategoryNotFound(t *testing.T) {
	svc := service.NewService(&fakeRepo{}, nil).WithCategoryDirectory(fakeCategories{})
	app := fiber.New()
	app.Get("/settings", NewHandler(svc, fullPlan).ReadTransactionSettings)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings?categoryId=42", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestReadTransactionSettingsHandler_PlanFailure(t *testing.T) {
	app := fiber.New()
	app.Get("/settings", NewHandler(service.NewService(&fakeRepo{}, nil), failingPlan).ReadTransactionSettings)
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings", nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
