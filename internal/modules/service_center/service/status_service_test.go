package service

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	analyticsTemporal "gitea.tagsamurai.local/BETS-V2/service-contracts/services/analytics_reporting/temporal"
	coreEntity "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/entity"
)

type fakeStatusRepository struct {
	tx        *fakeStatusTransaction
	beginErr  error
	beginCall int
}

func (r *fakeStatusRepository) BeginStatusTransaction(context.Context, string) (StatusTransaction, error) {
	r.beginCall++
	if r.beginErr != nil {
		return nil, r.beginErr
	}
	return r.tx, nil
}

type fakeStatusTransaction struct {
	center       StoredServiceCenterStatus
	found        bool
	groupIDs     []int64
	lockErr      error
	groupsErr    error
	updateErr    error
	commitErr    error
	updated      bool
	updateTarget bool
	updateID     int64
	updateVer    int32
	actorID      int64
	actorName    string
	committed    bool
	rolledBack   bool
	closed       bool
}

func (tx *fakeStatusTransaction) LockStatusCenter(context.Context, int64) (StoredServiceCenterStatus, bool, error) {
	return tx.center, tx.found, tx.lockErr
}

func (tx *fakeStatusTransaction) LoadAvailableToGroupIDs(context.Context, int64) ([]int64, error) {
	return tx.groupIDs, tx.groupsErr
}

func (tx *fakeStatusTransaction) UpdateStatus(_ context.Context, id int64, expectedVersion int32, target bool, actorID int64, actorName string) error {
	if tx.updateErr != nil {
		return tx.updateErr
	}
	tx.updated = true
	tx.updateID = id
	tx.updateVer = expectedVersion
	tx.updateTarget = target
	tx.actorID = actorID
	tx.actorName = actorName
	return nil
}

func (tx *fakeStatusTransaction) Commit(context.Context) error {
	if tx.commitErr != nil {
		return tx.commitErr
	}
	tx.committed = true
	return nil
}

func (tx *fakeStatusTransaction) Rollback(context.Context) error {
	tx.rolledBack = true
	return nil
}

func (tx *fakeStatusTransaction) Close() {
	tx.closed = true
}

type fakeStatusLogDispatcher struct {
	calls      int
	param      analyticsTemporal.RecordChangeLogsParam
	err        error
	onDispatch func()
}

func (d *fakeStatusLogDispatcher) RecordChangeLogs(_ context.Context, param analyticsTemporal.RecordChangeLogsParam) error {
	d.calls++
	d.param = param
	if d.onDispatch != nil {
		d.onDispatch()
	}
	return d.err
}

func TestStatusAuthorizationCoversOnlyExplicitGroups(t *testing.T) {
	tests := []struct {
		name   string
		auth   StatusAuthorization
		groups []int64
		want   bool
	}{
		{name: "total control covers empty scope", auth: StatusAuthorization{TotalControl: true}, want: true},
		{name: "empty scope fails closed", auth: StatusAuthorization{ManageGroupIDs: []int64{11}}, want: false},
		{name: "one exact group manager scope", auth: StatusAuthorization{ManageGroupIDs: []int64{11}}, groups: []int64{11}, want: true},
		{name: "asset admin exact group scope", auth: StatusAuthorization{AssetAdminGroupIDs: []int64{11}}, groups: []int64{11}, want: true},
		{name: "all stored groups need coverage", auth: StatusAuthorization{ManageGroupIDs: []int64{11}}, groups: []int64{11, 12}, want: false},
		{name: "parent group does not imply child", auth: StatusAuthorization{ManageGroupIDs: []int64{11}}, groups: []int64{12}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.auth.CoversAvailableTo(tt.groups); got != tt.want {
				t.Fatalf("CoversAvailableTo(%v) = %v, want %v", tt.groups, got, tt.want)
			}
		})
	}
}

