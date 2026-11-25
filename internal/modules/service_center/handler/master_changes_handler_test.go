package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

type masterChangesServiceStub struct {
	clientID string
	change   dto.MasterChangesRequest
	affected int64
	err      error
	calls    int
}

func (s *masterChangesServiceStub) ApplyMasterChanges(
	_ context.Context,
	clientID string,
	change dto.MasterChangesRequest,
) (int64, error) {
	s.clientID = clientID
	s.change = change
	s.calls++
	return s.affected, s.err
}

func TestMasterChangesHandlerValidatesBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "malformed JSON", body: "{", wantStatus: http.StatusBadRequest},
		{name: "non-object JSON", body: `[]`, wantStatus: http.StatusBadRequest},
		{name: "invalid type", body: `{"type":"asset","action":"deleted","ids":["1"],"mergedIntoId":null}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "invalid action", body: `{"type":"category","action":"archived","ids":["1"],"mergedIntoId":null}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "empty ids", body: `{"type":"brand","action":"deleted","ids":[],"mergedIntoId":null}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "number ID", body: `{"type":"brand","action":"deleted","ids":[1],"mergedIntoId":null}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "merge target required", body: `{"type":"brand","action":"merged","ids":["1"],"mergedIntoId":null}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "merge target must otherwise be null", body: `{"type":"brand","action":"deleted","ids":["1"],"mergedIntoId":"2"}`, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &masterChangesServiceStub{}
			app := fiber.New()
			app.Post("/master-changes", NewMasterChangesHandler(service).MasterChanges)
			req := httptest.NewRequest(http.MethodPost, "/master-changes", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, tt.wantStatus, resp.StatusCode)
			require.Zero(t, service.calls)
		})
	}
}

func TestMasterChangesHandlerReturnsAffected(t *testing.T) {
	service := &masterChangesServiceStub{affected: 3}
	app := fiber.New()
	app.Post("/master-changes", NewMasterChangesHandler(service).MasterChanges)
	req := httptest.NewRequest(http.MethodPost, "/master-changes", strings.NewReader(
		`{"type":"brand","action":"merged","ids":["10","11"],"mergedIntoId":"99"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "tenant-a")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, 1, service.calls)
	require.Equal(t, "tenant-a", service.clientID)
	require.Equal(t, []int64{10, 11}, service.change.IDs)
	require.Equal(t, int64(99), *service.change.MergedIntoID)
	var result struct {
		Message string `json:"message"`
		Data    struct {
			Affected int64 `json:"affected"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	require.Equal(t, "Success", result.Message)
	require.Equal(t, int64(3), result.Data.Affected)
}

func TestMasterChangesHandlerMapsUnsupportedMergeToInternalError(t *testing.T) {
	service := &masterChangesServiceStub{err: errors.New("unsupported composite-link insert")}
	app := fiber.New()
	app.Post("/master-changes", NewMasterChangesHandler(service).MasterChanges)
	req := httptest.NewRequest(http.MethodPost, "/master-changes", strings.NewReader(
		`{"type":"brand","action":"merged","ids":["10"],"mergedIntoId":"99"}`,
	))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Client-ID", "tenant-a")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, 1, service.calls)
}
