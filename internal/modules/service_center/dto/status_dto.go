package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
)

var ErrMalformedStatusRequest = errors.New("malformed JSON status request")

type ChangeStatusRequest struct {
	IsActive bool
	Version  int32
}

type StatusFieldError struct {
	Field   string
	Message string
}

func ParseChangeStatusRequest(body []byte) (ChangeStatusRequest, []StatusFieldError, error) {
	if !json.Valid(body) {
		return ChangeStatusRequest{}, nil, ErrMalformedStatusRequest
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return ChangeStatusRequest{}, []StatusFieldError{{Field: "body", Message: "must be a JSON object"}}, nil
	}

	var fieldErrors []StatusFieldError
	unknownFields := make([]string, 0)
	for field := range fields {
		if field != "isActive" && field != "version" {
			unknownFields = append(unknownFields, field)
		}
	}
	sort.Strings(unknownFields)
	for _, field := range unknownFields {
		fieldErrors = append(fieldErrors, StatusFieldError{Field: field, Message: "unknown field"})
	}

	var request ChangeStatusRequest
	activeRaw, ok := fields["isActive"]
	if !ok {
		fieldErrors = append(fieldErrors, StatusFieldError{Field: "isActive", Message: "isActive is required"})
	} else if isJSONNull(activeRaw) || json.Unmarshal(activeRaw, &request.IsActive) != nil {
		fieldErrors = append(fieldErrors, StatusFieldError{Field: "isActive", Message: "must be a boolean"})
	}

	versionRaw, ok := fields["version"]
	if !ok {
		fieldErrors = append(fieldErrors, StatusFieldError{Field: "version", Message: "Version is required"})
	} else if isJSONNull(versionRaw) {
		fieldErrors = append(fieldErrors, StatusFieldError{Field: "version", Message: "must be an integer"})
	} else {
		version, err := strconv.ParseInt(string(bytes.TrimSpace(versionRaw)), 10, 32)
		if err != nil {
			fieldErrors = append(fieldErrors, StatusFieldError{Field: "version", Message: "must be an integer"})
		} else if version < 1 {
			fieldErrors = append(fieldErrors, StatusFieldError{Field: "version", Message: "must be at least 1"})
		} else {
			request.Version = int32(version)
		}
	}

	return request, fieldErrors, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}