func TestChangeStatusOrdersNoOpBeforeVersionAndCommitsChanges(t *testing.T) {
	t.Run("already at target ignores stale version and writes nothing", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: false, Version: 9},
			found:    true,
			groupIDs: []int64{11},
		}
		dispatcher := &fakeStatusLogDispatcher{}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, dispatcher)
		got, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 1}, StatusAuthorization{ManageGroupIDs: []int64{11}}, 7, "Rina")
		if err != nil {
			t.Fatalf("ChangeStatus() error = %v", err)
		}
		if got.Changed || tx.updated || tx.committed || !tx.rolledBack || !tx.closed || dispatcher.calls != 0 {
			t.Fatalf("no-op transaction state = result %#v, tx %#v, logs %#v", got, tx, dispatcher)
		}
	})

	t.Run("real change stores actor, commits, and dispatches changelog afterward", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: true, Version: 9},
			found:    true,
			groupIDs: []int64{11, 12},
		}
		dispatchedAfterCommit := false
		dispatcher := &fakeStatusLogDispatcher{onDispatch: func() { dispatchedAfterCommit = tx.committed }}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, dispatcher)
		got, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 9}, StatusAuthorization{AssetAdminGroupIDs: []int64{11, 12}}, 7, "Rina Wijaya")
		if err != nil {
			t.Fatalf("ChangeStatus() error = %v", err)
		}
		if !got.Changed || !tx.updated || !tx.committed || tx.rolledBack || !tx.closed {
			t.Fatalf("change transaction state = result %#v, tx %#v", got, tx)
		}
		if tx.updateID != 42 || tx.updateVer != 9 || tx.updateTarget || tx.actorID != 7 || tx.actorName != "Rina Wijaya" {
			t.Fatalf("stored update = %#v", tx)
		}
		if !dispatchedAfterCommit || dispatcher.calls != 1 || dispatcher.param.ClientID != "tenant" {
			t.Fatalf("dispatch state = afterCommit:%v calls:%d payload:%#v", dispatchedAfterCommit, dispatcher.calls, dispatcher.param)
		}
		if len(dispatcher.param.ChangeLogs) != 1 {
			t.Fatalf("ChangeLogs = %#v", dispatcher.param.ChangeLogs)
		}
		log := dispatcher.param.ChangeLogs[0]
		if log.Action != "Deactivated" || log.Field == nil || *log.Field != "Status" || log.OldValue == nil || *log.OldValue != "Active" || log.NewValue == nil || *log.NewValue != "Inactive" {
			t.Fatalf("status changelog = %#v", log)
		}
		if log.LogTable != "service_centers" || log.CreatedByID != "7" || log.CreatedBySnapshot != "Rina Wijaya" || log.EntityID != "tenant" || log.EntityType != analyticsTemporal.EntityTypeClient || log.Product == nil || *log.Product != analyticsTemporal.ProductFixedAsset || log.Object != "Service Center" || log.ObjectID == nil || *log.ObjectID != "42" || log.ObjectName == nil || *log.ObjectName != "Center" {
			t.Fatalf("status changelog metadata = %#v", log)
		}
		if !reflect.DeepEqual(log.ScopeGroupIDs, []string{"11", "12"}) {
			t.Fatalf("ScopeGroupIDs = %v", log.ScopeGroupIDs)
		}
	})
}

func TestChangeStatusRejectsMissingAndOutOfScopeTargets(t *testing.T) {
	t.Run("missing center", func(t *testing.T) {
		tx := &fakeStatusTransaction{}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, nil)
		_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 1}, StatusAuthorization{TotalControl: true}, 7, "Rina")
		httpErr, ok := err.(*coreEntity.HttpError)
		if !ok || httpErr.Code != 404 || httpErr.Message != statusNoServiceCenterMessage || !tx.rolledBack {
			t.Fatalf("error = %#v, tx %#v", err, tx)
		}
	})

	t.Run("partial scope is denied", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: true, Version: 9},
			found:    true,
			groupIDs: []int64{11, 12},
		}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, nil)
		_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 9}, StatusAuthorization{ManageGroupIDs: []int64{11}}, 7, "Rina")
		httpErr, ok := err.(*coreEntity.HttpError)
		if !ok || httpErr.Code != 403 || !reflect.DeepEqual(httpErr.Data, map[string]string{"reason": "manage_scope"}) || tx.updated {
			t.Fatalf("error = %#v, tx %#v", err, tx)
		}
	})
}

