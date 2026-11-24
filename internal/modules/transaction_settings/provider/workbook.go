package provider

import (
	"fmt"
	"slices"

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

// Typed export rows; their export tags are the sheet headers above.
type rulesRow struct {
	Category    any `export:"Category"`
	Transaction any `export:"Transaction"`
	Field       any `export:"Field"`
	Value       any `export:"Value"`
	Unit        any `export:"Unit"`
	Source      any `export:"Source"`
}

type overridesRow struct {
	Scope       any `export:"Scope"`
	Target      any `export:"Target"`
	Category    any `export:"Category"`
	Transaction any `export:"Transaction"`
	Field       any `export:"Field"`
	Value       any `export:"Value"`
	Unit        any `export:"Unit"`
	Status      any `export:"Status"`
	CreatedBy   any `export:"Created By"`
	CreatedAt   any `export:"Created At"`
}

type infoRow struct {
	Item  any `export:"Item"`
	Value any `export:"Value"`
}

// WorkbookRenderer renders the export sheets into one XLSX workbook with be-core-utils/helper.ExportSheets.
type WorkbookRenderer struct{}

// NewWorkbookRenderer returns the renderer of the export workbook.
func NewWorkbookRenderer() WorkbookRenderer {
	return WorkbookRenderer{}
}

// Render writes the sheets, in order, into one workbook. Each sheet must be one of the three export sheets with
// its exact headers, and every row must be as wide as its headers.
func (WorkbookRenderer) Render(sheets []dto.Sheet) ([]byte, error) {
	built := make([]coreHelper.ExportSheet, 0, len(sheets))
	for _, sheet := range sheets {
		exportSheet, err := buildSheet(sheet)
		if err != nil {
			return nil, err
		}
		built = append(built, exportSheet)
	}
	result, err := coreHelper.ExportSheets(built, coreHelper.ExportSheetsParam{FallbackName: workbookFallbackName})
	if err != nil {
		return nil, fmt.Errorf("render workbook: %w", err)
	}
	return result.Data, nil
}

// buildSheet maps one service sheet onto its typed export rows.
func buildSheet(sheet dto.Sheet) (coreHelper.ExportSheet, error) {
	switch sheet.Name {
	case "Rules":
		rows, err := sheetRows(sheet, rulesHeaders, func(c []any) rulesRow {
			return rulesRow{Category: c[0], Transaction: c[1], Field: c[2], Value: c[3], Unit: c[4], Source: c[5]}
		})
		if err != nil {
			return coreHelper.ExportSheet{}, err
		}
		return coreHelper.NewExportSheet(sheet.Name, rows), nil
	case "Overrides":
		rows, err := sheetRows(sheet, overridesHeaders, func(c []any) overridesRow {
			return overridesRow{
				Scope: c[0], Target: c[1], Category: c[2], Transaction: c[3], Field: c[4],
				Value: c[5], Unit: c[6], Status: c[7], CreatedBy: c[8], CreatedAt: c[9],
			}
		})
		if err != nil {
			return coreHelper.ExportSheet{}, err
		}
		return coreHelper.NewExportSheet(sheet.Name, rows), nil
	case "Info":
		rows, err := sheetRows(sheet, infoHeaders, func(c []any) infoRow {
			return infoRow{Item: c[0], Value: c[1]}
		})
		if err != nil {
			return coreHelper.ExportSheet{}, err
		}
		return coreHelper.NewExportSheet(sheet.Name, rows), nil
	}
	return coreHelper.ExportSheet{}, fmt.Errorf("unknown export sheet %q", sheet.Name)
}

// sheetRows checks the sheet headers and row widths, then maps each row with build.
func sheetRows[T any](sheet dto.Sheet, headers []string, build func([]any) T) ([]T, error) {
	if !slices.Equal(sheet.Headers, headers) {
		return nil, fmt.Errorf("export sheet %q has unexpected headers", sheet.Name)
	}
	rows := make([]T, 0, len(sheet.Rows))
	for i, cells := range sheet.Rows {
		if len(cells) != len(headers) {
			return nil, fmt.Errorf("export sheet %q row %d has %d cells, want %d", sheet.Name, i, len(cells), len(headers))
		}
		rows = append(rows, build(cells))
	}
	return rows, nil
}
