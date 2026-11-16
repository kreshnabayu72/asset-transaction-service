package handler

import (
	"errors"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/group_guard/service"
	contract "gitea.tagsamurai.local/BETS-V2/service-contracts/services/asset_transaction/http"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

type Handler struct{ svc *service.Service }

func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) HandleGroupDeleteGuard(c fiber.Ctx) error {
	var req contract.GroupDeleteGuardRequest
	if err := c.Bind().Body(&req); err != nil {
		return coreEntity.BadRequest("Invalid request body", coreValidator.ParseBindError(err)).SendResponse(c)
	}
	res, err := h.svc.GroupDeleteGuard(c.Context(), c.Get("X-Client-ID"), req)
	if errors.Is(err, service.ErrInvalidGroupIDs) {
		return coreEntity.UnprocessableEntity("Validation failed", []coreValidator.BindFieldError{{Field: "groupIds", Message: err.Error()}}).SendResponse(c)
	}
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	return coreHelper.Success(c, "Data retrieved successfully", res)
}
