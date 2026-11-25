package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	statusService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

type StatusOperation interface {
	ChangeStatus(
		ctx context.Context,
		clientID string,
		serviceCenterID int64,
		request dto.ChangeStatusRequest,
		authorization statusService.StatusAuthorization,
		actorID int64,
		actorName string,
	) (statusService.StatusChangeResult, error)
}

type StatusHandler struct {
	service StatusOperation
}

func NewStatusHandler(service StatusOperation) *StatusHandler {
	return &StatusHandler{service: service}
}

func (h *StatusHandler) PatchStatus(c fiber.Ctx) error {
	clientID, roles, actorID, err := tsMiddleware.RequestIdentity(c)
	if err != nil || clientID == "" || actorID <= 0 {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}

	authorization := statusAuthorizationFromSnapshot(roles)
	if !authorization.HasStatusRight() {
		return statusForbiddenResponse(c, "capability")
	}

	serviceCenterID, ok := parseStatusPathID(c.Params("id"))
	if !ok {
		return coreEntity.BadRequest("Invalid request path").SendResponse(c)
	}

	request, fieldErrors, err := dto.ParseChangeStatusRequest(c.Body())
	if errors.Is(err, dto.ErrMalformedStatusRequest) {
		return coreEntity.BadRequest("Invalid request body").SendResponse(c)
	}
	if err != nil {
		return coreEntity.InternalServerError("Internal server error").SendResponse(c)
	}
	if len(fieldErrors) > 0 {
		validationErrors := make([]coreValidator.BindFieldError, 0, len(fieldErrors))
		for _, fieldError := range fieldErrors {
			validationErrors = append(validationErrors, coreValidator.BindFieldError{
				Field:   fieldError.Field,
				Message: fieldError.Message,
			})
		}
		return coreEntity.UnprocessableEntity("Invalid request body", validationErrors).SendResponse(c)
	}

	result, err := h.service.ChangeStatus(
		c.Context(),
		clientID,
		serviceCenterID,
		request,
		authorization,
		actorID,
		roles.FullName,
	)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	if result.Changed {
		return c.SendStatus(http.StatusNoContent)
	}
	return c.Status(http.StatusOK).JSON(fiber.Map{"message": "Success"})
}

func statusAuthorizationFromSnapshot(roles tsMiddleware.UserAccessCache) statusService.StatusAuthorization {
	authorization := statusService.StatusAuthorization{
		TotalControl: roles.IsTotalControl(enum.ModuleAccessFixedAsset),
		ManageGroupIDs: roles.GroupIDsWithPermission(
			tsMiddleware.PermEdit,
			enum.SystemRoleKeyManageGroup,
		),
	}

	adminGroupIDs := roles.GroupIDsWithPermission(
		tsMiddleware.PermEdit,
		enum.SystemRoleKeyManageAssetAndTAG,
	)
	for _, groupID := range adminGroupIDs {
		if hasStatusTransactionRole(roles, groupID) {
			authorization.AssetAdminGroupIDs = append(authorization.AssetAdminGroupIDs, groupID)
		}
	}
	return authorization
}

func hasStatusTransactionRole(roles tsMiddleware.UserAccessCache, groupID int64) bool {
	roleTypes := []enum.TransactionRolesType{
		enum.TransactionRolesTypeManager,
		enum.TransactionRolesTypeMonitoring,
		enum.TransactionRolesTypeStaff,
	}
	return roles.HasTransactionRoleFixedAsset(
		enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole,
		groupID,
		roleTypes...,
	) || roles.HasTransactionRoleFixedAsset(
		enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole,
		groupID,
		roleTypes...,
	)
}

func parseStatusPathID(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func statusForbiddenResponse(c fiber.Ctx, reason string) error {
	err := coreEntity.Forbidden("Forbidden")
	err.Data = fiber.Map{"reason": reason}
	return err.SendResponse(c)
}
