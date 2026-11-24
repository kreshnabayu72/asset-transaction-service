// Package handler binds the damaged module read requests and maps service outcomes to the
// response envelope.
package handler

import (
	"errors"
	"log/slog"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/damaged/service"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	"github.com/gofiber/fiber/v3"
)

const (
	msgInternal      = "Internal server error"
	msgAssetNotFound = "Asset not found"
	reasonGroupScope = "group_scope"
)

// Handler serves the damaged module read routes.
type Handler struct {
	svc *service.Service
}

// NewHandler builds the damaged module handler.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// GetSummary serves GET /v2/asset-transaction/damaged/summary.
func (h *Handler) GetSummary(c fiber.Ctx) error {
	clientID, roles, err := callerScope(c)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	data, err := h.svc.Summary(c.Context(), clientID, roles)
	if err != nil {
		slog.ErrorContext(c.Context(), "damaged summary failed", "error", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", data)
}

// GetRunningRecord serves GET /v2/asset-transaction/damaged/assets/:assetId/running-record. An ID
// that is not a decimal Snowflake cannot name a mirror asset, so it answers Asset not found.
func (h *Handler) GetRunningRecord(c fiber.Ctx) error {
	clientID, roles, callerID, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	assetID, ok := parseAssetID(c.Params("assetId"))
	if !ok {
		return coreEntity.NotFound(msgAssetNotFound).SendResponse(c)
	}
	data, err := h.svc.RunningRecord(c.Context(), clientID, roles, callerID, assetID)
	switch {
	case errors.Is(err, service.ErrAssetNotFound):
		return coreEntity.NotFound(msgAssetNotFound).SendResponse(c)
	case errors.Is(err, service.ErrGroupScope):
		denied := coreEntity.Forbidden("Forbidden")
		denied.Data = fiber.Map{"reason": reasonGroupScope}
		return denied.SendResponse(c)
	case err != nil:
		slog.ErrorContext(c.Context(), "damaged running record failed", "error", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", data)
}

// callerScope reads the tenant and the RBAC snapshot that LoadUserRoles placed on the request.
func callerScope(c fiber.Ctx) (string, tsMiddleware.UserAccessCache, error) {
	clientID, err := tsMiddleware.ClientID(c)
	if err != nil {
		return "", tsMiddleware.UserAccessCache{}, err
	}
	roles, ok := tsMiddleware.GetUserRoles(c)
	if !ok {
		return "", tsMiddleware.UserAccessCache{}, coreEntity.Unauthorized("Unauthorized")
	}
	return clientID, roles, nil
}

// parseAssetID accepts only the decimal digit form of a Snowflake ID.
func parseAssetID(raw string) (int64, bool) {
	if raw == "" {
		return 0, false
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}
