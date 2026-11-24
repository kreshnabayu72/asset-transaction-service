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

// parseAssetIDs turns decimal Snowflake ids into int64, dropping duplicates and keeping request order.
// It returns the {field, message} entries of malformed ids.
func parseAssetIDs(raw []string) ([]int64, []coreValidator.BindFieldError) {
	var (
		ids  []int64
		errs []coreValidator.BindFieldError
		seen = map[int64]bool{}
	)
	for i, s := range raw {
		id, err := strconv.ParseInt(s, 10, 64)
		if err != nil || id <= 0 || s != strconv.FormatInt(id, 10) {
			errs = append(errs, coreValidator.BindFieldError{Field: "assetIds[" + strconv.Itoa(i) + "]", Message: "must be a positive numeric id"})
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, errs
}

// ResolveEffectiveRules handles POST /v2/asset-transaction/fixed-asset/transaction-settings/effective-rules.
func (h *Handler) ResolveEffectiveRules(c fiber.Ctx) error {
	who, err := h.identity(c)
	if err != nil || who.ClientID == "" {
		return coreEntity.Unauthorized(msgUnauthorized).SendResponse(c)
	}
	var req dto.EffectiveRulesRequest
	if bindErr := coreValidator.BindBody(c, &req); bindErr != nil {
		return bindErr.SendResponse(c)
	}
	if err := coreValidator.Validate.Struct(&req); err != nil {
		return validationFailed(c, err)
	}
	ids, idErrs := parseAssetIDs(req.AssetIDs)
	if len(idErrs) > 0 {
		return coreEntity.UnprocessableEntity(msgValidation, idErrs).SendResponse(c)
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings effective rules plan read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	data, err := h.svc.ResolveEffectiveRules(c.Context(), service.EffectiveInput{
		ClientID: who.ClientID, Transaction: req.Transaction, AssetIDs: ids, Plan: plan, Access: who.Access,
	})
	switch {
	case errors.Is(err, service.ErrAssetNotFound):
		return coreEntity.NotFound("Not found").SendResponse(c)
	case err != nil:
		log.Printf("transaction settings effective rules failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", data)
}
