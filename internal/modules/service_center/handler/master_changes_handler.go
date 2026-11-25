package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	tsMiddleware "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/middleware"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"
	coreValidator "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/validator"
	"github.com/gofiber/fiber/v3"
)

type MasterChangesService interface {
	ApplyMasterChanges(ctx context.Context, clientID string, change dto.MasterChangesRequest) (int64, error)
}

type MasterChangesHandler struct {
	service MasterChangesService
}

func NewMasterChangesHandler(service MasterChangesService) *MasterChangesHandler {
	return &MasterChangesHandler{service: service}
}

func (h *MasterChangesHandler) MasterChanges(c fiber.Ctx) error {
	change, fieldErrors, bindErr := parseMasterChangesBody(c.Body())
	if bindErr != nil {
		return coreEntity.BadRequest("Invalid request body", coreValidator.ParseBindError(bindErr)).SendResponse(c)
	}
	if len(fieldErrors) > 0 {
		return coreEntity.UnprocessableEntity("Invalid request body", fieldErrors).SendResponse(c)
	}

	clientID, err := tsMiddleware.ClientID(c)
	if err != nil {
		return coreEntity.BadRequest("Invalid client id").SendResponse(c)
	}
	affected, err := h.service.ApplyMasterChanges(c.Context(), clientID, change)
	if err != nil {
		return coreEntity.ToHttpError(err).SendResponse(c)
	}
	return coreHelper.Success(c, "Success", dto.MasterChangesData{Affected: affected})
}

func parseMasterChangesBody(body []byte) (dto.MasterChangesRequest, []coreValidator.BindFieldError, error) {
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawFields); err != nil {
		return dto.MasterChangesRequest{}, nil, err
	}
	if rawFields == nil {
		return dto.MasterChangesRequest{}, nil, &json.UnmarshalTypeError{
			Value: "null",
			Type:  reflect.TypeOf(map[string]json.RawMessage{}),
		}
	}

	var request dto.MasterChangesRequest
	var fieldErrors []coreValidator.BindFieldError
	request.Type, _ = parseStringField(rawFields, "type", &fieldErrors)
	request.Action, _ = parseStringField(rawFields, "action", &fieldErrors)
	request.IDs, _ = parseSnowflakeIDs(rawFields, "ids", &fieldErrors)
	request.MergedIntoID, _ = parseMergedIntoID(rawFields, &fieldErrors)
	if len(fieldErrors) == 0 {
		for _, validationError := range request.Validate() {
			fieldErrors = append(fieldErrors, coreValidator.BindFieldError{
				Field: validationError.Field, Message: validationError.Message,
			})
		}
	}
	return request, fieldErrors, nil
}

func parseStringField(
	fields map[string]json.RawMessage,
	name string,
	fieldErrors *[]coreValidator.BindFieldError,
) (string, bool) {
	rawValue, ok := fields[name]
	if !ok {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{Field: name, Message: "is required"})
		return "", true
	}
	trimmed := bytes.TrimSpace(rawValue)
	var value string
	if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(rawValue, &value) != nil {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{Field: name, Message: "must be a string"})
		return "", true
	}
	return value, false
}

func parseSnowflakeIDs(
	fields map[string]json.RawMessage,
	name string,
	fieldErrors *[]coreValidator.BindFieldError,
) ([]int64, bool) {
	rawValue, ok := fields[name]
	if !ok {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{Field: name, Message: "is required"})
		return nil, true
	}
	var values []json.RawMessage
	if err := json.Unmarshal(rawValue, &values); err != nil || values == nil {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{Field: name, Message: "must be an array of Snowflake ID strings"})
		return nil, true
	}
	ids := make([]int64, 0, len(values))
	invalid := false
	for i, rawID := range values {
		id, err := parseSnowflakeID(rawID)
		if err != nil {
			*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{
				Field: name + "[" + strconv.Itoa(i) + "]", Message: "must be a positive decimal Snowflake ID string",
			})
			invalid = true
			continue
		}
		ids = append(ids, id)
	}
	return ids, invalid
}

func parseMergedIntoID(
	fields map[string]json.RawMessage,
	fieldErrors *[]coreValidator.BindFieldError,
) (*int64, bool) {
	rawValue, ok := fields["mergedIntoId"]
	if !ok {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{
			Field: "mergedIntoId", Message: "is required; use null when action is not merged",
		})
		return nil, true
	}
	if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
		return nil, false
	}
	id, err := parseSnowflakeID(rawValue)
	if err != nil {
		*fieldErrors = append(*fieldErrors, coreValidator.BindFieldError{
			Field: "mergedIntoId", Message: "must be null or a positive decimal Snowflake ID string",
		})
		return nil, true
	}
	return &id, false
}

func parseSnowflakeID(rawValue json.RawMessage) (int64, error) {
	trimmed := bytes.TrimSpace(rawValue)
	var value string
	if len(trimmed) == 0 || trimmed[0] != '"' || json.Unmarshal(rawValue, &value) != nil || value == "" {
		return 0, strconv.ErrSyntax
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, strconv.ErrRange
	}
	return id, nil
}
