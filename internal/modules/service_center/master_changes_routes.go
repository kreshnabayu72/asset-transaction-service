package service_center

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	"github.com/gofiber/fiber/v3"
)

// SetupMasterChangesRoutes mounts the internal service-token protected endpoint.
func SetupMasterChangesRoutes(
	app *fiber.App,
	dbName, serviceToken string,
	dispatcher ...repository.MasterChangesSideEffectDispatcher,
) {
	repo := repository.NewMasterChangesRepository(dbName, dispatcher...)
	masterChangesService := service.NewMasterChangesService(repo)
	masterChangesHandler := handler.NewMasterChangesHandler(masterChangesService)

	app.Group("/asset-transaction/internal/service-centers", tsMiddleware.RequireServiceToken(serviceToken)).
		Post("/master-changes", masterChangesHandler.MasterChanges)
}
