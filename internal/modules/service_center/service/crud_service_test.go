package service

import (
	"context"
	"errors"
	"testing"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
)

type fakeCRUDRepository struct {
	tx CRUDTransaction
}

func (r fakeCRUDRepository) BeginCRUDTransaction(context.Context, string) (CRUDTransaction, error) {
	return r.tx, nil
}

type fakeCRUDTransaction struct {
	CRUDTransaction
	center        model.ServiceCenter
	groups        []int64
	categories    []int64
	brands        []int64
	contacts      []model.ServiceCenterContact
	hasAttachment bool
	rollbackCount int
	commitCount   int
	updateCount   int
	deleteCount   int
}

func (t *fakeCRUDTransaction) LockServiceCenter(context.Context, int64) (model.ServiceCenter, bool, error) {
	return t.center, true, nil
}
func (t *fakeCRUDTransaction) LoadGroupIDs(context.Context, int64) ([]int64, error) {
	return t.groups, nil
}
func (t *fakeCRUDTransaction) LoadCategoryIDs(context.Context, int64) ([]int64, error) {
	return t.categories, nil
}
func (t *fakeCRUDTransaction) LoadBrandIDs(context.Context, int64) ([]int64, error) {
	return t.brands, nil
}
func (t *fakeCRUDTransaction) LoadContacts(context.Context, int64) ([]model.ServiceCenterContact, error) {
	return t.contacts, nil
}
func (t *fakeCRUDTransaction) HasAttachment(context.Context, int64) (bool, error) {
	return t.hasAttachment, nil
}
func (t *fakeCRUDTransaction) UpdateServiceCenter(context.Context, int64, int32, CRUDWriteData) error {
	t.updateCount++
	return nil
}
func (t *fakeCRUDTransaction) DeleteServiceCenter(context.Context, int64, int32) error {
	t.deleteCount++
	return nil
}
func (t *fakeCRUDTransaction) Commit(context.Context) error {
	t.commitCount++
	return nil
}
func (t *fakeCRUDTransaction) Rollback(context.Context) error {
	t.rollbackCount++
	return nil
}
func (t *fakeCRUDTransaction) Close() {}

func TestUpdateNoOpDoesNotWriteEvenWhenVersionIsStale(t *testing.T) {
	phoneCode, phoneNumber := "+62", "81234567"
	tx := &fakeCRUDTransaction{
		center: model.ServiceCenter{
			ID:                   42,
			Name:                 "Local Center",
			NameKey:              "local center",
			ServiceTypes:         []string{"Routine"},
			Country:              "Indonesia",
			City:                 "Jakarta",
			CompanyPhoneDialCode: &phoneCode,
			CompanyPhoneNumber:   &phoneNumber,
			Version:              9,
		},
		groups: []int64{7},
	}
	svc := NewCRUDService(fakeCRUDRepository{tx: tx}, nil)
	request := dto.UpdateServiceCenterRequest{
		ServiceCenterWriteRequest: dto.ServiceCenterWriteRequest{
			Name:             "Local Center",
			ServiceTypes:     []string{"Routine"},
			AvailableTo:      []string{"7"},
			Country:          "Indonesia",
			City:             "Jakarta",
			CompanyPhone:     &dto.ServiceCenterPhone{DialCode: "+62", Number: "81234567"},
			AttachmentFileID: dto.OptionalString{Set: true},
		},
		Version: 8,
	}

	changed, err := svc.UpdateServiceCenter(context.Background(), "client_one", 42, request, 11, "Editor")
	if err != nil {
		t.Fatalf("UpdateServiceCenter returned error: %v", err)
	}
	if changed {
		t.Fatal("changed = true, want false")
	}
	if tx.updateCount != 0 || tx.deleteCount != 0 || tx.commitCount != 0 {
		t.Fatalf("no-op performed writes: updates=%d deletes=%d commits=%d", tx.updateCount, tx.deleteCount, tx.commitCount)
	}
	if tx.rollbackCount != 1 {
		t.Fatalf("rollback count = %d, want 1", tx.rollbackCount)
	}
}

func TestUpdateNullAttachmentFailsClosedWhenAttachmentExists(t *testing.T) {
	tx := &fakeCRUDTransaction{
		center:        model.ServiceCenter{ID: 42, Name: "Local Center", Version: 3},
		hasAttachment: true,
	}
	svc := NewCRUDService(fakeCRUDRepository{tx: tx}, nil)
	request := dto.UpdateServiceCenterRequest{
		ServiceCenterWriteRequest: dto.ServiceCenterWriteRequest{
			Name:             "Local Center",
			ServiceTypes:     []string{"Routine"},
			AvailableTo:      []string{"7"},
			Country:          "Indonesia",
			City:             "Jakarta",
			AttachmentFileID: dto.OptionalString{Set: true},
		},
		Version: 3,
	}

	changed, err := svc.UpdateServiceCenter(context.Background(), "client_one", 42, request, 11, "Editor")
	if changed {
		t.Fatal("changed = true, want false")
	}
	if !errors.Is(err, ErrCRUDAttachmentContractUnresolved) {
		t.Fatalf("UpdateServiceCenter error = %v, want attachment-contract-unresolved", err)
	}
	if tx.updateCount != 0 || tx.deleteCount != 0 || tx.commitCount != 0 {
		t.Fatalf("attachment removal mutated data: updates=%d deletes=%d commits=%d", tx.updateCount, tx.deleteCount, tx.commitCount)
	}
	if tx.rollbackCount != 1 {
		t.Fatalf("rollback count = %d, want 1", tx.rollbackCount)
	}
}

func TestDeleteFailsClosedWithoutUsageProvider(t *testing.T) {
	tx := &fakeCRUDTransaction{center: model.ServiceCenter{ID: 42, Name: "Local Center", Version: 3}}
	svc := NewCRUDService(fakeCRUDRepository{tx: tx}, nil)

	err := svc.DeleteServiceCenter(context.Background(), "client_one", 42, 3)
	if !errors.Is(err, ErrCRUDUsageProviderUnavailable) {
		t.Fatalf("DeleteServiceCenter error = %v, want usage-provider-unavailable", err)
	}
	if tx.deleteCount != 0 || tx.commitCount != 0 {
		t.Fatalf("fail-closed delete mutated data: deletes=%d commits=%d", tx.deleteCount, tx.commitCount)
	}
	if tx.rollbackCount != 1 {
		t.Fatalf("rollback count = %d, want 1", tx.rollbackCount)
	}
}

func TestDeleteWithAttachmentFailsClosedBeforeMutation(t *testing.T) {
	tx := &fakeCRUDTransaction{
		center:        model.ServiceCenter{ID: 42, Name: "Local Center", Version: 3},
		hasAttachment: true,
	}
	svc := NewCRUDService(fakeCRUDRepository{tx: tx}, nil)

	err := svc.DeleteServiceCenter(context.Background(), "client_one", 42, 3)
	if !errors.Is(err, ErrCRUDAttachmentContractUnresolved) {
		t.Fatalf("DeleteServiceCenter error = %v, want attachment-contract-unresolved", err)
	}
	if tx.deleteCount != 0 || tx.commitCount != 0 {
		t.Fatalf("fail-closed delete mutated data: deletes=%d commits=%d", tx.deleteCount, tx.commitCount)
	}
}
