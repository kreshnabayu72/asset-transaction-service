package handler

import (
	"context"

	"github.com/gofiber/fiber/v3"

	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type CheckEligibilityService interface {
	CheckEligibility(ctx context.Context, clientID string, request dto.CheckEligibilityRequest) (dto.CheckEligibilityResponse, error)
}

type CheckEligibilityHandler struct {
	service CheckEligibilityService
}

func NewCheckEligibilityHandler(service CheckEligibilityService) *CheckEligibilityHandler {
	return &CheckEligibilityHandler{service: service}
}

func (h *CheckEligibilityHandler) CheckEligibility(c fiber.Ctx) error {
	request, validationErrors, err := dto.ParseCheckEligibilityRequest(c.Body())
	if err != nil {
		return coreEntity.BadRequest("Invalid request body", coreValidator.ParseBindError(err)).SendResponse(c)
	}
	if len(validationErrors) > 0 {
		errors := make([]coreValidator.BindFieldError, 0, len(validationErrors))
		for _, validationError := range validationErrors {
			errors = append(errors, coreValidator.BindFieldError{
				Field:   validationError.Field,
				Message: validationError.Message,
			})
		}
		return coreEntity.UnprocessableEntity("Invalid request body", errors).SendResponse(c)
	}

	clientID, err := tsMiddleware.ClientID(c)
	if err != nil {
		return coreEntity.BadRequest("Invalid client id").SendResponse(c)
	}

	response, err := h.service.CheckEligibility(c.Context(), clientID, request)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", response)
}
