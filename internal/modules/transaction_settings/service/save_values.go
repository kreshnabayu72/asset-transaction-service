package service

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
)

// Validation messages of a Save (LLD PATCH step 3).
const (
	msgValueRequired       = "You must enter a value"
	msgMinOne              = "Value must be at least 1"
	msgMinZero             = "Value must be at least 0"
	msgWholeNumber         = "Value must be a whole number"
	msgBoolean             = "Value must be true or false"
	msgInvalidOption       = "Value is not a valid option"
	msgNoDisposalType      = "At least one disposal type must be selected"
	msgNoAssigneeType      = "At least one assignee type must be enabled"
	msgFieldGated          = "This field is not available on the current plan"
	msgUnknownField        = "Unknown field for this transaction"
	msgInheritNotAllowed   = "Inherit is not allowed here"
	msgOwnNotAllowed       = "This field cannot be set at a Category"
	msgInvalidCategoryID   = "categoryId must be a positive numeric id"
	msgAllCategoriesOnlyTx = "This transaction only exists at All categories"
)

type valueKind int

const (
	kindInt valueKind = iota
	kindBool
	kindChoice
	kindList
)

// fieldRule is the value rule of one field: its type, integer minimum and choice tokens.
type fieldRule struct {
	kind    valueKind
	min     int
	choices []string
}

// minOneFields are the numeric fields with minimum 1; every other numeric field has minimum 0.
var minOneFields = []string{
	constants.FieldMaxDuration, constants.FieldMaxAssetsPerBorrower, constants.FieldInTransitOverdueAfter,
	constants.FieldMinAssetAgeWarning, constants.FieldMaxDeviationFromBookValue,
	constants.FieldAutoApproveMaxDuration, constants.FieldTransactionTimeout,
}

// choiceTokens is the value catalogue of the choice fields (OpenAPI FieldValue).
var choiceTokens = map[string][]string{
	constants.FieldDurationUnit:              {constants.ValueDays, constants.ValueHours},
	constants.FieldAllowedBorrower:           {constants.ValueSameGroupOnly, constants.ValueSameDivisionOnly, constants.ValueEmployeeInAnyGroup},
	constants.FieldDestinationScope:          {constants.ValueWithinSameParentGroup, constants.ValueAnyGroup},
	constants.FieldChangeCostCenterWithGroup: {constants.ValueNever, constants.ValueAlways},
	constants.FieldApprovalType:              {constants.ValueAnyOneApprover, constants.ValueAllApprovers},
	constants.FieldApprovalScope:             {constants.ValueOriginOnly, constants.ValueDestinationOnly, constants.ValueBoth},
	constants.FieldDueDateReminders:          {constants.ValueRemindersD3D1Dday, constants.ValueRemindersD1Dday, constants.ValueRemindersDdayOnly},
}

// ruleFor derives the value rule of a field from its catalogue default.
func ruleFor(def constants.FieldDef) fieldRule {
	switch def.Default.(type) {
	case bool:
		return fieldRule{kind: kindBool}
	case []string:
		return fieldRule{kind: kindList, choices: constants.DisposalTypes}
	case string:
		return fieldRule{kind: kindChoice, choices: choiceTokens[def.Key]}
	}
	min := 0
	if slices.Contains(minOneFields, def.Key) {
		min = 1
	}
	return fieldRule{kind: kindInt, min: min}
}

// parseValue type-checks a raw JSON value against the rule. It returns the normalized value (int,
// bool, string or []string) or the validation message.
func parseValue(rule fieldRule, raw json.RawMessage) (any, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte(`""`)) {
		return nil, msgValueRequired
	}
	var v any
	if err := json.Unmarshal(trimmed, &v); err != nil {
		return nil, msgInvalidOption
	}
	switch rule.kind {
	case kindInt:
		return parseInt(rule, v)
	case kindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, msgBoolean
		}
		return b, ""
	case kindChoice:
		s, ok := v.(string)
		if !ok || !slices.Contains(rule.choices, s) {
			return nil, msgInvalidOption
		}
		return s, ""
	default:
		return parseList(rule, v)
	}
}

func parseInt(rule fieldRule, v any) (any, string) {
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) || math.Abs(f) >= 1<<53 {
		return nil, msgWholeNumber
	}
	if int(f) < rule.min {
		if rule.min == 1 {
			return nil, msgMinOne
		}
		return nil, msgMinZero
	}
	return int(f), ""
}

func parseList(rule fieldRule, v any) (any, string) {
	items, ok := v.([]any)
	if !ok {
		return nil, msgInvalidOption
	}
	if len(items) == 0 {
		return nil, msgNoDisposalType
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s, ok := item.(string)
		if !ok || !slices.Contains(rule.choices, s) || slices.Contains(out, s) {
			return nil, msgInvalidOption
		}
		out = append(out, s)
	}
	return out, ""
}

// sameValue compares two normalized values; lists compare as sets.
func sameValue(a, b any) bool {
	la, okA := a.([]string)
	lb, okB := b.([]string)
	if okA && okB {
		ca, cb := slices.Clone(la), slices.Clone(lb)
		sort.Strings(ca)
		sort.Strings(cb)
		return slices.Equal(ca, cb)
	}
	return reflect.DeepEqual(a, b)
}

// optionOrder returns the list value in catalogue option order.
func optionOrder(list []string) []string {
	out := make([]string, 0, len(list))
	for _, t := range constants.DisposalTypes {
		if slices.Contains(list, t) {
			out = append(out, t)
		}
	}
	return out
}

func itoa(n int) string { return strconv.Itoa(n) }
