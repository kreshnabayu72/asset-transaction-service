package provider

import (
	"context"
	"errors"
	"testing"

	analyticsTemporal "gitea.tagsamurai.local/BETS-V2/service-contracts/services/analytics_reporting/temporal"
	"github.com/stretchr/testify/require"
	temporalclient "go.temporal.io/sdk/client"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/service"
)

// fakeStarter records the workflow starts of one dispatch.
type fakeStarter struct {
	workflows []any
	options   []temporalclient.StartWorkflowOptions
	params    []analyticsTemporal.RecordChangeLogsParam
	err       error
}

func (f *fakeStarter) ExecuteWorkflow(_ context.Context, options temporalclient.StartWorkflowOptions, workflow any, args ...any) (temporalclient.WorkflowRun, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.workflows = append(f.workflows, workflow)
	f.options = append(f.options, options)
	f.params = append(f.params, args[0].(analyticsTemporal.RecordChangeLogsParam))
	return nil, nil
}

func TestDispatchLogsMapsEveryRequiredField(t *testing.T) {
	starter := &fakeStarter{}
	d := NewChangeLogDispatcher(starter, "analytics-queue")
	err := d.DispatchLogs(context.Background(), testClientID, []service.ChangeLogEntry{{
		Action: "Edit", Entity: "Root > Leaf", Field: "Borrow (Max duration)", OldValue: "14 days", NewValue: "7 days",
		ModifiedBy: "Rina Wulandari", CreatedByID: 42,
	}}, nil)
	require.NoError(t, err)
	require.Len(t, starter.params, 1)
	require.Equal(t, analyticsTemporal.RecordChangeLogsWorkflowName, starter.workflows[0])
	require.Equal(t, "analytics-queue", starter.options[0].TaskQueue)
	require.Contains(t, starter.options[0].ID, "transaction-settings-changelog-client-1-")

	param := starter.params[0]
	require.Equal(t, testClientID, param.ClientID)
	require.Len(t, param.ChangeLogs, 1)
	row := param.ChangeLogs[0]
	require.Equal(t, "Edit", row.Action)
	require.Equal(t, "Borrow (Max duration)", *row.Field)
	require.Equal(t, "14 days", *row.OldValue)
	require.Equal(t, "7 days", *row.NewValue)
	require.Equal(t, service.LogTableTransactionSettings, row.LogTable)
	require.Equal(t, "42", row.CreatedByID)
	require.Equal(t, "Rina Wulandari", row.CreatedBySnapshot)
	require.Equal(t, testClientID, row.EntityID)
	require.Equal(t, analyticsTemporal.EntityTypeClient, row.EntityType)
	require.Equal(t, analyticsTemporal.ProductFixedAsset, *row.Product)
	require.Equal(t, service.LogTableTransactionSettings, row.Object)
	require.Equal(t, "Root > Leaf", *row.ObjectName)
	require.Nil(t, row.ObjectID)
	require.Equal(t, []string{}, row.ScopeGroupIDs)
}

func TestDispatchLogsKeepsEmptyValuesNull(t *testing.T) {
	starter := &fakeStarter{}
	err := NewChangeLogDispatcher(starter, "q").DispatchLogs(context.Background(), testClientID, []service.ChangeLogEntry{{
		Action: "Edit", ObjectPath: "Transaction Settings", LogTable: service.LogTableTransactionSettings,
		CreatedByID: 1, ModifiedBy: "Rina",
	}}, nil)
	require.NoError(t, err)
	row := starter.params[0].ChangeLogs[0]
	require.Nil(t, row.Field)
	require.Nil(t, row.OldValue)
	require.Nil(t, row.NewValue)
	require.Nil(t, row.ObjectName)
}

func TestDispatchLogsRefusesIncompleteRows(t *testing.T) {
	cases := map[string]service.ChangeLogEntry{
		"no action":     {CreatedByID: 1, ModifiedBy: "Rina"},
		"no created by": {Action: "Edit", ModifiedBy: "Rina"},
		"no snapshot":   {Action: "Edit", CreatedByID: 1},
	}
	for name, entry := range cases {
		t.Run(name, func(t *testing.T) {
			starter := &fakeStarter{}
			err := NewChangeLogDispatcher(starter, "q").DispatchLogs(context.Background(), testClientID, []service.ChangeLogEntry{entry}, nil)
			require.ErrorIs(t, err, errIncompleteLog)
			require.Empty(t, starter.params, "an incomplete row must not start a workflow")
		})
	}
}

