package repository

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"sync"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/stretchr/testify/require"
)

type fakeCenterState struct {
	active         bool
	version        int32
	modifiedByName string
	modifiedByID   *int64
}

type fakeLinkKey struct {
	typeName        string
	serviceCenterID int64
}

type masterChangesPostgresFake struct {
	coreService.PostgreSQLServicer
	mu      sync.Mutex
	centers map[int64]*fakeCenterState
	links   map[fakeLinkKey]map[int64]struct{}
	tx      *masterChangesTxFake
	closed  bool
}

func newMasterChangesPostgresFake() *masterChangesPostgresFake {
	fake := &masterChangesPostgresFake{
		centers: make(map[int64]*fakeCenterState),
		links:   make(map[fakeLinkKey]map[int64]struct{}),
	}
	fake.tx = &masterChangesTxFake{fake: fake}
	return fake
}

func (f *masterChangesPostgresFake) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

type masterChangesTxFake struct {
	pgx.Tx
	fake                *masterChangesPostgresFake
	failServiceCenterID int64
	execCalls           int
	lastQuery           string
	lastArgs            []any
}

func (f *masterChangesPostgresFake) GetTransaction() pgx.Tx {
	return f.tx
}

func (tx *masterChangesTxFake) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	tx.execCalls++
	tx.lastQuery = query
	tx.lastArgs = append([]any(nil), args...)
	if len(args) != 2 {
		return pgconn.CommandTag{}, fmt.Errorf("merge insert received %d args", len(args))
	}
	serviceCenterID, ok := args[0].(int64)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("invalid service center id argument %T", args[0])
	}
	brandID, ok := args[1].(int64)
	if !ok {
		return pgconn.CommandTag{}, fmt.Errorf("invalid brand id argument %T", args[1])
	}
	if tx.failServiceCenterID == serviceCenterID {
		return pgconn.CommandTag{}, fmt.Errorf("injected merge insert failure")
	}

	tx.fake.mu.Lock()
	defer tx.fake.mu.Unlock()
	key := fakeLinkKey{typeName: dto.MasterChangeTypeBrand, serviceCenterID: serviceCenterID}
	if tx.fake.links[key] == nil {
		tx.fake.links[key] = make(map[int64]struct{})
	}
	if _, exists := tx.fake.links[key][brandID]; exists {
		return pgconn.NewCommandTag("INSERT 0 0"), nil
	}
	tx.fake.links[key][brandID] = struct{}{}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (f *masterChangesPostgresFake) SelectOne(v any, _ context.Context, _ string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	serviceCenterID := args[0].(int64)
	state, ok := f.centers[serviceCenterID]
	if !ok {
		return fmt.Errorf("Service Center not found")
	}
	row, ok := v.(*query_builder.LockedServiceCenterRow)
	if !ok {
		return fmt.Errorf("unexpected SelectOne destination %T", v)
	}
	*row = query_builder.LockedServiceCenterRow{
		ID: serviceCenterID, IsActive: state.active, Version: state.version,
		Name: "Example center", CreatedByID: 7,
	}
	return nil
}

func (f *masterChangesPostgresFake) SelectMany(v any, _ context.Context, query string, args ...any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	typeName, err := linkTypeFromQuery(query)
	if err != nil {
		return err
	}

	switch rows := v.(type) {
	case *[]query_builder.TargetCenterRow:
		requested := make(map[int64]struct{}, len(args))
		for _, arg := range args {
			requested[arg.(int64)] = struct{}{}
		}
		var targets []int64
		for key, linkedIDs := range f.links {
			if key.typeName != typeName {
				continue
			}
			for linkedID := range linkedIDs {
				if _, ok := requested[linkedID]; ok {
					targets = append(targets, key.serviceCenterID)
					break
				}
			}
		}
		sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })
		unique := targets[:0]
		for _, target := range targets {
			if len(unique) == 0 || unique[len(unique)-1] != target {
				unique = append(unique, target)
			}
		}
		*rows = make([]query_builder.TargetCenterRow, 0, len(unique))
		for _, target := range unique {
			*rows = append(*rows, query_builder.TargetCenterRow{ServiceCenterID: target})
		}
	case *[]query_builder.LinkIDRow:
		serviceCenterID := args[0].(int64)
		ids := make([]int64, 0, len(f.links[fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}]))
		for id := range f.links[fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}] {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		*rows = make([]query_builder.LinkIDRow, 0, len(ids))
		for _, id := range ids {
			*rows = append(*rows, query_builder.LinkIDRow{ID: id})
		}
	default:
		return fmt.Errorf("unexpected SelectMany destination %T", v)
	}
	return nil
}

