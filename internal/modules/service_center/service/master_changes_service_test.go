package service

import (
	"context"
	"errors"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"github.com/stretchr/testify/require"
)

type masterChangesRepositoryStub struct {
	clientID string
	change   dto.MasterChangesRequest
	affected int64
	err      error
	calls    int
}

func (r *masterChangesRepositoryStub) ApplyMasterChanges(
	_ context.Context,
	clientID string,
	change dto.MasterChangesRequest,
) (int64, error) {
	r.calls++
	r.clientID = clientID
	r.change = change
	return r.affected, r.err
}

func TestMasterChangesNoOpsDoNotReachRepository(t *testing.T) {
	repo := &masterChangesRepositoryStub{}
	svc := NewMasterChangesService(repo)

	affected, err := svc.ApplyMasterChanges(context.Background(), "tenant-a", dto.MasterChangesRequest{
		Type: dto.MasterChangeTypeBrand, Action: "renamed", IDs: []int64{10},
	})

	require.NoError(t, err)
	require.Zero(t, affected)
	require.Zero(t, repo.calls)
}

func TestMasterChangesDelegatesAndPropagatesResult(t *testing.T) {
	wantErr := errors.New("database failed")
	repo := &masterChangesRepositoryStub{affected: 2, err: wantErr}
	svc := NewMasterChangesService(repo)
	change := dto.MasterChangesRequest{Type: dto.MasterChangeTypeCategory, Action: "deleted", IDs: []int64{12}}

	affected, err := svc.ApplyMasterChanges(context.Background(), "tenant-a", change)

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, int64(2), affected)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "tenant-a", repo.clientID)
	require.Equal(t, change, repo.change)
}
