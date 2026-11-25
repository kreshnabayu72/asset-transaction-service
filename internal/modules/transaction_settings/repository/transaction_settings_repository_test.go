package repository

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTransactionSettingsRepositoryKeepsFactory(t *testing.T) {
	fake := &fakeServicer{}
	repo := NewTransactionSettingsRepository(fake.factory())
	require.Same(t, fake, repo.open("c1"))
	require.NotNil(t, NewTenantServicerFactory("db"))
}
