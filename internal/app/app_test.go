package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
)

func TestNewRegistersHealthRoute(t *testing.T) {
	serviceApp := New(config.AppConfig{Port: "5013"})

	response, err := serviceApp.httpApp.Test(httptest.NewRequest(http.MethodGet, "/health", nil))
	require.NoError(t, err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.JSONEq(t, `{"status":"ok"}`, string(body))
}
