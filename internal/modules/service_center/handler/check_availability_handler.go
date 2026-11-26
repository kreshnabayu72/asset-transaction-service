package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"

	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

type CheckAvailabilityService interface {
	CheckAvailability(ctx context.Context, clientID, name string, excludeID *int64) (bool, error)
}

type CheckAvailabilityHandler struct {
	service CheckAvailabilityService
}

func NewCheckAvailabilityHandler(service CheckAvailabilityService) *CheckAvailabilityHandler {
	return &CheckAvailabilityHandler{service: service}
}

func (h *CheckAvailabilityHandler) CheckAvailability(c fiber.Ctx) error {
	if !json.Valid(c.Body()) {
		return coreEntity.BadRequest("Invalid request body").SendResponse(c)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(c.Body(), &body); err != nil {
		return invalidRequestBody(c, "", "must be a JSON object")
	}

	nameRaw, ok := body["name"]
	if !ok {
		return invalidRequestBody(c, "name", "is required")
	}
	var name string
	if bytes.Equal(bytes.TrimSpace(nameRaw), []byte("null")) || json.Unmarshal(nameRaw, &name) != nil {
		return invalidRequestBody(c, "name", "must be a string")
	}

	var excludeID *int64
	if raw, ok := body["excludeId"]; ok && !bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		token := bytes.TrimSpace(raw)
		if len(token) == 0 || token[0] == '"' {
			return invalidRequestBody(c, "excludeId", "must be a number")
		}
		var numeric json.Number
		if err := json.Unmarshal(token, &numeric); err != nil {
			return invalidRequestBody(c, "excludeId", "must be a number")
		}
		parsed, err := strconv.ParseInt(string(numeric), 10, 64)
		if err != nil {
			return invalidRequestBody(c, "excludeId", "must be an integer")
		}
		excludeID = &parsed
	}

	clientID, err := tsMiddleware.ClientID(c)
	if err != nil {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	available, err := h.service.CheckAvailability(c.Context(), clientID, name, excludeID)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", map[string]bool{"available": available})
}

func invalidRequestBody(c fiber.Ctx, field, message string) error {
	return coreEntity.UnprocessableEntity("Invalid request body", []coreValidator.BindFieldError{{
		Field:   field,
		Message: strings.TrimSpace(message),
	}}).SendResponse(c)
}
