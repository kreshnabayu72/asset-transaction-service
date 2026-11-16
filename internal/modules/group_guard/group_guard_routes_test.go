package group_guard

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/service"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func do(t *testing.T, token, body string) (int, string) {
	t.Helper()
	app := fiber.New()
	SetupInternalRoutes(app, service.NewService(), "secret")
	req := httptest.NewRequest(http.MethodPost, "/internal/asset-transactions/group-delete-guard", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Service-Token", token)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestGroupDeleteGuardRoute(t *testing.T) {
	code, body := do(t, "secret", `{"groupIds":["10","20","10"]}`)
	assert.Equal(t, 200, code)
	assert.Contains(t, body, `"results":[{"groupId":"10","ongoingTransfer":false,"ongoingDisposal":false},{"groupId":"20","ongoingTransfer":false,"ongoingDisposal":false}]`)

	code, _ = do(t, "", `{"groupIds":["10"]}`)
	assert.Equal(t, 401, code)
	code, _ = do(t, "wrong", `{"groupIds":["10"]}`)
	assert.Equal(t, 401, code)

	for _, b := range []string{`{"groupIds":[]}`, `{"groupIds":["abc"]}`, `{"groupIds":["0"]}`} {
		code, _ = do(t, "secret", b)
		assert.Equal(t, 422, code, b)
	}
	code, _ = do(t, "secret", `not json`)
	assert.Equal(t, 400, code)
}
