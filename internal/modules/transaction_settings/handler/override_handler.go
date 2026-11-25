package handler

import (
	"errors"
	"log"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

const (
	msgOverrideAdded      = "Success, override has been added."
	msgOverrideDeleted    = "Success, override has been deleted."
	msgOverrideSkipped    = "Success, override has been deleted. Some items were skipped."
	msgDuplicateOverride  = "An active override already exists for this target, category, and transaction. Delete it first"
	msgAssigneeGuard      = "At least one assignee type must be enabled"
	msgOverrideAddFailed  = "Error, failed to add override."
	msgOverrideDeleteFail = "Error, failed to delete override."
	msgSuccess            = "Success"
)

// overrideFailure answers the errors the override operations share.
func overrideFailure(c fiber.Ctx, err error, internalMsg string) error {
	var invalid *service.ValidationError
	var dup *service.DuplicateOverrideError
	var guard *service.GuardSkipError
	switch {
	case errors.As(err, &invalid):
		out := make([]coreValidator.BindFieldError, 0, len(invalid.Errors))
		for _, e := range invalid.Errors {
			out = append(out, coreValidator.BindFieldError{Field: e.Field, Message: e.Message})
		}
		return coreEntity.UnprocessableEntity(msgValidation, out).SendResponse(c)
	case errors.Is(err, service.ErrOverrideForbidden):
		return coreEntity.Forbidden(msgForbidden).SendResponse(c)
	case errors.Is(err, service.ErrCategoryNotFound), errors.Is(err, service.ErrTargetNotFound):
		return coreEntity.NotFound("Not found").SendResponse(c)
	case errors.As(err, &dup):
		return coreHelper.SendResponse(c, fiber.StatusConflict, dto.DuplicateOverrideData{DuplicateTargetIDs: idStrings(dup.TargetIDs)}, msgDuplicateOverride, nil)
	case errors.As(err, &guard):
		return coreHelper.SendResponse(c, fiber.StatusConflict, dto.DeleteOverridesData{DeletedIDs: []string{}, Skipped: skippedDTO(guard.Skipped)}, msgAssigneeGuard, nil)
	}
	log.Printf("transaction settings override request failed: %v", err)
	return coreEntity.InternalServerError(internalMsg).SendResponse(c)
}

func idStrings(ids []int64) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, strconv.FormatInt(id, 10))
	}
	return out
}

func skippedDTO(rows []service.SkippedRow) []dto.SkippedOverride {
	out := make([]dto.SkippedOverride, 0, len(rows))
	for _, r := range rows {
		out = append(out, dto.SkippedOverride{ID: strconv.FormatInt(r.ID, 10), Reason: r.Reason})
	}
	return out
}

// bindOverrideBody authenticates the caller and binds the body; ok=false means the
// response was already sent.
func bindOverrideBody[T any](h *Handler, c fiber.Ctx, req *T) (Identity, bool, error) {
	who, err := h.identity(c)
	if err != nil || who.ClientID == "" || who.UserID <= 0 {
		return who, false, coreEntity.Unauthorized(msgUnauthorized).SendResponse(c)
	}
	if bindErr := coreValidator.BindBody(c, req); bindErr != nil {
		return who, false, bindErr.SendResponse(c)
	}
	// Tier 2 (field validation) runs in the service: the LLD fixes a literal message per field.
	return who, true, nil
}

// CreateOverrides handles POST /v2/asset-transaction/fixed-asset/transaction-settings/overrides.
func (h *Handler) CreateOverrides(c fiber.Ctx) error {
	var req dto.CreateOverrideRequest
	who, ok, err := bindOverrideBody(h, c, &req)
	if !ok {
		return err
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings override plan read failed: %v", err)
		return coreEntity.InternalServerError(msgOverrideAddFailed).SendResponse(c)
	}
	if err := h.svc.CreateOverrides(c.Context(), service.CreateOverrideInput{
		ClientID: who.ClientID, Request: req, Plan: plan, ActorID: who.UserID, ActorName: who.FullName, Access: who.Access,
	}); err != nil {
		return overrideFailure(c, err, msgOverrideAddFailed)
	}
	return coreHelper.Created(c, msgOverrideAdded, nil)
}

// ListOverrides handles POST .../overrides/list.
func (h *Handler) ListOverrides(c fiber.Ctx) error {
	var req dto.ListOverridesRequest
	who, ok, err := bindOverrideBody(h, c, &req)
	if !ok {
		return err
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings override list plan read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	data, err := h.svc.ListOverrides(c.Context(), service.ListOverridesInput{
		ClientID: who.ClientID, Request: req, Plan: plan, Access: who.Access,
	})
	if err != nil {
		return overrideFailure(c, err, msgInternal)
	}
	return coreHelper.Success(c, msgSuccess, data)
}

// OverridesOptions handles POST .../overrides/options.
func (h *Handler) OverridesOptions(c fiber.Ctx) error {
	var req dto.OverridesOptionsRequest
	who, ok, err := bindOverrideBody(h, c, &req)
	if !ok {
		return err
	}
	data, err := h.svc.OverrideOptions(c.Context(), service.OverridesOptionsInput{
		ClientID: who.ClientID, Request: req, Access: who.Access,
	})
	if err != nil {
		return overrideFailure(c, err, msgInternal)
	}
	return coreHelper.Success(c, msgSuccess, data)
}

// DeleteOverrides handles DELETE .../overrides.
func (h *Handler) DeleteOverrides(c fiber.Ctx) error {
	var req dto.DeleteOverridesRequest
	who, ok, err := bindOverrideBody(h, c, &req)
	if !ok {
		return err
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings override delete plan read failed: %v", err)
		return coreEntity.InternalServerError(msgOverrideDeleteFail).SendResponse(c)
	}
	res, err := h.svc.DeleteOverrides(c.Context(), service.DeleteOverridesInput{
		ClientID: who.ClientID, Request: req, Plan: plan, ActorID: who.UserID, ActorName: who.FullName, Access: who.Access,
	})
	if err != nil {
		return overrideFailure(c, err, msgOverrideDeleteFail)
	}
	if res.NoneExist {
		return c.SendStatus(fiber.StatusNoContent)
	}
	data := dto.DeleteOverridesData{DeletedIDs: idStrings(res.DeletedIDs), Skipped: skippedDTO(res.Skipped)}
	if len(res.Skipped) > 0 {
		return coreHelper.Success(c, msgOverrideSkipped, data)
	}
	return coreHelper.Success(c, msgOverrideDeleted, data)
}
