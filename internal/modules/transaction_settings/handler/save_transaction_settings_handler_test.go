package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

type fakeCategories struct{ info service.CategoryInfo }

func (f fakeCategories) LookupCategory(context.Context, string, int64) (service.CategoryInfo, error) {
	return f.info, nil
}

func (f fakeCategories) ListLeafCategories(context.Context, string) ([]service.LeafCategory, error) {
	return nil, nil
}

type fakeAssets struct {
	ids []int64
	err error
}

func (f *fakeAssets) ResolveAssets(_ context.Context, _ string, ids []int64) ([]service.AssetContext, error) {
	f.ids = ids
	out := make([]service.AssetContext, 0, len(ids))
	for _, id := range ids {
		out = append(out, service.AssetContext{ID: id})
	}
	return out, f.err
}

func okIdentity(fiber.Ctx) (Identity, error) {
	return Identity{ClientID: "c1", UserID: 7, FullName: "Rina Wulandari", Access: service.CallerAccess{TotalControl: true, ViewAll: true}}, nil
}

func send(t *testing.T, app *fiber.App, method, path, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(raw)
}

func saveApp(repo *fakeRepo, id IdentityReader, configure ...func(*service.Service)) *fiber.App {
	svc := service.NewService(repo, nil)
	for _, c := range configure {
		c(svc)
	}
	app := fiber.New()
	app.Patch("/settings", NewHandler(svc, fullPlan).WithIdentity(id).SaveTransactionSettings)
	return app
}

const validSave = `{"changes":[{"categoryId":null,"transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":25}]}]}`

func TestSaveHandler_Saved(t *testing.T) {
	repo := &fakeRepo{}
	code, body := send(t, saveApp(repo, okIdentity), http.MethodPatch, "/settings", validSave)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "Success, settings have been saved.")
	require.Equal(t, 1, repo.tx.rows)
}

func TestSaveHandler_NoChanges(t *testing.T) {
	same := `{"changes":[{"categoryId":null,"transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":14}]}]}`
	code, body := send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", same)
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "No changes to save.")
}

func TestSaveHandler_Unauthorized(t *testing.T) {
	for name, id := range map[string]IdentityReader{
		"error":     func(fiber.Ctx) (Identity, error) { return Identity{}, io.EOF },
		"no user":   func(fiber.Ctx) (Identity, error) { return Identity{ClientID: "c1"}, nil },
		"no tenant": func(fiber.Ctx) (Identity, error) { return Identity{UserID: 1}, nil },
	} {
		t.Run(name, func(t *testing.T) {
			code, _ := send(t, saveApp(&fakeRepo{}, id), http.MethodPatch, "/settings", validSave)
			require.Equal(t, http.StatusUnauthorized, code)
		})
	}
}

func TestSaveHandler_BadBody(t *testing.T) {
	code, body := send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", `{"changes": "x"}`)
	require.Equal(t, http.StatusBadRequest, code)
	require.Contains(t, body, "Invalid request body")
}

func TestSaveHandler_StructValidation(t *testing.T) {
	for _, body := range []string{
		`{"changes":[]}`,
		`{"changes":[{"transaction":"nope","fields":[{"key":"a","mode":"own"}]}]}`,
		`{"changes":[{"transaction":"borrow","fields":[]}]}`,
		`{"changes":[{"transaction":"borrow","fields":[{"key":"max_duration","mode":"maybe"}]}]}`,
	} {
		code, resp := send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", body)
		require.Equal(t, http.StatusUnprocessableEntity, code, body)
		require.Contains(t, resp, "Invalid request body")
		require.Contains(t, resp, `"field"`)
	}
}

func TestSaveHandler_ServiceValidation(t *testing.T) {
	body := `{"changes":[{"categoryId":null,"transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":0}]}]}`
	code, resp := send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", body)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	var parsed struct {
		Message string           `json:"message"`
		Error   []dto.FieldError `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp), &parsed))
	require.Equal(t, "Invalid request body", parsed.Message)
	require.Equal(t, []dto.FieldError{{Field: "fields[0].value", Message: "Value must be at least 1"}}, parsed.Error)
}

func TestSaveHandler_CategoryNotFound(t *testing.T) {
	body := `{"changes":[{"categoryId":"5","transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":3}]}]}`
	app := saveApp(&fakeRepo{}, okIdentity, func(s *service.Service) {
		s.WithCategoryDirectory(fakeCategories{info: service.CategoryInfo{}})
	})
	code, resp := send(t, app, http.MethodPatch, "/settings", body)
	require.Equal(t, http.StatusNotFound, code)
	require.Contains(t, resp, "Not found")
}

func TestSaveHandler_InternalError(t *testing.T) {
	code, resp := send(t, saveApp(&fakeRepo{fail: true}, okIdentity), http.MethodPatch, "/settings", validSave)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, resp, "Internal server error")
}

func TestSaveHandler_PlanFailure(t *testing.T) {
	app := fiber.New()
	app.Patch("/settings", NewHandler(service.NewService(&fakeRepo{}, nil), failingPlan).WithIdentity(okIdentity).SaveTransactionSettings)
	body := `{"changes":[{"categoryId":null,"transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":3}]}]}`
	code, resp := send(t, app, http.MethodPatch, "/settings", body)
	require.Equal(t, http.StatusInternalServerError, code)
	require.Contains(t, resp, "Internal server error")
}

func TestRequestIdentity(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		if c.Get("X-Snapshot") != "" {
			c.Locals(tsMiddleware.UserRolesKey, tsMiddleware.UserAccessCache{FullName: "Rina Wulandari"})
		}
		return c.Next()
	})
	app.Get("/who", func(c fiber.Ctx) error {
		who, err := requestIdentity(c)
		if err != nil {
			return c.SendStatus(http.StatusUnauthorized)
		}
		return c.SendString(who.ClientID + "|" + who.FullName)
	})
	code, _ := send(t, app, http.MethodGet, "/who", "")
	require.Equal(t, http.StatusUnauthorized, code)

	req := httptest.NewRequest(http.MethodGet, "/who", nil)
	req.Header.Set("X-Snapshot", "1")
	req.Header.Set(constant.HEADER_CLIENT_ID, "c1")
	req.Header.Set(constant.HEADER_USER_ID, "7")
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, "c1|Rina Wulandari", string(raw))
}

func TestSaveHandler_StructValidationPaths(t *testing.T) {
	body := `{"changes":[{"transaction":"borrow","fields":[{"key":"max_duration","mode":"own","value":1}]},` +
		`{"transaction":"nope","fields":[{"key":"a","mode":"own"},{"key":"max_extensions","mode":"maybe"}]},{"transaction":"borrow","fields":[]}]}`
	code, resp := send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", body)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	var parsed struct {
		Error []dto.FieldError `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(resp), &parsed))
	paths := []string{}
	for _, e := range parsed.Error {
		paths = append(paths, e.Field)
	}
	// The mode of the third field of the payload is `fields[2].value`; change-level entries name the change.
	require.ElementsMatch(t, []string{"changes[1].transaction", "fields[2].value", "changes[2].fields"}, paths)

	code, resp = send(t, saveApp(&fakeRepo{}, okIdentity), http.MethodPatch, "/settings", `{"changes":[]}`)
	require.Equal(t, http.StatusUnprocessableEntity, code)
	require.Contains(t, resp, `"field":"changes"`)
}

func TestSavePath_UnknownNamespaceAndNonViolationErrors(t *testing.T) {
	require.Equal(t, "changes", savePath(dto.SaveSettingsRequest{}, "Other.Thing"))
	require.Nil(t, fieldViolations(io.EOF))
}