func (f *masterChangesPostgresFake) DeleteMany(_ context.Context, query string, args ...any) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	typeName, err := linkTypeFromQuery(query)
	if err != nil {
		return 0, err
	}
	serviceCenterID := args[0].(int64)
	key := fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}
	linkedIDs := f.links[key]
	var deleted int64
	for _, arg := range args[1:] {
		id := arg.(int64)
		if _, ok := linkedIDs[id]; ok {
			delete(linkedIDs, id)
			deleted++
		}
	}
	return deleted, nil
}

func (f *masterChangesPostgresFake) UpdateOne(_ context.Context, query string, args ...any) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	serviceCenterID := args[len(args)-1].(int64)
	state, ok := f.centers[serviceCenterID]
	if !ok {
		return nil, fmt.Errorf("Service Center not found")
	}
	state.version++
	state.modifiedByID = nil
	state.modifiedByName = "System"
	if strings.Contains(query, `"is_active" =`) {
		state.active = false
	}
	return serviceCenterID, nil
}

func linkTypeFromQuery(query string) (string, error) {
	switch {
	case strings.Contains(query, "service_center_groups"):
		return dto.MasterChangeTypeGroup, nil
	case strings.Contains(query, "service_center_categories"):
		return dto.MasterChangeTypeCategory, nil
	case strings.Contains(query, "service_center_brands"):
		return dto.MasterChangeTypeBrand, nil
	default:
		return "", fmt.Errorf("unrecognized link query: %s", query)
	}
}

func addFakeLinks(fake *masterChangesPostgresFake, typeName string, serviceCenterID int64, ids ...int64) {
	key := fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}
	if fake.links[key] == nil {
		fake.links[key] = make(map[int64]struct{})
	}
	for _, id := range ids {
		fake.links[key][id] = struct{}{}
	}
}

func TestFindTargetCentersReturnsDistinctAscendingIDs(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	addFakeLinks(fake, dto.MasterChangeTypeGroup, 9, 1, 2)
	addFakeLinks(fake, dto.MasterChangeTypeGroup, 3, 1)

	targets, err := NewMasterChangesRepository("asset_transaction").findTargetCenters(
		context.Background(), fake,
		dto.MasterChangesRequest{Type: dto.MasterChangeTypeGroup, IDs: []int64{1, 2}},
	)

	require.NoError(t, err)
	require.Equal(t, []int64{3, 9}, targets)
}

func TestApplyGroupDeletionAutoDeactivatesWhenLastLinkIsRemoved(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[50] = &fakeCenterState{active: true, version: 4}
	addFakeLinks(fake, dto.MasterChangeTypeGroup, 50, 7)

	_, changed, err := NewMasterChangesRepository("asset_transaction").applyToServiceCenter(
		context.Background(), fake, 50,
		dto.MasterChangesRequest{Type: dto.MasterChangeTypeGroup, Action: "deleted", IDs: []int64{7}},
	)

	require.NoError(t, err)
	require.True(t, changed)
	require.Empty(t, fake.links[fakeLinkKey{typeName: dto.MasterChangeTypeGroup, serviceCenterID: 50}])
	require.False(t, fake.centers[50].active)
	require.Equal(t, int32(5), fake.centers[50].version)
	require.Equal(t, "System", fake.centers[50].modifiedByName)
	require.Nil(t, fake.centers[50].modifiedByID)
}

func TestApplyCategoryDeletionKeepsActiveStatusAndBumpsVersion(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[51] = &fakeCenterState{active: true, version: 2}
	addFakeLinks(fake, dto.MasterChangeTypeCategory, 51, 10, 11)

	_, changed, err := NewMasterChangesRepository("asset_transaction").applyToServiceCenter(
		context.Background(), fake, 51,
		dto.MasterChangesRequest{Type: dto.MasterChangeTypeCategory, Action: "deleted", IDs: []int64{10}},
	)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []int64{11}, currentFakeIDs(fake, dto.MasterChangeTypeCategory, 51))
	require.True(t, fake.centers[51].active)
	require.Equal(t, int32(3), fake.centers[51].version)
}

