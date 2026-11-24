package provider

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

func TestWorkbookRendersTheThreeExportSheets(t *testing.T) {
	data, err := NewWorkbookRenderer().Render([]dto.Sheet{
		{Name: "Rules", Headers: rulesHeaders, Rows: [][]any{{"All categories", "Borrow", "Max duration", 14, "days", "Company default"}}},
		{Name: "Overrides", Headers: overridesHeaders, Rows: [][]any{{"Group", "Root", "All categories", "Borrow", "Max duration", 7, "days", "Applied", "Rina Wulandari", "2026-11-24"}}},
		{Name: "Info", Headers: infoHeaders, Rows: [][]any{{"Settings version", 3}}},
	})
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("PK")), "an XLSX workbook is a zip container")
}

func TestWorkbookRejectsSheetsOutsideTheContract(t *testing.T) {
	renderer := NewWorkbookRenderer()

	_, err := renderer.Render([]dto.Sheet{{Name: "Unknown", Headers: infoHeaders}})
	require.ErrorContains(t, err, "unknown export sheet")

	_, err = renderer.Render([]dto.Sheet{{Name: "Info", Headers: []string{"Item"}}})
	require.ErrorContains(t, err, "unexpected headers")

	_, err = renderer.Render([]dto.Sheet{{Name: "Info", Headers: infoHeaders, Rows: [][]any{{"only one"}}}})
	require.ErrorContains(t, err, "row 0 has 1 cells")

	_, err = renderer.Render([]dto.Sheet{{Name: "Rules", Headers: infoHeaders}})
	require.ErrorContains(t, err, "unexpected headers")

	_, err = renderer.Render([]dto.Sheet{{Name: "Overrides", Headers: rulesHeaders}})
	require.ErrorContains(t, err, "unexpected headers")

	_, err = renderer.Render(nil)
	require.ErrorContains(t, err, "render workbook")

	_, err = renderer.Render([]dto.Sheet{
		{Name: "Info", Headers: infoHeaders},
		{Name: "Info", Headers: infoHeaders},
	})
	require.ErrorContains(t, err, "render workbook")
}

// exportTags lists the export tags of a row struct in field order.
func exportTags(v any) []string {
	typ := reflect.TypeOf(v)
	tags := make([]string, 0, typ.NumField())
	for i := range typ.NumField() {
		tags = append(tags, typ.Field(i).Tag.Get("export"))
	}
	return tags
}

func TestExportRowTagsMatchTheSheetHeaders(t *testing.T) {
	require.Equal(t, rulesHeaders, exportTags(rulesRow{}))
	require.Equal(t, overridesHeaders, exportTags(overridesRow{}))
	require.Equal(t, infoHeaders, exportTags(infoRow{}))
}
