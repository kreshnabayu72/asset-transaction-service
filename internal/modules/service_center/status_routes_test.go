package service_center

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestSetupStatusRoutesRegistersPatch(t *testing.T) {
	app := fiber.New()
	SetupStatusRoutes(app, "asset-transaction", nil)

	request := httptest.NewRequest(http.MethodPatch, "/service-centers/42/status", nil)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d (route should run its identity gate)", response.StatusCode, http.StatusUnauthorized)
	}
}
