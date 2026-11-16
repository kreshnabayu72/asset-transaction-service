package group_guard

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/service"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"github.com/gofiber/fiber/v3"
)

// SetupInternalRoutes mounts the iam-called group delete guard behind the
// X-Service-Token check: POST /internal/asset-transactions/group-delete-guard.
func SetupInternalRoutes(app *fiber.App, svc *service.Service, serviceToken string) {
	h := handler.NewHandler(svc)
	app.Group("/internal/asset-transactions", tsMiddleware.RequireServiceToken(serviceToken)).
		Post("/group-delete-guard", h.HandleGroupDeleteGuard)
}
