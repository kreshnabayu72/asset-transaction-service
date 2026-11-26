package handler

import (
	"errors"
	"log"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	"github.com/gofiber/fiber/v3"
)

// ReadTransactionSettings handles GET /v2/asset-transaction/fixed-asset/transaction-settings.
func (h *Handler) ReadTransactionSettings(c fiber.Ctx) error {
	raw := c.Query("categoryId")
	categoryID, err := parseCategoryID(&raw)
	if errors.Is(err, errCategoryIDOutOfRange) {
		return coreEntity.NotFound("Not found").SendResponse(c)
	}
	if err != nil {
		return coreEntity.BadRequest(msgInvalidBody).SendResponse(c)
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings plan read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	data, err := h.svc.ReadTransactionSettings(c.Context(), c.Get(constant.HEADER_CLIENT_ID), categoryID, plan)
	switch {
	case errors.Is(err, service.ErrCategoryNotFound):
		return coreEntity.NotFound("Not found").SendResponse(c)
	case err != nil:
		log.Printf("transaction settings read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", data)
}
