package consumer

import (
	"context"
	"testing"

	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/stretchr/testify/require"
)

// fakeUpdater records calls without touching a database.
type fakeUpdater struct {
	calls  int
	fields int
	ovr    int
	err    error
}

func (f *fakeUpdater) UpdateModifiedByName(_ context.Context, _ coreService.PostgreSQLServicer, _ int64, _ string) error {
	f.calls++
	return f.err
}

func (f *fakeUpdater) UpdateRuleFieldsModifiedByName(_ context.Context, _ coreService.PostgreSQLServicer, _ int64, _ string) error {
	f.fields++
	return nil
}

func (f *fakeUpdater) UpdateRuleOverridesCreatedByName(_ context.Context, _ coreService.PostgreSQLServicer, _ int64, _ string) error {
	f.ovr++
	return nil
}

func TestApplyRenameUpdatesAllTables(t *testing.T) {
	updater := &fakeUpdater{}
	c := &UserRenamedConsumer{updater: updater}

	require.NoError(t, c.applyRename(context.Background(), nil, 42, "Ada"))
	require.Equal(t, []int{1, 1, 1}, []int{updater.calls, updater.fields, updater.ovr})
}

func TestApplyRenameStopsOnError(t *testing.T) {
	updater := &fakeUpdater{err: context.DeadlineExceeded}
	c := &UserRenamedConsumer{updater: updater}

	require.ErrorIs(t, c.applyRename(context.Background(), nil, 42, "Ada"), context.DeadlineExceeded)
	require.Zero(t, updater.fields+updater.ovr)
}

// Malformed events are committed and dropped before any database access, so no pool is needed.
func TestProcessDropsEventsThatCanNeverSucceed(t *testing.T) {
	cases := map[string][]byte{
		"undecodable":    []byte(`{not json`),
		"missing client": []byte(`{"outbox_id":"o1","client_id":"","user_id":"42","full_name":"Ada"}`),
		"non-numeric id": []byte(`{"outbox_id":"o1","client_id":"c1","user_id":"abc","full_name":"Ada"}`),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			updater := &fakeUpdater{}
			c := &UserRenamedConsumer{dbName: "db", updater: updater}

			require.NoError(t, c.process(context.Background(), payload))
			require.Zero(t, updater.calls)
		})
	}
}
