package service_center

import (
	"github.com/gofiber/fiber/v3"

	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/config"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
)

const checkEligibilityPath = "/asset-transaction/internal/service-centers/eligibility"

func SetupEligibilityRoutes(app *fiber.App, cfg config.AppConfig) {
	repo := repository.NewServiceCenterEligibilityRepository(cfg.PostgresDB)
	checkEligibilityService := service.NewCheckEligibilityService(repo)
	checkEligibilityHandler := handler.NewCheckEligibilityHandler(checkEligibilityService)

	app.Post(checkEligibilityPath, tsMiddleware.RequireServiceToken(cfg.ServiceToken), checkEligibilityHandler.CheckEligibility)
}
