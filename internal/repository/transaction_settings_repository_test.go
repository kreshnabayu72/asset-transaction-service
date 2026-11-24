package repository

import (
	"context"
	"errors"
	"testing"

	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
	"github.com/stretchr/testify/require"
)

// fakeService records UpdateManyWithData calls. The embedded interface satisfies the rest of
// PostgreSQLServicer; any method not overridden here panics if called.
type fakeService struct {
	coreService.PostgreSQLServicer
	table  string
	where  sql_query.WhereQuery
	update sql_query.UpdateQuery
	calls  int
	err    error
}

func (f *fakeService) UpdateManyWithData(
	_ context.Context,
	table string,
	where sql_query.WhereQuery,
	body sql_query.UpdateQuery,
	_ ...coreService.ReturningConfig,
) (int64, error) {
	f.calls++
	f.table, f.where, f.update = table, where, body
	return 1, f.err
}

func TestUpdateModifiedByNameTargetsUserRows(t *testing.T) {
	svc := &fakeService{}

	err := NewTransactionSettingsRepository().UpdateModifiedByName(context.Background(), svc, 42, "Ada Lovelace")

	require.NoError(t, err)
	require.Equal(t, 1, svc.calls)
	require.Equal(t, "transaction_settings", svc.table)
	require.Equal(t, sql_query.UpdateQuery{"modified_by_name": "Ada Lovelace"}, svc.update)
	require.Equal(t, sql_query.SQLOperatorEqual, svc.where["modified_by_id"].Operator)
	require.Equal(t, int64(42), svc.where["modified_by_id"].Value)
}

func TestUpdateModifiedByNameWrapsError(t *testing.T) {
	boom := errors.New("boom")
	svc := &fakeService{err: boom}

	err := NewTransactionSettingsRepository().UpdateModifiedByName(context.Background(), svc, 1, "x")

	require.ErrorIs(t, err, boom)
}

func TestRuleTableRenamesTargetUserColumns(t *testing.T) {
	repo := NewTransactionSettingsRepository()
	cases := []struct {
		name, table, idCol, nameCol string
		run                         func(*fakeService) error
	}{
		{"fields", "transaction_rule_fields", "modified_by_id", "modified_by_name", func(s *fakeService) error {
			return repo.UpdateRuleFieldsModifiedByName(context.Background(), s, 7, "Grace")
		}},
		{"overrides", "transaction_rule_overrides", "created_by_id", "created_by_name", func(s *fakeService) error {
			return repo.UpdateRuleOverridesCreatedByName(context.Background(), s, 7, "Grace")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{}

			require.NoError(t, tc.run(svc))
			require.Equal(t, tc.table, svc.table)
			require.Equal(t, sql_query.UpdateQuery{tc.nameCol: "Grace"}, svc.update)
			require.Equal(t, int64(7), svc.where[tc.idCol].Value)
		})
	}
}
