package route

import (
	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/api/change_log/controller"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
	"github.com/gofiber/fiber/v2"
)

func SetupRoute(router fiber.Router, serviceManager service.Managerer) {
	c := SetupController(serviceManager)

	log := router.Group("/change-log")

	log.Get("", c.GetChangeLogs)
}

func SetupController(serviceManager service.Managerer) *controller.LogController {
	return &controller.LogController{
		ServiceManager: serviceManager,
	}
}
