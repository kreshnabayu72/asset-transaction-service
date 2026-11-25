package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type checkAvailabilityServiceStub struct {
	clientID  string
	name      string
	excludeID *int64
	available bool
}

func (s *checkAvailabilityServiceStub) CheckAvailability(_ context.Context, clientID, name string, excludeID *int64) (bool, error) {
	s.clientID = clientID
	s.name = name
	s.excludeID = excludeID
	return s.available, nil
}

func TestCheckAvailabilityHandlerValidatesBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "malformed JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "missing name", body: `{}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "name is not a string", body: `{"name":12}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "excludeId is not numeric", body: `{"name":"Workshop","excludeId":"12"}`, wantStatus: http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Post("/check-availability", NewCheckAvailabilityHandler(&checkAvailabilityServiceStub{}).CheckAvailability)
			req := httptest.NewRequest(http.MethodPost, "/check-availability", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

func TestCheckAvailabilityHandlerUsesClientAndReturnsData(t *testing.T) {
	excludeID := int64(73)
	service := &checkAvailabilityServiceStub{available: true}
	app := fiber.New()
	app.Post("/check-availability", NewCheckAvailabilityHandler(service).CheckAvailability)
	req := httptest.NewRequest(http.MethodPost, "/check-availability", strings.NewReader(`{"name":"  Workshop ","excludeId":73}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "tenant-a")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "tenant-a", service.clientID)
	require.Equal(t, "  Workshop ", service.name)
	require.Equal(t, &excludeID, service.excludeID)
	var result struct {
		Message string `json:"message"`
		Data    struct {
			Available bool `json:"available"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, "Success", result.Message)
	require.True(t, result.Data.Available)
}

type exportContractServiceStub struct{}

func (exportContractServiceStub) List(context.Context, string, dto.ListServiceCentersRequest, dto.ServiceCenterReadAccess, tsMiddleware.UserAccessCache) ([]dto.ListServiceCenterItem, int, error) {
	return nil, 0, nil
}

func (exportContractServiceStub) Get(context.Context, string, int64, dto.ServiceCenterReadAccess, tsMiddleware.UserAccessCache) (dto.ServiceCenterDetail, bool, error) {
	return dto.ServiceCenterDetail{}, false, nil
}

func (exportContractServiceStub) Options(context.Context, string, map[string]bool, dto.ServiceCenterReadAccess) (map[string][]dto.OptionItem, error) {
	return nil, nil
}

func (exportContractServiceStub) Export(context.Context, string, dto.ExportServiceCentersRequest, dto.ServiceCenterReadAccess) (*coreHelper.ExportResult, error) {
	return &coreHelper.ExportResult{
		Data:        []byte("csv"),
		Filename:    "Service-Centers.csv",
		ContentType: "text/csv",
	}, nil
}

func TestExportUsesContractContentType(t *testing.T) {
	app := fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(tsMiddleware.UserRolesKey, tsMiddleware.UserAccessCache{
			ModuleAccess: map[enum.ModuleAccess]tsMiddleware.ModuleFlags{
				enum.ModuleAccessFixedAsset: {IsTotalControl: true},
			},
		})
		return c.Next()
	})
	app.Post("/export", NewReadServiceCenterHandler(exportContractServiceStub{}).Export)

	body := `{"columns":[{"header":"Name","key":"name"}],"format":"csv","sortBy":"name","sortOrder":1}`
	req := httptest.NewRequest(http.MethodPost, "/export", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "tenant-a")
	req.Header.Set("X-User-ID", "7")

	resp, err := app.Test(req)

	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "application/octet-stream", resp.Header.Get("Content-Type"))
}
