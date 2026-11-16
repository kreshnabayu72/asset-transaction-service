package app

import (
	"context"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/gofiber/fiber/v3/middleware/requestid"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard"
	groupGuardService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/service"
)

// App owns the HTTP server and its lifecycle.
type App struct {
	httpApp *fiber.App
	port    string
}

// New creates the HTTP application with baseline middleware and routes.
func New(appConfig config.AppConfig) *App {
	fiberApp := fiber.New()
	fiberApp.Use(requestid.New())
	fiberApp.Use(logger.New())
	fiberApp.Use(recover.New())

	fiberApp.Get("/health", func(ctx fiber.Ctx) error {
		return ctx.JSON(fiber.Map{"status": "ok"})
	})

	group_guard.SetupInternalRoutes(fiberApp, groupGuardService.NewService(), appConfig.ServiceToken)

	return &App{
		httpApp: fiberApp,
		port:    appConfig.Port,
	}
}

// Run starts the HTTP server.
func (a *App) Run() error {
	return a.httpApp.Listen(":" + a.port)
}

// Shutdown stops the HTTP server using the provided context.
func (a *App) Shutdown(ctx context.Context) error {
	return a.httpApp.ShutdownWithContext(ctx)
}
