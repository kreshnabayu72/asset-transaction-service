package service

import (
	"encoding/json"
	"math"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
)

// fieldIndex maps transaction + field key to a stored row.
type fieldIndex map[string]model.TransactionRuleField

func indexKey(tx, key string) string { return tx + "\x00" + key }

func newFieldIndex(rows []model.TransactionRuleField) fieldIndex {
	idx := make(fieldIndex, len(rows))
	for _, r := range rows {
		idx[indexKey(r.TransactionType, r.FieldKey)] = r
	}
	return idx
}

// own returns the decoded value of an own row.
func (idx fieldIndex) own(tx, key string) (any, bool) {
	row, ok := idx[indexKey(tx, key)]
	if !ok || row.Mode != constants.ModeOwn || row.Value == nil {
		return nil, false
	}
	return decodeStored(*row.Value), true
}

// companyValue is the Company default value of a field: the stored own value, else the catalogue default.
func companyValue(company fieldIndex, tx string, f constants.FieldDef) any {
	if v, ok := company.own(tx, f.Key); ok {
		return v
	}
	return f.Default
}

// decodeStored turns a stored JSON value into a wire value: integral numbers become int,
// string arrays become []string.
func decodeStored(raw json.RawMessage) any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	return normalizeJSON(v)
}

func normalizeJSON(v any) any {
	switch val := v.(type) {
	case float64:
		if val == math.Trunc(val) && math.Abs(val) < 1<<53 {
			return int(val)
		}
		return val
	case []any:
		out := make([]string, 0, len(val))
		for _, item := range val {
			s, ok := item.(string)
			if !ok {
				return val
			}
			out = append(out, s)
		}
		return out
	default:
		return v
	}
}
