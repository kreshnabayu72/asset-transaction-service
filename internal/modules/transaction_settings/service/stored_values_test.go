package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
)

func raw(s string) *json.RawMessage {
	m := json.RawMessage(s)
	return &m
}

func TestDecodeStored(t *testing.T) {
	require.Equal(t, 14, decodeStored(json.RawMessage(`14`)))
	require.Equal(t, 1.5, decodeStored(json.RawMessage(`1.5`)))
	require.Equal(t, true, decodeStored(json.RawMessage(`true`)))
	require.Equal(t, "days", decodeStored(json.RawMessage(`"days"`)))
	require.Equal(t, []string{"sell", "scrap"}, decodeStored(json.RawMessage(`["sell","scrap"]`)))
	require.Equal(t, []any{1.0, "a"}, decodeStored(json.RawMessage(`[1,"a"]`)))
	require.Nil(t, decodeStored(json.RawMessage(`{bad`)))
}

func TestFieldIndexOwnAndCompanyValue(t *testing.T) {
	idx := newFieldIndex([]model.TransactionRuleField{
		{TransactionType: "borrow", FieldKey: "max_duration", Mode: "own", Value: raw("21")},
		{TransactionType: "borrow", FieldKey: "max_extensions", Mode: "inherit"},
	})
	v, ok := idx.own("borrow", "max_duration")
	require.True(t, ok)
	require.Equal(t, 21, v)
	_, ok = idx.own("borrow", "max_extensions")
	require.False(t, ok)
	_, ok = idx.own("borrow", "missing")
	require.False(t, ok)

	require.Equal(t, 21, companyValue(idx, "borrow", constants.FieldDef{Key: "max_duration", Default: 14}))
	require.Equal(t, 1, companyValue(idx, "borrow", constants.FieldDef{Key: "max_extensions", Default: 1}))
}
