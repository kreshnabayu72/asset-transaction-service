package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMasterChangesRequestValidateAcceptsAllowedActions(t *testing.T) {
	for _, change := range []MasterChangesRequest{
		{Type: MasterChangeTypeGroup, Action: "deleted", IDs: []int64{1}},
		{Type: MasterChangeTypeGroup, Action: "moved", IDs: []int64{1}},
		{Type: MasterChangeTypeGroup, Action: "deactivated", IDs: []int64{1}},
		{Type: MasterChangeTypeGroup, Action: "archived", IDs: []int64{1}},
		{Type: MasterChangeTypeGroup, Action: "renamed", IDs: []int64{1}},
		{Type: MasterChangeTypeCategory, Action: "deleted", IDs: []int64{1}},
		{Type: MasterChangeTypeCategory, Action: "moved", IDs: []int64{1}},
		{Type: MasterChangeTypeCategory, Action: "renamed", IDs: []int64{1}},
		{Type: MasterChangeTypeBrand, Action: "deleted", IDs: []int64{1}},
		{Type: MasterChangeTypeBrand, Action: "deactivated", IDs: []int64{1}},
		{Type: MasterChangeTypeBrand, Action: "renamed", IDs: []int64{1}},
		{Type: MasterChangeTypeBrand, Action: "merged", IDs: []int64{1}, MergedIntoID: masterChangesInt64Pointer(2)},
	} {
		require.Empty(t, change.Validate(), "%s/%s", change.Type, change.Action)
	}
}

func TestMasterChangesRequestValidateRejectsInvalidFields(t *testing.T) {
	validationErrors := (MasterChangesRequest{
		Type: "asset", Action: "merged", IDs: []int64{0},
	}).Validate()

	require.NotEmpty(t, validationErrors)
	fields := make(map[string]bool, len(validationErrors))
	for _, validationError := range validationErrors {
		fields[validationError.Field] = true
	}
	require.True(t, fields["type"])
	require.True(t, fields["ids[0]"])
	require.True(t, fields["mergedIntoId"])

	validationErrors = (MasterChangesRequest{
		Type: MasterChangeTypeBrand, Action: "deleted", IDs: []int64{1}, MergedIntoID: masterChangesInt64Pointer(2),
	}).Validate()
	require.Len(t, validationErrors, 1)
	require.Equal(t, "mergedIntoId", validationErrors[0].Field)
}

func TestMasterChangesRequestIsNoOpForSnapshotOnlyChanges(t *testing.T) {
	for _, change := range []MasterChangesRequest{
		{Type: MasterChangeTypeGroup, Action: "moved"},
		{Type: MasterChangeTypeGroup, Action: "renamed"},
		{Type: MasterChangeTypeCategory, Action: "moved"},
		{Type: MasterChangeTypeCategory, Action: "renamed"},
		{Type: MasterChangeTypeBrand, Action: "deactivated"},
		{Type: MasterChangeTypeBrand, Action: "renamed"},
	} {
		require.True(t, change.IsNoOp(), "%s/%s", change.Type, change.Action)
	}
	require.False(t, (MasterChangesRequest{Type: MasterChangeTypeBrand, Action: "deleted"}).IsNoOp())
}

func masterChangesInt64Pointer(value int64) *int64 {
	return &value
}