func TestDispatchLogsWithoutEntriesStartsNothing(t *testing.T) {
	starter := &fakeStarter{}
	d := NewChangeLogDispatcher(starter, "q")
	require.NoError(t, d.DispatchLogs(context.Background(), testClientID, nil, nil))
	require.NoError(t, d.DispatchActivityEntries(context.Background(), testClientID, nil))
	require.Empty(t, starter.params)
}

func TestDispatchLogsWrapsStartFailure(t *testing.T) {
	starter := &fakeStarter{err: errors.New("temporal unreachable")}
	err := NewChangeLogDispatcher(starter, "q").DispatchLogs(context.Background(), testClientID, []service.ChangeLogEntry{{
		Action: "Edit", CreatedByID: 1, ModifiedBy: "Rina",
	}}, nil)
	require.ErrorContains(t, err, "temporal unreachable")
}

func TestDispatchLogsStartsOneWorkflowForBothKinds(t *testing.T) {
	starter := &fakeStarter{}
	err := NewChangeLogDispatcher(starter, "q").DispatchLogs(context.Background(), testClientID,
		[]service.ChangeLogEntry{{Action: "Edit", Entity: "All categories", Field: "Borrow (Max duration)", ModifiedBy: "Rina", CreatedByID: 7}},
		[]service.ActivityLogEntry{{ActorID: 7, ActorName: "Rina", Action: "Edit", Object: "Borrow", ObjectName: "All categories", Field: "Max duration"}})
	require.NoError(t, err)
	require.Len(t, starter.workflows, 1, "one save starts one RecordChangeLogsWorkflow run")
	require.Contains(t, starter.options[0].ID, "transaction-settings-changelog-client-1-")
	rows := starter.params[0].ChangeLogs
	require.Len(t, rows, 2)
	require.Equal(t, "Borrow (Max duration)", *rows[0].Field)
	require.Equal(t, "Borrow", rows[1].Object)
	require.Equal(t, "7", rows[1].CreatedByID)

	starter = &fakeStarter{}
	err = NewChangeLogDispatcher(starter, "q").DispatchLogs(context.Background(), testClientID, nil,
		[]service.ActivityLogEntry{{Action: "Edit", ActorName: "Rina"}})
	require.ErrorIs(t, err, errIncompleteLog)
	require.Empty(t, starter.params, "an incomplete row must not start a workflow")
}

func TestDispatchActivityEntriesUsesTheStructuredShape(t *testing.T) {
	starter := &fakeStarter{}
	d := NewChangeLogDispatcher(starter, "q")
	err := d.DispatchActivityEntries(context.Background(), testClientID, []service.ActivityLogEntry{
		{ActorID: 9, ActorName: "Rina Wulandari", Action: "Export", ObjectName: "All categories"},
		{ActorID: 9, ActorName: "Rina Wulandari", Action: "Create", Object: "Override", ObjectName: "Proyek", Field: "Max duration", OldValue: "a", NewValue: "b"},
	})
	require.NoError(t, err)
	rows := starter.params[0].ChangeLogs
	require.Len(t, rows, 2)
	require.Equal(t, service.LogTableTransactionSettings, rows[0].Object)
	require.Equal(t, "All categories", *rows[0].ObjectName)
	require.Nil(t, rows[0].Field)
	require.Equal(t, "9", rows[0].CreatedByID)
	require.Equal(t, "Override", rows[1].Object)
	require.Equal(t, "Max duration", *rows[1].Field)
	require.Equal(t, "a", *rows[1].OldValue)
	require.Equal(t, "b", *rows[1].NewValue)

	err = d.DispatchActivityEntries(context.Background(), testClientID, []service.ActivityLogEntry{{Action: "Export", ActorName: "Rina"}})
	require.ErrorIs(t, err, errIncompleteLog)
}
