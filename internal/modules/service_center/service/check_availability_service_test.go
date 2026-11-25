package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type checkAvailabilityRepositoryStub struct {
	calls     int
	clientID  string
	name      string
	excludeID *int64
	available bool
	err       error
}

func (r *checkAvailabilityRepositoryStub) CheckNameAvailability(_ context.Context, clientID, name string, excludeID *int64) (bool, error) {
	r.calls++
	r.clientID = clientID
	r.name = name
	r.excludeID = excludeID
	return r.available, r.err
}

func TestCheckAvailabilityInvalidNamesReturnUnavailable(t *testing.T) {
	for _, name := range []string{" \t\n ", strings.Repeat("a", 121), "Workshop 🏎️"} {
		t.Run(name, func(t *testing.T) {
			repo := &checkAvailabilityRepositoryStub{available: true}
			svc := NewCheckAvailabilityService(repo)

			available, err := svc.CheckAvailability(context.Background(), "tenant-a", name, nil)

			require.NoError(t, err)
			require.False(t, available)
			require.Zero(t, repo.calls)
		})
	}
}

func TestCheckAvailabilityTrimsAndDelegates(t *testing.T) {
	excludeID := int64(41)
	repo := &checkAvailabilityRepositoryStub{available: true}
	svc := NewCheckAvailabilityService(repo)

	available, err := svc.CheckAvailability(context.Background(), "tenant-a", "  Workshop Élite  ", &excludeID)

	require.NoError(t, err)
	require.True(t, available)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, "tenant-a", repo.clientID)
	require.Equal(t, "Workshop Élite", repo.name)
	require.Same(t, &excludeID, repo.excludeID)
}

func TestCheckAvailabilityPropagatesRepositoryError(t *testing.T) {
	wantErr := errors.New("database unavailable")
	repo := &checkAvailabilityRepositoryStub{err: wantErr}
	svc := NewCheckAvailabilityService(repo)

	_, err := svc.CheckAvailability(context.Background(), "tenant-a", "Workshop", nil)

	require.ErrorIs(t, err, wantErr)
}
