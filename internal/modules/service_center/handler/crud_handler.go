package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v3"

	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	centerService "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
)

type CRUDOperations interface {
	CreateServiceCenter(ctx context.Context, clientID string, request dto.CreateServiceCenterRequest, actorID int64, actorName string) error
	UpdateServiceCenter(ctx context.Context, clientID string, id int64, request dto.UpdateServiceCenterRequest, actorID int64, actorName string) (bool, error)
	DeleteServiceCenter(ctx context.Context, clientID string, id int64, version int32) error
}

type CRUDHandler struct {
	service CRUDOperations
}

func NewCRUDHandler(service CRUDOperations) *CRUDHandler {
	return &CRUDHandler{service: service}
}

func (h *CRUDHandler) CreateServiceCenter(c fiber.Ctx) error {
	clientID, roles, actorID, err := tsMiddleware.RequestIdentity(c)
	if err != nil || clientID == "" || actorID <= 0 {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	var request dto.CreateServiceCenterRequest
	if err := decodeCRUDBody(c.Body(), &request); err != nil {
		return coreEntity.BadRequest("Invalid request body", coreValidator.ParseBindError(err)).SendResponse(c)
	}
	request.Normalize()
	if errs := request.Validate(); len(errs) > 0 {
		return sendCRUDValidationError(c, errs)
	}
	if err := h.service.CreateServiceCenter(c.Context(), clientID, request, actorID, roles.FullName); err != nil {
		return sendCRUDServiceError(c, err)
	}
	return coreHelper.Created(c, fmt.Sprintf("Success, %s has been added.", request.Name), nil)
}

func (h *CRUDHandler) UpdateServiceCenter(c fiber.Ctx) error {
	clientID, roles, actorID, err := tsMiddleware.RequestIdentity(c)
	if err != nil || clientID == "" || actorID <= 0 {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	id, ok := parseCRUDPathID(c.Params("id"))
	if !ok {
		return coreEntity.BadRequest("Invalid request path").SendResponse(c)
	}
	var request dto.UpdateServiceCenterRequest
	if err := decodeCRUDBody(c.Body(), &request); err != nil {
		return coreEntity.BadRequest("Invalid request body", coreValidator.ParseBindError(err)).SendResponse(c)
	}
	request.Normalize()
	if errs := request.Validate(); len(errs) > 0 {
		return sendCRUDValidationError(c, errs)
	}
	changed, err := h.service.UpdateServiceCenter(c.Context(), clientID, id, request, actorID, roles.FullName)
	if err != nil {
		return sendCRUDServiceError(c, err)
	}
	if !changed {
		return coreHelper.Success(c, "No changes to save.", nil)
	}
	return coreHelper.Success(c, fmt.Sprintf("Success, %s has been updated.", request.Name), nil)
}

func (h *CRUDHandler) DeleteServiceCenter(c fiber.Ctx) error {
	clientID, _, actorID, err := tsMiddleware.RequestIdentity(c)
	if err != nil || clientID == "" || actorID <= 0 {
		return coreEntity.Unauthorized("Unauthorized").SendResponse(c)
	}
	id, ok := parseCRUDPathID(c.Params("id"))
	if !ok {
		return coreEntity.BadRequest("Invalid request path").SendResponse(c)
	}
	version, err := strconv.ParseInt(c.Query("version"), 10, 32)
	if err != nil || version < 1 {
		return coreEntity.BadRequest("Invalid request body").SendResponse(c)
	}
	if err := h.service.DeleteServiceCenter(c.Context(), clientID, id, int32(version)); err != nil {
		return sendCRUDServiceError(c, err)
	}
	return c.SendStatus(http.StatusNoContent)
}

func decodeCRUDBody(body []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body contains multiple JSON values")
		}
		return err
	}
	return nil
}

func parseCRUDPathID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func sendCRUDValidationError(c fiber.Ctx, errs []dto.CRUDFieldError) error {
	fieldErrors := make([]coreValidator.BindFieldError, 0, len(errs))
	for _, fieldErr := range errs {
		fieldErrors = append(fieldErrors, coreValidator.BindFieldError{
			Field:   fieldErr.Field,
			Message: fieldErr.Message,
		})
	}
	return coreEntity.UnprocessableEntity("Invalid request body", fieldErrors).SendResponse(c)
}

func sendCRUDServiceError(c fiber.Ctx, err error) error {
	if errors.Is(err, centerService.ErrCRUDNotFound) {
		return coreEntity.NotFound("This service center is no longer available.").SendResponse(c)
	}
	if errors.Is(err, centerService.ErrCRUDDuplicateName) {
		return sendCRUDValidationError(c, []dto.CRUDFieldError{{Field: "name", Message: "Name already exists"}})
	}
	var versionErr *centerService.CRUDVersionConflictError
	if errors.As(err, &versionErr) {
		conflict := coreEntity.Conflict("This service center was changed by someone else. Please review.")
		conflict.Data = map[string]any{
			"reason": "version_conflict",
			"current": map[string]any{
				"_id":     strconv.FormatInt(versionErr.Current.ID, 10),
				"name":    versionErr.Current.Name,
				"version": versionErr.Current.Version,
			},
		}
		return conflict.SendResponse(c)
	}
	slog.ErrorContext(c.Context(), "Service Center CRUD operation failed", "err", err)
	return coreEntity.InternalServerError("Internal server error").SendResponse(c)
}
