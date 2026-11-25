package handler

import (
	"errors"
	"log"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

const (
	msgSaved        = "Success, settings have been saved."
	msgNoChanges    = "No changes to save."
	msgValidation   = "Validation failed"
	msgForbidden    = "Forbidden"
	msgInternal     = "Internal server error"
	msgUnauthorized = "Unauthorized"
)

// validationFailed answers 422 with {field, message} entries from struct-tag violations.
func validationFailed(c fiber.Ctx, err error) error {
	fields := coreValidator.ParseValidationErrors(err)
	out := make([]coreValidator.BindFieldError, 0, len(fields))
	for _, f := range fields {
		out = append(out, coreValidator.BindFieldError{Field: f.Field, Message: "failed rule " + f.Rule})
	}
	return coreEntity.UnprocessableEntity(msgValidation, out).SendResponse(c)
}

// SaveTransactionSettings handles PATCH /v2/asset-transaction/fixed-asset/transaction-settings.
func (h *Handler) SaveTransactionSettings(c fiber.Ctx) error {
	who, err := h.identity(c)
	if err != nil || who.ClientID == "" || who.UserID <= 0 {
		return coreEntity.Unauthorized(msgUnauthorized).SendResponse(c)
	}
	var req dto.SaveSettingsRequest
	if bindErr := coreValidator.BindBody(c, &req); bindErr != nil {
		return bindErr.SendResponse(c)
	}
	if err := coreValidator.Validate.Struct(&req); err != nil {
		return saveValidationFailed(c, req, err)
	}
	plan, err := h.plan(c)
	if err != nil {
		log.Printf("transaction settings plan read failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	}
	result, err := h.svc.SaveTransactionSettings(c.Context(), service.SaveInput{
		ClientID: who.ClientID, Request: req, Plan: plan, ActorID: who.UserID, ActorName: who.FullName,
	})
	var invalid *service.ValidationError
	switch {
	case errors.As(err, &invalid):
		out := make([]coreValidator.BindFieldError, 0, len(invalid.Errors))
		for _, e := range invalid.Errors {
			out = append(out, coreValidator.BindFieldError{Field: e.Field, Message: e.Message})
		}
		return coreEntity.UnprocessableEntity(msgValidation, out).SendResponse(c)
	case errors.Is(err, service.ErrCategoryNotFound):
		return coreEntity.NotFound("Not found").SendResponse(c)
	case err != nil:
		log.Printf("transaction settings save failed: %v", err)
		return coreEntity.InternalServerError(msgInternal).SendResponse(c)
	case !result.Changed:
		return coreHelper.Success(c, msgNoChanges, nil)
	}
	return coreHelper.Success(c, msgSaved, nil)
}

var changeFieldPath = regexp.MustCompile(`Changes\[(\d+)\](?:\.Fields\[(\d+)\])?(?:\.(\w+))?$`)

// saveValidationFailed answers 422 with one entry per struct-tag violation. An entry on a field
// reads `fields[<index>].value`, the index counting the fields of the whole payload in order (the
// same index the service uses); an entry on a change reads `changes[<i>].<name>`.
func saveValidationFailed(c fiber.Ctx, req dto.SaveSettingsRequest, err error) error {
	violations := fieldViolations(err)
	if len(violations) == 0 {
		return validationFailed(c, err)
	}
	out := make([]coreValidator.BindFieldError, 0, len(violations))
	for _, v := range violations {
		out = append(out, coreValidator.BindFieldError{Field: savePath(req, v.Namespace()), Message: "failed rule " + v.Tag()})
	}
	return coreEntity.UnprocessableEntity(msgValidation, out).SendResponse(c)
}

func savePath(req dto.SaveSettingsRequest, namespace string) string {
	m := changeFieldPath.FindStringSubmatch(namespace)
	if m == nil {
		return "changes"
	}
	ci, _ := strconv.Atoi(m[1])
	if m[2] != "" {
		fi, _ := strconv.Atoi(m[2])
		flat := fi
		for i := 0; i < ci && i < len(req.Changes); i++ {
			flat += len(req.Changes[i].Fields)
		}
		return "fields[" + strconv.Itoa(flat) + "].value"
	}
	name := strings.ToLower(m[3][:1]) + m[3][1:]
	return "changes[" + strconv.Itoa(ci) + "]." + name
}

type violation interface {
	Namespace() string
	Tag() string
}

// fieldViolations reads the per-field violations of a validator error (a slice of field errors);
// the validator package is not a direct dependency of this service, so it is read by method.
func fieldViolations(err error) []violation {
	v := reflect.ValueOf(err)
	if v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]violation, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		if fe, ok := v.Index(i).Interface().(violation); ok {
			out = append(out, fe)
		}
	}
	return out
}
