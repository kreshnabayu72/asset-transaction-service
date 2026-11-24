package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

func TestOverrideValueHelpers(t *testing.T) {
	require.Empty(t, decodeOverrideFields(json.RawMessage(`[1]`)))
	require.Equal(t, map[string]any{"max_duration": 3}, decodeOverrideFields(json.RawMessage(`{"max_duration":3}`)))

	// A unit that is not a string falls back to days.
	company := newFieldIndex([]model.TransactionRuleField{storedRow(nil, "borrow", "duration_unit", `5`)})
	require.Equal(t, "days", overrideUnit(nil, nil, company))
	require.Equal(t, "hours", overrideUnit([]map[string]any{{"duration_unit": "hours"}}, nil, company))

	require.True(t, isOverrideTransaction("assign"))
	require.False(t, isOverrideTransaction("audit"))
	id, ok := parseSnowflake("0042")
	require.False(t, ok)
	require.Zero(t, id)
}
