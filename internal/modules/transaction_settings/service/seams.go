package service

import (
	"context"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

// SaveTx is the open database transaction of one settings save.
type SaveTx interface {
	UpsertFields(ctx context.Context, rows []model.TransactionRuleField) error
	BumpSettingsVersion(ctx context.Context, actorID int64, actorName string) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Close()
}

// CategoryInfo is what the Category provider answers for one id. Exists is false for an unknown, deleted or
// other-entity id: the provider does not tell them apart, so each is answered as not found.
type CategoryInfo struct {
	Exists bool
	Path   string // full Category path: the Changelog entity and the export label
	Leaf   bool   // a leaf Category in any state; GET, export and override create accept only leaves
}

// CategoryDirectory looks Categories up (settings-categories). Unset: every categoryId is accepted
// and logged by its id.
type CategoryDirectory interface {
	LookupCategory(ctx context.Context, clientID string, id int64) (CategoryInfo, error)
}

// LogTableTransactionSettings is the logTable of every change-log row of this module: the Changelog
// drawer "Transaction Settings" of the page.
const LogTableTransactionSettings = "Transaction Settings"

// Action values of the Changelog and Activity Log rows: the analytics-reporting ChangeLog vocabulary (Create,
// Edit, Delete). Export has no contract constant, so the export row carries its own verb.
const (
	actionCreate = "Create"
	actionEdit   = "Edit"
	actionDelete = "Delete"
	actionExport = "Export"
)

// ChangeLogEntry is one Changelog document of a changed field (LLD PATCH Logs Written). CreatedByID is the
// acting user's id; ModifiedBy is the name written at log time and LogTable is LogTableTransactionSettings.
type ChangeLogEntry struct {
	Action      string
	Entity      string
	ObjectPath  string
	Field       string
	OldValue    string
	NewValue    string
	ModifiedBy  string
	CreatedByID int64
	LogTable    string
}

// ChangeLogDispatcher sends Changelog documents after commit, best-effort. Unset: no dispatch.
type ChangeLogDispatcher interface {
	DispatchChangeLogs(ctx context.Context, clientID string, entries []ChangeLogEntry) error
}

// ActivityLogEntry is one structured Activity Log row (LLD Logs Written). Analytics-reporting stores the actor
// and the Action, Object, Name, Field and value columns, so an entry is not a sentence: Object is the
// transaction ("Transaction Rules" for an export), ObjectName the Category or the override target in its
// Category, and Field the field label. LogTable is LogTableTransactionSettings.
type ActivityLogEntry struct {
	ActorID    int64
	ActorName  string
	LogTable   string
	Action     string
	Object     string
	ObjectName string
	Field      string
	OldValue   string
	NewValue   string
}

// ActivityEntryDispatcher sends structured Activity Log entries after commit, best-effort. Unset: no dispatch.
type ActivityEntryDispatcher interface {
	DispatchActivityEntries(ctx context.Context, clientID string, entries []ActivityLogEntry) error
}

// WithCategoryDirectory sets the Category provider.
func (s *Service) WithCategoryDirectory(d CategoryDirectory) *Service { s.categories = d; return s }

// WithChangeLogDispatcher sets the Changelog dispatcher.
func (s *Service) WithChangeLogDispatcher(d ChangeLogDispatcher) *Service { s.changeLogs = d; return s }

// WithActivityEntryDispatcher sets the structured Activity Log dispatcher.
func (s *Service) WithActivityEntryDispatcher(d ActivityEntryDispatcher) *Service {
	s.activityEntries = d
	return s
}

// WithAssetDirectory sets the asset ownership provider of effective-rules.
func (s *Service) WithAssetDirectory(d AssetDirectory) *Service { s.assets = d; return s }

// WithDenyContactResolver sets the Group Manager / Total Control provider of effective-rules.
func (s *Service) WithDenyContactResolver(d DenyContactResolver) *Service {
	s.denyContacts = d
	return s
}