func TestApplyBrandMergeKeepsExistingDestinationLink(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[52] = &fakeCenterState{active: true, version: 8}
	addFakeLinks(fake, dto.MasterChangeTypeBrand, 52, 20, 99)
	targetID := int64(99)

	_, changed, err := NewMasterChangesRepository("asset_transaction").applyToServiceCenter(
		context.Background(), fake, 52,
		dto.MasterChangesRequest{
			Type: dto.MasterChangeTypeBrand, Action: "merged", IDs: []int64{20}, MergedIntoID: &targetID,
		},
	)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []int64{99}, currentFakeIDs(fake, dto.MasterChangeTypeBrand, 52))
	require.True(t, fake.centers[52].active)
	require.Equal(t, int32(9), fake.centers[52].version)
}

func TestApplyBrandMergeReplacesSingleSourceLink(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[53] = &fakeCenterState{active: true, version: 8}
	addFakeLinks(fake, dto.MasterChangeTypeBrand, 53, 20)
	targetID := int64(99)

	effect, changed, err := NewMasterChangesRepository("asset_transaction").applyToServiceCenter(
		context.Background(), fake, 53,
		dto.MasterChangesRequest{
			Type: dto.MasterChangeTypeBrand, Action: "merged", IDs: []int64{20}, MergedIntoID: &targetID,
		},
	)

	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []int64{20}, effect.BeforeIDs)
	require.Equal(t, []int64{99}, effect.AfterIDs)
	require.Equal(t, []int64{99}, currentFakeIDs(fake, dto.MasterChangeTypeBrand, 53))
	require.Equal(t, int32(9), fake.centers[53].version)
	require.Equal(t, 1, fake.tx.execCalls)
	require.Equal(t, int64(53), fake.tx.lastArgs[0])
	require.Equal(t, int64(99), fake.tx.lastArgs[1])
	require.Contains(t, fake.tx.lastQuery, "ON CONFLICT (service_center_id, brand_id) DO NOTHING")
}

func currentFakeIDs(fake *masterChangesPostgresFake, typeName string, serviceCenterID int64) []int64 {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	ids := make([]int64, 0, len(fake.links[fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}]))
	for id := range fake.links[fakeLinkKey{typeName: typeName, serviceCenterID: serviceCenterID}] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

type masterChangesFakeSnapshot struct {
	centers map[int64]*fakeCenterState
	links   map[fakeLinkKey]map[int64]struct{}
}

func snapshotMasterChangesFake(fake *masterChangesPostgresFake) masterChangesFakeSnapshot {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	snapshot := masterChangesFakeSnapshot{
		centers: make(map[int64]*fakeCenterState, len(fake.centers)),
		links:   make(map[fakeLinkKey]map[int64]struct{}, len(fake.links)),
	}
	for id, state := range fake.centers {
		stateCopy := *state
		snapshot.centers[id] = &stateCopy
	}
	for key, ids := range fake.links {
		linkCopy := make(map[int64]struct{}, len(ids))
		for id := range ids {
			linkCopy[id] = struct{}{}
		}
		snapshot.links[key] = linkCopy
	}
	return snapshot
}

func restoreMasterChangesFake(fake *masterChangesPostgresFake, snapshot masterChangesFakeSnapshot) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.centers = snapshot.centers
	fake.links = snapshot.links
}

func fakeTransactionRunner(fake *masterChangesPostgresFake) masterChangesTransactionFunc {
	return func(_ context.Context, _ coreService.PostgreSQLServicer, operation func() (int64, error)) (int64, error) {
		snapshot := snapshotMasterChangesFake(fake)
		affected, err := operation()
		if err != nil {
			restoreMasterChangesFake(fake, snapshot)
		}
		return affected, err
	}
}