func TestChangeStatusReturnsConflictAndEmptyActivationValidation(t *testing.T) {
	t.Run("stale version returns current record", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: true, Version: 9},
			found:    true,
			groupIDs: []int64{11},
		}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, nil)
		_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 8}, StatusAuthorization{TotalControl: true}, 7, "Rina")
		httpErr, ok := err.(*coreEntity.HttpError)
		if !ok || httpErr.Code != 409 || tx.updated {
			t.Fatalf("error = %#v, tx %#v", err, tx)
		}
		wantData := map[string]any{
			"reason":  "version_conflict",
			"current": map[string]any{"_id": "42", "name": "Center", "version": int32(9)},
		}
		if !reflect.DeepEqual(httpErr.Data, wantData) {
			t.Fatalf("conflict data = %#v, want %#v", httpErr.Data, wantData)
		}
	})

	t.Run("activation with empty Available to is rejected", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: false, Version: 3},
			found:    true,
			groupIDs: []int64{},
		}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, nil)
		_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: true, Version: 3}, StatusAuthorization{TotalControl: true}, 7, "Rina")
		httpErr, ok := err.(*coreEntity.HttpError)
		if !ok || httpErr.Code != 422 || httpErr.Message != "Invalid request body" || tx.updated {
			t.Fatalf("error = %#v, tx %#v", err, tx)
		}
	})
}

func TestChangeStatusMapsConcurrentVersionAndBestEffortLogFailure(t *testing.T) {
	t.Run("conditional update conflict maps to 409", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:    StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: true, Version: 9},
			found:     true,
			groupIDs:  []int64{11},
			updateErr: ErrStatusVersionChanged,
		}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, nil)
		_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 9}, StatusAuthorization{TotalControl: true}, 7, "Rina")
		httpErr, ok := err.(*coreEntity.HttpError)
		if !ok || httpErr.Code != 409 || !tx.rolledBack {
			t.Fatalf("error = %#v, tx %#v", err, tx)
		}
	})

	t.Run("log dispatch error does not roll back or change success", func(t *testing.T) {
		tx := &fakeStatusTransaction{
			center:   StoredServiceCenterStatus{ID: 42, Name: "Center", IsActive: false, Version: 3},
			found:    true,
			groupIDs: []int64{11},
		}
		dispatcher := &fakeStatusLogDispatcher{err: errors.New("temporal unavailable")}
		svc := NewStatusOperationService(&fakeStatusRepository{tx: tx}, dispatcher)
		got, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: true, Version: 3}, StatusAuthorization{TotalControl: true}, 7, "Rina")
		if err != nil || !got.Changed || !tx.committed || tx.rolledBack || dispatcher.calls != 1 {
			t.Fatalf("result = %#v, error = %v, tx %#v, logs %#v", got, err, tx, dispatcher)
		}
		log := dispatcher.param.ChangeLogs[0]
		if log.Action != "Activated" || log.OldValue == nil || *log.OldValue != "Inactive" || log.NewValue == nil || *log.NewValue != "Active" {
			t.Fatalf("activation changelog = %#v", log)
		}
	})
}

func TestChangeStatusRollsBackRepositoryFailures(t *testing.T) {
	boom := errors.New("database failed")
	tests := []struct {
		name string
		repo *fakeStatusRepository
		tx   *fakeStatusTransaction
	}{
		{name: "begin failure", repo: &fakeStatusRepository{beginErr: boom}},
		{name: "lock failure", tx: &fakeStatusTransaction{lockErr: boom}},
		{name: "scope read failure", tx: &fakeStatusTransaction{center: StoredServiceCenterStatus{ID: 42, IsActive: true, Version: 9}, found: true, groupIDs: []int64{11}, groupsErr: boom}},
		{name: "update failure", tx: &fakeStatusTransaction{center: StoredServiceCenterStatus{ID: 42, IsActive: true, Version: 9}, found: true, groupIDs: []int64{11}, updateErr: boom}},
		{name: "commit failure", tx: &fakeStatusTransaction{center: StoredServiceCenterStatus{ID: 42, IsActive: true, Version: 9}, found: true, groupIDs: []int64{11}, commitErr: boom}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.repo
			if repo == nil {
				repo = &fakeStatusRepository{tx: tt.tx}
			}
			svc := NewStatusOperationService(repo, nil)
			_, err := svc.ChangeStatus(context.Background(), "tenant", 42, dto.ChangeStatusRequest{IsActive: false, Version: 9}, StatusAuthorization{TotalControl: true}, 7, "Rina")
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v, want %v", err, boom)
			}
			if tt.tx != nil && (!tt.tx.rolledBack || tt.tx.committed) {
				t.Fatalf("transaction = %#v", tt.tx)
			}
		})
	}
}
