package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type checkEligibilityServiceStub struct {
	clientID string
	request  dto.CheckEligibilityRequest
	response dto.CheckEligibilityResponse
	calls    int
	err      error
}

func (s *checkEligibilityServiceStub) CheckEligibility(_ context.Context, clientID string, request dto.CheckEligibilityRequest) (dto.CheckEligibilityResponse, error) {
	s.calls++
	s.clientID = clientID
	s.request = request
	return s.response, s.err
}

func TestCheckEligibilityHandlerValidatesBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "malformed JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "non-object JSON", body: "[]", wantStatus: http.StatusUnprocessableEntity},
		{name: "missing required values", body: "{}", wantStatus: http.StatusUnprocessableEntity},
		{
			name:       "invalid nested IDs",
			body:       `{"serviceCenterId":"1","serviceType":"Routine","assets":[{"groupId":2,"categoryId":"3","brandId":null}]}`,
			wantStatus: http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &checkEligibilityServiceStub{}
			app := fiber.New()
			app.Post("/eligibility", NewCheckEligibilityHandler(service).CheckEligibility)
			req := httptest.NewRequest(http.MethodPost, "/eligibility", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Client-ID", "tenant-a")

			resp, err := app.Test(req)

			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.Zero(t, service.calls)
		})
	}
}

func TestCheckEligibilityHandlerUsesTenantAndReturnsEnvelope(t *testing.T) {
	service := &checkEligibilityServiceStub{
		response: dto.CheckEligibilityResponse{
			Eligible: false,
			Reason:   stringPointer("available_to"),
			Message:  stringPointer("Partner Workshop doesn't cover this asset."),
		},
	}
	app := fiber.New()
	app.Post("/eligibility", NewCheckEligibilityHandler(service).CheckEligibility)
	req := httptest.NewRequest(http.MethodPost, "/eligibility", strings.NewReader(`{"serviceCenterId":"1924881519926425600","serviceType":"Routine","assets":[{"groupId":"1924881519926425702","categoryId":"1924881519926425703","brandId":null}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "tenant-a")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, service.calls)
	require.Equal(t, "tenant-a", service.clientID)
	require.Equal(t, int64(1924881519926425600), service.request.ServiceCenterID)
	require.Equal(t, "Routine", service.request.ServiceType)
	require.Len(t, service.request.Assets, 1)
	require.Nil(t, service.request.Assets[0].BrandID)

	var result struct {
		Message string `json:"message"`
		Data    struct {
			Eligible bool    `json:"eligible"`
			Reason   *string `json:"reason"`
			Message  *string `json:"message"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, "Success", result.Message)
	require.False(t, result.Data.Eligible)
	require.Equal(t, "available_to", *result.Data.Reason)
	require.Equal(t, "Partner Workshop doesn't cover this asset.", *result.Data.Message)
}

func TestCheckEligibilityHandlerRejectsMissingClientID(t *testing.T) {
	service := &checkEligibilityServiceStub{}
	app := fiber.New()
	app.Post("/eligibility", NewCheckEligibilityHandler(service).CheckEligibility)
	req := httptest.NewRequest(http.MethodPost, "/eligibility", strings.NewReader(`{"serviceCenterId":"1","serviceType":"Repair","assets":[{"groupId":"2","categoryId":"3","brandId":null}]}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Zero(t, service.calls)
}

func stringPointer(value string) *string {
	return &value
}
