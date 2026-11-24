package dto

// ExportRulesRequest is the body of POST /transaction-settings/export; a null categoryId selects All categories.
type ExportRulesRequest struct {
	CategoryID *string `json:"categoryId"`
}

// Sheet is one worksheet of the export workbook: header row then data rows.
type Sheet struct {
	Name    string
	Headers []string
	Rows    [][]any
}

// ExportFile is the rendered workbook returned to the client.
type ExportFile struct {
	FileName string
	Data     []byte
}
