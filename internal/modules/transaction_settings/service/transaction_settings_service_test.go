package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/transaction_settings/constants"
)

func TestNewServiceKeepsSeams(t *testing.T) {
	renderer, activities := &fakeRenderer{}, &fakeActivities{}
	repo := &fakeRepo{}
	svc := NewService(repo, renderer).WithActivityEntryDispatcher(activities)
	require.Same(t, repo, svc.repo)
	require.Same(t, renderer, svc.renderer)
	require.Same(t, activities, svc.activityEntries)
}

func TestFullPlanGatesNothing(t *testing.T) {
	require.True(t, FullPlan.ApprovalWorkflowAvailable)
	require.Equal(t, constants.LevelFull, FullPlan.FeatureSuite.Borrowing)
	require.Equal(t, constants.LevelFull, FullPlan.FeatureSuite.Assignment)
	require.Equal(t, constants.LevelFull, FullPlan.FeatureSuite.Transfer)
	require.Equal(t, constants.LevelFull, FullPlan.FeatureSuite.Disposal)
}
