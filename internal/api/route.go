package api

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"

	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/api/change_log/route"
)

// RegisterRoutes registers route for REST api endpoint of the application.
func RegisterRoutes(app *fiber.App, serviceManager service.Managerer) {
	slog.Debug("Registering REST api routes...")

	app.Use(cors.New())
	app.Use(logger.New())

	// TODO(refan): Currently, middleware is mocked. It should be implemented later.

	app.Get("/__health", func(ctx *fiber.Ctx) error {
		return ctx.Status(fiber.StatusOK).SendString("OK")
	})

	analyticsReporting := app.Group("/analytics-reporting")
	route.SetupRoute(analyticsReporting, serviceManager)

	slog.Debug("Successfully registered API routes.")
}
