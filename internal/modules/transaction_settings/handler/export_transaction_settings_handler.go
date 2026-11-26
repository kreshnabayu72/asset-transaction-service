package handler

import (
	"errors"
	"log"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	"github.com/gofiber/fiber/v3"
)

// ExportTransactionSettings handles POST /v2/asset-transaction/fixed-asset/transaction-settings/export.
func (h *Handler) ExportTransactionSettings(c fiber.Ctx) error {
	who, err := h.identity(c)
	if err != nil || who.ClientID == "" || who.UserID <= 0 {
		return coreEntity.Unauthorized(msgUnauthorized).SendResponse(c)
	}
	var req dto.ExportRulesRequest
	if err := c.Bind().Body(&req); err != nil {
		return coreEntity.BadRequest(msgInvalidBody).SendResponse(c)
	}
	categoryID, err := parseCategoryID(req.CategoryID)
	if errors.Is(err, errCategoryIDOutOfRange) {
		return coreEntity.NotFound("Not found").SendResponse(c)
	}
	if err != nil {
		return coreEntity.BadRequest(msgInvalidBody).SendResponse(c)
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings export plan read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	file, err := h.svc.ExportTransactionSettings(c.Context(), service.ExportInput{
		ClientID: who.ClientID, CategoryID: categoryID, Plan: plan,
		ActorID: who.UserID, ActorName: who.FullName, ExportedAt: time.Now(),
	})
	switch {
	case errors.Is(err, service.ErrCategoryNotFound):
		return coreEntity.NotFound("Not found").SendResponse(c)
	case err != nil:
		log.Printf("transaction settings export failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	c.Set(fiber.HeaderContentType, fiber.MIMEOctetStream)
	c.Set(fiber.HeaderContentDisposition, "attachment; filename*=UTF-8''"+file.FileName)
	return c.Send(file.Data)
}
