package crud

import (
	"context"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

type WriteData struct {
	Center      model.ServiceCenter
	GroupIDs    []int64
	CategoryIDs []int64
	BrandIDs    []int64
	Contacts    []model.ServiceCenterContact
}

type Transaction interface {
	NameAvailable(ctx context.Context, name string, excludeID *int64) (bool, error)
	LockServiceCenter(ctx context.Context, id int64) (model.ServiceCenter, bool, error)
	LoadGroupIDs(ctx context.Context, id int64) ([]int64, error)
	LoadCategoryIDs(ctx context.Context, id int64) ([]int64, error)
	LoadBrandIDs(ctx context.Context, id int64) ([]int64, error)
	LoadContacts(ctx context.Context, id int64) ([]model.ServiceCenterContact, error)
	HasAttachment(ctx context.Context, id int64) (bool, error)
	InsertServiceCenter(ctx context.Context, center model.ServiceCenter) error
	InsertGroup(ctx context.Context, row model.ServiceCenterGroup) error
	InsertCategory(ctx context.Context, row model.ServiceCenterCategory) error
	InsertBrand(ctx context.Context, row model.ServiceCenterBrand) error
	InsertContact(ctx context.Context, row model.ServiceCenterContact) error
	UpdateServiceCenter(ctx context.Context, id int64, version int32, data WriteData) error
	DeleteChildRows(ctx context.Context, table string, id int64) error
	DeleteServiceCenter(ctx context.Context, id int64, version int32) error
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	Close()
}

type Repository interface {
	BeginCRUDTransaction(ctx context.Context, clientID string) (Transaction, error)
}
