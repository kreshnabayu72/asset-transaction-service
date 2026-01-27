package controller

import (
	"net/http/httptest"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestGetChangeLogs(t *testing.T) {
	// Setup
	app := fiber.New()
	mockServiceManager := service.MakeManager() // Or use a mock if needed, but controller doesn't use it yet
	logController := &LogController{ServiceManager: mockServiceManager}

	app.Get("/logs", logController.GetChangeLogs)

	// Test
	req := httptest.NewRequest("GET", "/logs", nil)
	resp, err := app.Test(req)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}
