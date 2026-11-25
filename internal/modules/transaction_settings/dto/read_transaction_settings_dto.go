// Package dto holds the request and response shapes of the transaction settings API.
package dto

// FeatureSuite is the plan's Feature Suite level per transaction family (none, basic, full).
type FeatureSuite struct {
	Borrowing  string `json:"borrowing"`
	Assignment string `json:"assignment"`
	Transfer   string `json:"transfer"`
	Disposal   string `json:"disposal"`
}

// Addons is the status of the Audit and Maintenance addons of the subscription.
type Addons struct {
	Audit       bool `json:"audit"`
	Maintenance bool `json:"maintenance"`
}

// SettingField is one field of one transaction at the read node.
type SettingField struct {
	Key            string  `json:"key"`
	Mode           string  `json:"mode"`
	Value          any     `json:"value"`
	InheritedValue any     `json:"inheritedValue"`
	Source         *string `json:"source"`
	DefaultValue   any     `json:"defaultValue"`
	Gated          bool    `json:"gated"`
}

// TransactionSettings is the field list of one transaction.
type TransactionSettings struct {
	Transaction string         `json:"transaction"`
	Available   bool           `json:"available"`
	NoApprover  bool           `json:"noApprover"`
	Fields      []SettingField `json:"fields"`
}

// ReadSettingsData is the data of GET /transaction-settings.
type ReadSettingsData struct {
	SettingsVersion           int                   `json:"settingsVersion"`
	CategoryID                *string               `json:"categoryId"`
	ApprovalWorkflowAvailable bool                  `json:"approvalWorkflowAvailable"`
	FeatureSuite              FeatureSuite          `json:"featureSuite"`
	Addons                    Addons                `json:"addons"`
	Transactions              []TransactionSettings `json:"transactions"`
}
