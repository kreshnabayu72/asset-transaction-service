package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
	centerCRUD "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/crud"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

type ServiceCenterCRUDRepository struct {
	dbName string
}

func NewServiceCenterCRUDRepository(dbName string) *ServiceCenterCRUDRepository {
	return &ServiceCenterCRUDRepository{dbName: dbName}
}

func (r *ServiceCenterCRUDRepository) BeginCRUDTransaction(ctx context.Context, clientID string) (centerCRUD.Transaction, error) {
	svc := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)
	tx, err := svc.GetPool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin PostgreSQL transaction: %w", err)
	}
	svc.SetTransaction(tx)
	return &serviceCenterCRUDTransaction{svc: svc}, nil
}

type serviceCenterCRUDTransaction struct {
	svc *coreService.PostgreSQLService
}

func (t *serviceCenterCRUDTransaction) NameAvailable(ctx context.Context, name string, excludeID *int64) (bool, error) {
	query, args, err := query_builder.BuildCheckAvailabilityQuery(name, excludeID)
	if err != nil {
		return false, fmt.Errorf("build Service Center name query: %w", err)
	}
	var row query_builder.CheckAvailabilityRow
	if err := t.svc.SelectOne(&row, ctx, query, args...); err != nil {
		return false, err
	}
	return row.Available, nil
}

func (t *serviceCenterCRUDTransaction) LockServiceCenter(ctx context.Context, id int64) (model.ServiceCenter, bool, error) {
	query, args, err := query_builder.BuildLockCRUDServiceCenterQuery(id)
	if err != nil {
		return model.ServiceCenter{}, false, fmt.Errorf("build Service Center lock query: %w", err)
	}
	var row model.ServiceCenterCRUDRow
	if err := t.svc.SelectOne(&row, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.ServiceCenter{}, false, nil
		}
		return model.ServiceCenter{}, false, err
	}
	return row.ToServiceCenter(), true, nil
}

func (t *serviceCenterCRUDTransaction) LoadGroupIDs(ctx context.Context, id int64) ([]int64, error) {
	query, args, err := query_builder.BuildCRUDGroupsQuery(id)
	if err != nil {
		return nil, fmt.Errorf("build Service Center group query: %w", err)
	}
	var rows []model.ServiceCenterCRUDGroupRow
	if err := t.svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.GroupID)
	}
	return ids, nil
}

func (t *serviceCenterCRUDTransaction) LoadCategoryIDs(ctx context.Context, id int64) ([]int64, error) {
	query, args, err := query_builder.BuildCRUDCategoriesQuery(id)
	if err != nil {
		return nil, fmt.Errorf("build Service Center category query: %w", err)
	}
	var rows []model.ServiceCenterCRUDCategoryRow
	if err := t.svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.CategoryID)
	}
	return ids, nil
}

func (t *serviceCenterCRUDTransaction) LoadBrandIDs(ctx context.Context, id int64) ([]int64, error) {
	query, args, err := query_builder.BuildCRUDBrandsQuery(id)
	if err != nil {
		return nil, fmt.Errorf("build Service Center brand query: %w", err)
	}
	var rows []model.ServiceCenterCRUDBrandRow
	if err := t.svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.BrandID)
	}
	return ids, nil
}

func (t *serviceCenterCRUDTransaction) LoadContacts(ctx context.Context, id int64) ([]model.ServiceCenterContact, error) {
	query, args, err := query_builder.BuildCRUDContactsQuery(id)
	if err != nil {
		return nil, fmt.Errorf("build Service Center contact query: %w", err)
	}
	var rows []model.ServiceCenterCRUDContactRow
	if err := t.svc.SelectMany(&rows, ctx, query, args...); err != nil {
		return nil, err
	}
	contacts := make([]model.ServiceCenterContact, 0, len(rows))
	for _, row := range rows {
		contacts = append(contacts, row.ToServiceCenterContact())
	}
	return contacts, nil
}

func (t *serviceCenterCRUDTransaction) HasAttachment(ctx context.Context, id int64) (bool, error) {
	count, err := t.svc.CountWithFilter(ctx, "service_center_attachments", sql_query.WhereQuery{
		"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
	})
	return count > 0, err
}

