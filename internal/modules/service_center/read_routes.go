package service_center

import (
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	"github.com/gofiber/fiber/v3"
)

const serviceCenterReadPath = "/service-centers"

func SetupReadRoutes(app *fiber.App, dbName string, cfg config.AppConfig) {
	readRepository := repository.NewReadServiceCenterRepository(dbName)
	references := service.NewHTTPServiceCenterReferenceResolver(
		cfg.MasterAssetRegistryBaseURL,
		cfg.EnterpriseManagementBaseURL,
		cfg.ServiceToken,
	)
	readService := service.NewServiceCenterReadService(readRepository, references)
	readHandler := handler.NewReadServiceCenterHandler(readService)

	app.Post(serviceCenterReadPath+"/list", readHandler.List)
	app.Post(serviceCenterReadPath+"/options", readHandler.Options)
	app.Post(serviceCenterReadPath+"/export", readHandler.Export)
	app.Get(serviceCenterReadPath+"/:id", readHandler.Get)
}
