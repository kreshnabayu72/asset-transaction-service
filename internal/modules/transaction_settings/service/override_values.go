package service

import (
	"encoding/json"
	"slices"
	"strconv"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
)

// Validation messages of the override operations (LLD create step 3).
const (
	msgPickScope      = "You must pick a scope"
	msgPickGroup      = "You must pick a group"
	msgPickAssetName  = "You must pick an asset name"
	msgPickAsset      = "You must pick an asset"
	msgPickField      = "You must pick a field"
	msgPickValue      = "You must pick a value"
	msgDuplicateField = "Field already added"
	msgNotLeaf        = "Category must be a leaf Category"
	msgInvalidID      = "must be a positive numeric id"
	msgInvalidChoice  = "Value is not a valid option"
	msgInvalidRange   = "from must not be after to"
	msgPickColumns    = "columns is required"
	msgPickIDs        = "ids must hold at least one id"
)

// parseSnowflake parses a decimal positive Snowflake id.
func parseSnowflake(raw string) (int64, bool) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || raw != strconv.FormatInt(id, 10) {
		return 0, false
	}
	return id, true
}

func isOverrideTransaction(tx string) bool {
	for _, t := range constants.OverrideTransactions {
		if t == tx {
			return true
		}
	}
	return false
}

func isScope(scope string) bool {
	_, ok := constants.ScopeLabels[scope]
	return ok
}

// overridable reports whether an override may set the field: every field of the transaction except
// the fields reserved for All categories and the multi-select field (LLD create step 3).
func overridable(def constants.FieldDef) bool {
	return !def.AllCategoriesOnly && ruleFor(def).kind != kindList
}

// decodeOverrideFields turns the stored {fieldKey: value} object into wire values.
func decodeOverrideFields(raw json.RawMessage) map[string]any {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(obj))
	for k, v := range obj {
		out[k] = decodeStored(v)
	}
	return out
}

// layeredValue is the effective value of a field for a chain of override layers (closest first),
// then the Category node's own value, then the Company default.
func layeredValue(tx string, def constants.FieldDef, layers []map[string]any, cat, company fieldIndex) any {
	for _, layer := range layers {
		if v, ok := layer[def.Key]; ok {
			return v
		}
	}
	if cat != nil {
		if v, ok := cat.own(tx, def.Key); ok {
			return v
		}
	}
	return companyValue(company, tx, def)
}

// assignState says whether Allow Assign is Deny and both Allowed assignee fields are OFF.
func assignState(layers []map[string]any, cat, company fieldIndex) (deny, bothOff bool) {
	tx := constants.TransactionAssign
	get := func(key string) any {
		def, _ := findDef(tx, key)
		return layeredValue(tx, def, layers, cat, company)
	}
	return get(constants.FieldAllowAssign) == false,
		get(constants.FieldAllowedAssigneeEmployee) == false && get(constants.FieldAllowedAssigneeDivision) == false
}

// overriddenDefs returns the fields set by the override in display order (DQ-80 B): catalogue
// order, with Self service placed directly before Duration unit when both are set.
func overriddenDefs(tx string, fields map[string]any) []constants.FieldDef {
	var out []constants.FieldDef
	for _, def := range constants.Catalogue[tx] {
		if _, ok := fields[def.Key]; ok {
			out = append(out, def)
		}
	}
	self := slices.IndexFunc(out, func(d constants.FieldDef) bool { return d.Key == constants.FieldSelfService })
	unit := slices.IndexFunc(out, func(d constants.FieldDef) bool { return d.Key == constants.FieldDurationUnit })
	if unit >= 0 && self > unit {
		selfDef := out[self]
		out = slices.Insert(slices.Delete(out, self, self+1), unit, selfDef)
	}
	return out
}

// overrideUnit is the Duration unit stored with the override, else the one of the layers below.
func overrideUnit(layers []map[string]any, cat, company fieldIndex) string {
	def, _ := findDef(constants.TransactionBorrow, constants.FieldDurationUnit)
	unit, _ := layeredValue(constants.TransactionBorrow, def, layers, cat, company).(string)
	if unit == "" {
		return constants.ValueDays
	}
	return unit
}