func TestApplyMasterChangesRollsBackWhenMergeInsertFails(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[52] = &fakeCenterState{active: true, version: 8}
	fake.centers[53] = &fakeCenterState{active: true, version: 4}
	addFakeLinks(fake, dto.MasterChangeTypeBrand, 52, 20, 99)
	addFakeLinks(fake, dto.MasterChangeTypeBrand, 53, 20)
	fake.tx.failServiceCenterID = 53
	var serviceName string
	dispatchCalls := 0
	repo := NewMasterChangesRepository("asset_transaction")
	repo.newService = func(name string) coreService.PostgreSQLServicer {
		serviceName = name
		return fake
	}
	repo.runTransaction = fakeTransactionRunner(fake)
	repo.dispatcher = func(context.Context, MasterChangesEffect) error {
		dispatchCalls++
		return nil
	}
	targetID := int64(99)

	affected, err := repo.ApplyMasterChanges(context.Background(), "tenant-a", dto.MasterChangesRequest{
		Type: dto.MasterChangeTypeBrand, Action: "merged", IDs: []int64{20}, MergedIntoID: &targetID,
	})

	require.ErrorContains(t, err, "failed to merge Brand link for Service Center 53")
	require.Zero(t, affected)
	require.Zero(t, dispatchCalls)
	require.Equal(t, "tenant-a:asset_transaction", serviceName)
	require.False(t, fake.closed)
	require.Equal(t, []int64{20, 99}, currentFakeIDs(fake, dto.MasterChangeTypeBrand, 52))
	require.Equal(t, []int64{20}, currentFakeIDs(fake, dto.MasterChangeTypeBrand, 53))
	require.Equal(t, int32(8), fake.centers[52].version)
	require.Equal(t, int32(4), fake.centers[53].version)
	require.Equal(t, 2, fake.tx.execCalls)
}

func TestApplyMasterChangesWithoutTargetsSkipsTransaction(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	repo := NewMasterChangesRepository("asset_transaction")
	repo.newService = func(string) coreService.PostgreSQLServicer { return fake }
	transactionCalled := false
	repo.runTransaction = func(_ context.Context, _ coreService.PostgreSQLServicer, operation func() (int64, error)) (int64, error) {
		transactionCalled = true
		return operation()
	}

	affected, err := repo.ApplyMasterChanges(context.Background(), "tenant-a", dto.MasterChangesRequest{
		Type: dto.MasterChangeTypeGroup, Action: "deleted", IDs: []int64{7},
	})

	require.NoError(t, err)
	require.Zero(t, affected)
	require.False(t, transactionCalled)
}

func TestApplyMasterChangesDispatchesAfterCommitAndIgnoresFailure(t *testing.T) {
	fake := newMasterChangesPostgresFake()
	fake.centers[54] = &fakeCenterState{active: true, version: 2}
	addFakeLinks(fake, dto.MasterChangeTypeGroup, 54, 7)
	repo := NewMasterChangesRepository("asset_transaction")
	repo.newService = func(string) coreService.PostgreSQLServicer { return fake }
	transactionCommitted := false
	repo.runTransaction = func(_ context.Context, _ coreService.PostgreSQLServicer, operation func() (int64, error)) (int64, error) {
		affected, err := operation()
		transactionCommitted = err == nil
		return affected, err
	}
	var dispatched MasterChangesEffect
	dispatchCalls := 0
	repo.dispatcher = func(_ context.Context, effect MasterChangesEffect) error {
		require.True(t, transactionCommitted)
		dispatchCalls++
		dispatched = effect
		return fmt.Errorf("notification workflow unavailable")
	}

	affected, err := repo.ApplyMasterChanges(context.Background(), "tenant-a", dto.MasterChangesRequest{
		Type: dto.MasterChangeTypeGroup, Action: "deleted", IDs: []int64{7},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	require.Equal(t, 1, dispatchCalls)
	require.Equal(t, "tenant-a", dispatched.ClientID)
	require.Equal(t, int64(54), dispatched.ServiceCenterID)
	require.Equal(t, "Example center", dispatched.ServiceCenterName)
	require.Equal(t, int64(7), dispatched.CreatedByID)
	require.Equal(t, []int64{7}, dispatched.BeforeIDs)
	require.Empty(t, dispatched.AfterIDs)
	require.True(t, dispatched.WasActive)
	require.False(t, dispatched.IsActive)
	require.True(t, dispatched.AutoDeactivated)
}
