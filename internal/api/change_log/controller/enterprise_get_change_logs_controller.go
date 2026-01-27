package controller

import (
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/helper"
	"github.com/gofiber/fiber/v2"
)

func (c *LogController) GetChangeLogs(ctx *fiber.Ctx) error {
	// Implement the controller logic here

	return helper.SendResponse(ctx, fiber.StatusOK, nil, "Successfully retrieved change logs", nil)
}