func (t *serviceCenterCRUDTransaction) InsertServiceCenter(ctx context.Context, center model.ServiceCenter) error {
	query, args, err := query_builder.BuildInsertServiceCenterQuery(center)
	if err != nil {
		return fmt.Errorf("build Service Center insert: %w", err)
	}
	_, err = t.svc.GetTransaction().Exec(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) InsertGroup(ctx context.Context, row model.ServiceCenterGroup) error {
	query, args, err := query_builder.BuildInsertServiceCenterGroupsQuery(row)
	if err != nil {
		return fmt.Errorf("build Service Center group insert: %w", err)
	}
	_, err = t.svc.GetTransaction().Exec(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) InsertCategory(ctx context.Context, row model.ServiceCenterCategory) error {
	query, args, err := query_builder.BuildInsertServiceCenterCategoriesQuery(row)
	if err != nil {
		return fmt.Errorf("build Service Center category insert: %w", err)
	}
	_, err = t.svc.GetTransaction().Exec(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) InsertBrand(ctx context.Context, row model.ServiceCenterBrand) error {
	query, args, err := query_builder.BuildInsertServiceCenterBrandsQuery(row)
	if err != nil {
		return fmt.Errorf("build Service Center brand insert: %w", err)
	}
	_, err = t.svc.GetTransaction().Exec(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) InsertContact(ctx context.Context, row model.ServiceCenterContact) error {
	query, args, err := query_builder.BuildInsertServiceCenterContactsQuery(row)
	if err != nil {
		return fmt.Errorf("build Service Center contact insert: %w", err)
	}
	_, err = t.svc.GetTransaction().Exec(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) UpdateServiceCenter(ctx context.Context, id int64, version int32, data centerCRUD.WriteData) error {
	query, args, err := query_builder.BuildUpdateServiceCenterQuery(id, version, sql_query.UpdateQuery{
		"name":                    data.Center.Name,
		"name_key":                data.Center.NameKey,
		"service_types":           data.Center.ServiceTypes,
		"country":                 data.Center.Country,
		"province_state":          data.Center.ProvinceState,
		"city":                    data.Center.City,
		"postal_code":             data.Center.PostalCode,
		"address_line":            data.Center.AddressLine,
		"company_phone_dial_code": data.Center.CompanyPhoneDialCode,
		"company_phone_number":    data.Center.CompanyPhoneNumber,
		"company_email":           data.Center.CompanyEmail,
		"notes":                   data.Center.Notes,
		"version":                 sql_query.UpdateRawSQL{Expr: "version + 1"},
		"modified_by_id":          data.Center.ModifiedByID,
		"modified_by_name":        data.Center.ModifiedByName,
	})
	if err != nil {
		return fmt.Errorf("build Service Center update: %w", err)
	}
	if _, err := t.svc.UpdateOne(ctx, query, args...); err != nil {
		return err
	}
	return nil
}

func (t *serviceCenterCRUDTransaction) DeleteChildRows(ctx context.Context, table string, id int64) error {
	query, args, err := query_builder.BuildDeleteServiceCenterChildRowsQuery(table, id)
	if err != nil {
		return fmt.Errorf("build Service Center child delete: %w", err)
	}
	_, err = t.svc.DeleteMany(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) DeleteServiceCenter(ctx context.Context, id int64, version int32) error {
	query, args, err := query_builder.BuildDeleteServiceCenterQuery(id, version)
	if err != nil {
		return fmt.Errorf("build Service Center delete: %w", err)
	}
	_, err = t.svc.DeleteOne(ctx, query, args...)
	return err
}

func (t *serviceCenterCRUDTransaction) Commit(ctx context.Context) error {
	return t.svc.CommitTransaction(ctx)
}

func (t *serviceCenterCRUDTransaction) Rollback(ctx context.Context) error {
	return t.svc.RollbackTransaction(ctx)
}

// Close leaves the process-wide pool open; Commit or Rollback releases the transaction.
func (t *serviceCenterCRUDTransaction) Close() {}
