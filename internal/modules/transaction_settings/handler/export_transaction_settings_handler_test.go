package handler

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

type stubRenderer struct{ sheets []dto.Sheet }

func (s *stubRenderer) Render(sheets []dto.Sheet) ([]byte, error) {
	s.sheets = sheets
	return []byte("xlsx-bytes"), nil
}

func exportApp(renderer service.WorkbookRenderer) *fiber.App {
	app := fiber.New()
	app.Post("/export", NewHandler(service.NewService(&fakeRepo{}, renderer), fullPlan).WithIdentity(okIdentity).ExportTransactionSettings)
	return app
}

func postExport(t *testing.T, app *fiber.App, body string) (*http.Response, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	return resp, string(raw)
}

func TestExportTransactionSettingsHandler_Success(t *testing.T) {
	renderer := &stubRenderer{}
	resp, body := postExport(t, exportApp(renderer), `{"categoryId":null}`)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
	require.Equal(t, "attachment; filename*=UTF-8''Transaction-Rules.xlsx", resp.Header.Get("Content-Disposition"))
	require.Equal(t, "xlsx-bytes", body)
	require.Len(t, renderer.sheets, 3)
}

func TestExportTransactionSettingsHandler_MalformedBody(t *testing.T) {
	resp, body := postExport(t, exportApp(&stubRenderer{}), `{"categoryId":`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, body, "Invalid request body")
}

func TestExportTransactionSettingsHandler_MalformedCategoryID(t *testing.T) {
	resp, body := postExport(t, exportApp(&stubRenderer{}), `{"categoryId":"x1"}`)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, body, "Invalid request body")
}

func TestExportTransactionSettingsHandler_RendererUnavailable(t *testing.T) {
	resp, body := postExport(t, exportApp(nil), `{"categoryId":null}`)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "Internal server error")
}

func TestExportTransactionSettingsHandler_Unauthorized(t *testing.T) {
	noSession := func(fiber.Ctx) (Identity, error) { return Identity{}, errors.New("no session") }
	app := fiber.New()
	app.Post("/export", NewHandler(service.NewService(&fakeRepo{}, &stubRenderer{}), fullPlan).WithIdentity(noSession).ExportTransactionSettings)
	resp, body := postExport(t, app, `{"categoryId":null}`)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.Contains(t, body, "Unauthorized")
}

func TestExportTransactionSettingsHandler_PlanFailure(t *testing.T) {
	app := fiber.New()
	app.Post("/export", NewHandler(service.NewService(&fakeRepo{}, &stubRenderer{}), failingPlan).WithIdentity(okIdentity).ExportTransactionSettings)
	resp, body := postExport(t, app, `{"categoryId":null}`)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, body, "Internal server error")
}

func TestExportTransactionSettingsHandler_CategoryNotFound(t *testing.T) {
	svc := service.NewService(&fakeRepo{}, &stubRenderer{}).WithCategoryDirectory(fakeCategories{})
	app := fiber.New()
	app.Post("/export", NewHandler(svc, fullPlan).WithIdentity(okIdentity).ExportTransactionSettings)
	resp, body := postExport(t, app, `{"categoryId":"42"}`)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Contains(t, body, "Not found")
}
