package handler

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/enum"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	"github.com/gofiber/fiber/v3"
)

type ReadServiceCenterHandler struct {
	service ReadServiceCenterOperations
}

type ReadServiceCenterOperations interface {
	List(context.Context, string, dto.ListServiceCentersRequest, dto.ServiceCenterReadAccess, tsMiddleware.UserAccessCache) ([]dto.ListServiceCenterItem, int, error)
	Get(context.Context, string, int64, dto.ServiceCenterReadAccess, tsMiddleware.UserAccessCache) (dto.ServiceCenterDetail, bool, error)
	Options(context.Context, string, map[string]bool, dto.ServiceCenterReadAccess) (map[string][]dto.OptionItem, error)
	Export(context.Context, string, dto.ExportServiceCentersRequest, dto.ServiceCenterReadAccess) (*coreHelper.ExportResult, error)
}

func NewReadServiceCenterHandler(readService ReadServiceCenterOperations) *ReadServiceCenterHandler {
	return &ReadServiceCenterHandler{service: readService}
}

func (h *ReadServiceCenterHandler) List(c fiber.Ctx) error {
	clientID, roles, _, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	access := readAccessFromSnapshot(roles)
	if !canReadServiceCenters(access) {
		return readForbidden().SendResponse(c)
	}

	var request dto.ListServiceCentersRequest
	if err := c.Bind().Body(&request); err != nil {
		return coreEntity.BadRequest("Invalid request body", err).SendResponse(c)
	}
	validationErrors := request.Validate()
	validationErrors = append(validationErrors, dto.UnknownReadRequestFields(c.Body(),
		"page", "limit", "sortBy", "sortOrder", "columns", "search", "serviceType",
		"specialization", "availableTo", "brand", "status", "country", "city",
		"modifiedBy", "updatedAt", "usageMin", "usageMax",
	)...)
	if len(validationErrors) > 0 {
		return coreEntity.UnprocessableEntity("Invalid request body", validationErrors).SendResponse(c)
	}

	items, total, err := h.service.List(c.Context(), clientID, request, access, roles)
	if err != nil {
		return readError(c, "list service centers", err)
	}
	return coreHelper.Success(c, "Success", map[string]any{"data": items, "totalRecords": total})
}

func (h *ReadServiceCenterHandler) Options(c fiber.Ctx) error {
	clientID, roles, _, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	access := readAccessFromSnapshot(roles)
	if !canReadServiceCenters(access) {
		return readForbidden().SendResponse(c)
	}

	var request dto.OptionsServiceCentersRequest
	if err := c.Bind().Body(&request); err != nil {
		return coreEntity.BadRequest("Invalid request body", err).SendResponse(c)
	}
	facets, validationErrors := request.RequestedFacets()
	if len(validationErrors) > 0 {
		return coreEntity.UnprocessableEntity("Invalid request body", validationErrors).SendResponse(c)
	}
	result, err := h.service.Options(c.Context(), clientID, facets, access)
	if err != nil {
		return readError(c, "read service center options", err)
	}
	return coreHelper.Success(c, "Success", result)
}

func (h *ReadServiceCenterHandler) Export(c fiber.Ctx) error {
	clientID, roles, _, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	access := readAccessFromSnapshot(roles)
	if !canReadServiceCenters(access) {
		return readForbidden().SendResponse(c)
	}

	var request dto.ExportServiceCentersRequest
	if err := c.Bind().Body(&request); err != nil {
		return coreEntity.BadRequest("Invalid request body", err).SendResponse(c)
	}
	validationErrors := request.Validate()
	validationErrors = append(validationErrors, dto.UnknownReadRequestFields(c.Body(),
		"page", "limit", "format", "sortBy", "sortOrder", "columns", "search", "serviceType",
		"specialization", "availableTo", "brand", "status", "country", "city",
		"modifiedBy", "updatedAt", "usageMin", "usageMax",
	)...)
	if len(validationErrors) > 0 {
		return coreEntity.UnprocessableEntity("Invalid request body", validationErrors).SendResponse(c)
	}
	file, err := h.service.Export(c.Context(), clientID, request, access)
	if err != nil {
		return readError(c, "export service centers", err)
	}
	c.Set("Content-Type", "application/octet-stream")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", file.Filename))
	return c.Send(file.Data)
}

func (h *ReadServiceCenterHandler) Get(c fiber.Ctx) error {
	clientID, roles, _, err := tsMiddleware.RequestIdentity(c)
	if err != nil {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	access := readAccessFromSnapshot(roles)
	if !canReadServiceCenters(access) {
		return readForbidden().SendResponse(c)
	}

	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return coreEntity.NotFound("This service center is no longer available.").SendResponse(c)
	}
	detail, found, err := h.service.Get(c.Context(), clientID, id, access, roles)
	if err != nil {
		return readError(c, "get service center", err)
	}
	if !found {
		return coreEntity.NotFound("This service center is no longer available.").SendResponse(c)
	}
	return coreHelper.Success(c, "Success", detail)
}

func canReadServiceCenters(access dto.ServiceCenterReadAccess) bool {
	return access.TotalControl || access.ReadOnly || len(access.GroupIDs) > 0
}

func readAccessFromSnapshot(roles tsMiddleware.UserAccessCache) dto.ServiceCenterReadAccess {
	if roles.IsTotalControl(enum.ModuleAccessFixedAsset) {
		return dto.ServiceCenterReadAccess{TotalControl: true}
	}
	if roles.IsReadOnly(enum.ModuleAccessFixedAsset) {
		return dto.ServiceCenterReadAccess{ReadOnly: true}
	}
	groups := make(map[int64]struct{})
	for _, groupID := range roles.GroupIDsWithPermission(tsMiddleware.PermView, enum.SystemRoleKeyManageGroup) {
		groups[groupID] = struct{}{}
	}
	for _, groupID := range roles.GroupIDsWithPermission(tsMiddleware.PermView, enum.SystemRoleKeyManageAssetAndTAG) {
		if roles.HasTransactionRoleFixedAsset(enum.FixedAssetTransactionRoleTransactionKeyMaintenanceRoutineRole, groupID, enum.TransactionRolesTypeManager, enum.TransactionRolesTypeMonitoring) ||
			roles.HasTransactionRoleFixedAsset(enum.FixedAssetTransactionRoleTransactionKeyRepairTicketingRole, groupID, enum.TransactionRolesTypeManager, enum.TransactionRolesTypeMonitoring) {
			groups[groupID] = struct{}{}
		}
	}
	ids := make([]int64, 0, len(groups))
	for groupID := range groups {
		ids = append(ids, groupID)
	}
	slices.Sort(ids)
	return dto.ServiceCenterReadAccess{GroupIDs: ids}
}

func readForbidden() *coreEntity.HttpError {
	err := coreEntity.Forbidden("Forbidden")
	err.Data = map[string]string{"reason": "capability"}
	return err
}

func readError(c fiber.Ctx, operation string, err error) error {
	slog.ErrorContext(c.Context(), operation, "err", err)
	return coreEntity.ToHttpError(err).SendResponse(c)
}
