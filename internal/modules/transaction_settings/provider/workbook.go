package provider

import (
	"errors"
	"fmt"
	"slices"
	"time"

	coreHelper "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/helper"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/dto"
)

// workbookFallbackName is the file name stem the workbook helper uses when none is given.
const workbookFallbackName = "Transaction-Rules"

// Sheet headers of the export workbook, in the order the export service builds them.
var (
	rulesHeaders     = []string{"Category", "Transaction", "Field", "Value", "Unit", "Source"}
	overridesHeaders = []string{"Scope", "Target", "Category", "Transaction", "Field", "Value", "Unit", "Status", "Created By", "Created At"}
	infoHeaders      = []string{"Item", "Value"}
)

// sheetHeaders are the exact headers of each export sheet; a sheet outside this table is rejected.
var sheetHeaders = map[string][]string{
	"Rules":     rulesHeaders,
	"Overrides": overridesHeaders,
	"Info":      infoHeaders,
}

// WorkbookRenderer renders the export sheets into one XLSX workbook with be-core-utils/helper.ExportV2.
type WorkbookRenderer struct{}

// NewWorkbookRenderer returns the renderer of the export workbook.
func NewWorkbookRenderer() WorkbookRenderer {
	return WorkbookRenderer{}
}

// Render writes the sheets, in order, into one workbook. Each sheet must be one of the three export sheets with
// its exact headers, and every row must be as wide as its headers.
func (WorkbookRenderer) Render(sheets []dto.Sheet) ([]byte, error) {
	if len(sheets) == 0 {
		return nil, errors.New("render workbook: no sheets")
	}
	built := make([]coreHelper.ExportV2Sheet, 0, len(sheets))
	for _, sheet := range sheets {
		exportSheet, err := buildSheet(sheet)
		if err != nil {
			return nil, err
		}
		built = append(built, exportSheet)
	}
	// Cells are already rendered text or numbers, so no tenant date column is used; ExportV2 still needs a
	// valid format and zone.
	result, err := coreHelper.ExportV2(nil, coreHelper.ExportV2Param{
		Format:      "xlsx",
		DefaultName: workbookFallbackName,
		Tenant:      coreHelper.TenantFormat{DateFormat: "dd/MM/yy", TimeFormat: "HH:mm", Location: time.UTC},
		Sheets:      built,
	})
	if err != nil {
		return nil, fmt.Errorf("render workbook: %w", err)
	}
	return result.Data, nil
}

// buildSheet checks the sheet headers and row widths, then maps it onto an ExportV2 sheet whose column keys are
// the headers; Value columns are numeric when the cell is a number and text otherwise.
func buildSheet(sheet dto.Sheet) (coreHelper.ExportV2Sheet, error) {
	headers, ok := sheetHeaders[sheet.Name]
	if !ok {
		return coreHelper.ExportV2Sheet{}, fmt.Errorf("unknown export sheet %q", sheet.Name)
	}
	if !slices.Equal(sheet.Headers, headers) {
		return coreHelper.ExportV2Sheet{}, fmt.Errorf("export sheet %q has unexpected headers", sheet.Name)
	}
	columns := make([]coreHelper.ExportColumn, len(headers))
	for i, h := range headers {
		columns[i] = coreHelper.ExportColumn{Key: h, Header: h}
	}
	rows := make([]map[string]any, 0, len(sheet.Rows))
	for i, cells := range sheet.Rows {
		if len(cells) != len(headers) {
			return coreHelper.ExportV2Sheet{}, fmt.Errorf("export sheet %q row %d has %d cells, want %d", sheet.Name, i, len(cells), len(headers))
		}
		row := make(map[string]any, len(headers))
		for j, h := range headers {
			row[h] = cells[j]
		}
		rows = append(rows, row)
	}
	return coreHelper.ExportV2Sheet{
		Name:    sheet.Name,
		Columns: columns,
		Types:   map[string]coreHelper.CellType{"Value": coreHelper.CellNumber},
		Rows:    rows,
	}, nil
}
