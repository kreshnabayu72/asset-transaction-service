package provider

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	analyticsTemporal "gitea.tagsamurai.local/BETS-V2/service-contracts/services/analytics_reporting/temporal"
	temporalclient "go.temporal.io/sdk/client"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// startTimeout bounds the workflow start; the write itself is asynchronous and best-effort.
const startTimeout = 5 * time.Second

// errIncompleteLog reports a change-log row missing a field analytics-reporting requires.
var errIncompleteLog = errors.New("change log row is incomplete")

// WorkflowStarter starts a workflow on Temporal; the Temporal client implements it.
type WorkflowStarter interface {
	ExecuteWorkflow(ctx context.Context, options temporalclient.StartWorkflowOptions, workflow any, args ...any) (temporalclient.WorkflowRun, error)
}

// ChangeLogDispatcher starts analytics-reporting RecordChangeLogsWorkflow for the transaction settings change logs.
// Changelog documents and Activity Log entries share that one workflow.
type ChangeLogDispatcher struct {
	starter   WorkflowStarter
	taskQueue string
}

// NewChangeLogDispatcher binds the dispatcher to the analytics-reporting task queue.
func NewChangeLogDispatcher(starter WorkflowStarter, taskQueue string) *ChangeLogDispatcher {
	return &ChangeLogDispatcher{starter: starter, taskQueue: taskQueue}
}

// auditFields is the content of one analytics ChangeLog row, shared by both entry shapes.
type auditFields struct {
	logTable          string
	object            string
	objectName        string
	action            string
	field             string
	oldValue          string
	newValue          string
	createdByID       int64
	createdBySnapshot string
}

// DispatchChangeLogs starts one workflow for the Changelog documents of one save. Every row is checked before
// anything starts, so an incomplete row never reaches the analytics retry loop.
func (d *ChangeLogDispatcher) DispatchChangeLogs(ctx context.Context, clientID string, entries []service.ChangeLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]analyticsTemporal.ChangeLog, 0, len(entries))
	for _, e := range entries {
		row, err := changeLogRow(clientID, auditFields{
			logTable:          e.LogTable,
			object:            e.ObjectPath,
			objectName:        e.Entity,
			action:            e.Action,
			field:             e.Field,
			oldValue:          e.OldValue,
			newValue:          e.NewValue,
			createdByID:       e.CreatedByID,
			createdBySnapshot: e.ModifiedBy,
		})
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	return d.start(ctx, clientID, rows)
}

// DispatchActivityEntries starts one workflow for the structured Activity Log entries of one request.
func (d *ChangeLogDispatcher) DispatchActivityEntries(ctx context.Context, clientID string, entries []service.ActivityLogEntry) error {
	if len(entries) == 0 {
		return nil
	}
	rows := make([]analyticsTemporal.ChangeLog, 0, len(entries))
	for _, e := range entries {
		row, err := changeLogRow(clientID, auditFields{
			logTable:          e.LogTable,
			object:            e.Object,
			objectName:        e.ObjectName,
			action:            e.Action,
			field:             e.Field,
			oldValue:          e.OldValue,
			newValue:          e.NewValue,
			createdByID:       e.ActorID,
			createdBySnapshot: e.ActorName,
		})
		if err != nil {
			return err
		}
		rows = append(rows, row)
	}
	return d.start(ctx, clientID, rows)
}

// changeLogRow maps one entry to an analytics ChangeLog of product fixed_asset. LogTable and Object default to the
// settings label; the action, the actor id and the actor name are required.
func changeLogRow(clientID string, f auditFields) (analyticsTemporal.ChangeLog, error) {
	switch {
	case f.action == "":
		return analyticsTemporal.ChangeLog{}, fmt.Errorf("%w: action", errIncompleteLog)
	case f.createdByID == 0:
		return analyticsTemporal.ChangeLog{}, fmt.Errorf("%w: createdById", errIncompleteLog)
	case f.createdBySnapshot == "":
		return analyticsTemporal.ChangeLog{}, fmt.Errorf("%w: createdBySnapshot", errIncompleteLog)
	}
	product := analyticsTemporal.ProductFixedAsset
	return analyticsTemporal.ChangeLog{
		Action:            f.action,
		Field:             optional(f.field),
		OldValue:          optional(f.oldValue),
		NewValue:          optional(f.newValue),
		LogTable:          cmp.Or(f.logTable, service.LogTableTransactionSettings),
		CreatedByID:       strconv.FormatInt(f.createdByID, 10),
		CreatedBySnapshot: f.createdBySnapshot,
		EntityID:          clientID,
		EntityType:        analyticsTemporal.EntityTypeClient,
		Product:           &product,
		Object:            cmp.Or(f.object, service.LogTableTransactionSettings),
		ObjectName:        optional(f.objectName),
		ScopeGroupIDs:     []string{},
	}, nil
}

// optional returns nil for an empty text, so analytics stores a null rather than an empty value.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// start records the rows in one RecordChangeLogsWorkflow run on the analytics-reporting queue.
func (d *ChangeLogDispatcher) start(ctx context.Context, clientID string, rows []analyticsTemporal.ChangeLog) error {
	startCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), startTimeout)
	defer cancel()
	workflowID := fmt.Sprintf("transaction-settings-changelog-%s-%d", clientID, time.Now().UnixNano())
	options := temporalclient.StartWorkflowOptions{ID: workflowID, TaskQueue: d.taskQueue}
	param := analyticsTemporal.RecordChangeLogsParam{ClientID: clientID, ChangeLogs: rows}
	if _, err := d.starter.ExecuteWorkflow(startCtx, options, analyticsTemporal.RecordChangeLogsWorkflowName, param); err != nil {
		return fmt.Errorf("start %s: %w", analyticsTemporal.RecordChangeLogsWorkflowName, err)
	}
	return nil
}
