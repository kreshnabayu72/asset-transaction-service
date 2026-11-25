package query_builder

import (
	"fmt"

	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/model"
)

func BuildInsertServiceCenterQuery(center model.ServiceCenter) (string, []any, error) {
	const query = `INSERT INTO service_centers (
		id, name, name_key, service_types, is_active, country, province_state, city,
		postal_code, address_line, company_phone_dial_code, company_phone_number,
		company_email, notes, version, created_by_id, created_by_name, created_at,
		updated_at, modified_by_id, modified_by_name
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
		$12, $13, $14, $15, $16, $17, $18, $19, $20, $21
	)`
	return query, []any{
		center.ID, center.Name, center.NameKey, center.ServiceTypes, center.IsActive,
		center.Country, center.ProvinceState, center.City, center.PostalCode,
		center.AddressLine, center.CompanyPhoneDialCode, center.CompanyPhoneNumber,
		center.CompanyEmail, center.Notes, center.Version, center.CreatedByID,
		center.CreatedByName, center.CreatedAt, center.UpdatedAt, center.ModifiedByID,
		center.ModifiedByName,
	}, nil
}

func BuildInsertServiceCenterGroupsQuery(row model.ServiceCenterGroup) (string, []any, error) {
	return "INSERT INTO service_center_groups (service_center_id, group_id) VALUES ($1, $2)", []any{row.ServiceCenterID, row.GroupID}, nil
}

func BuildInsertServiceCenterCategoriesQuery(row model.ServiceCenterCategory) (string, []any, error) {
	return "INSERT INTO service_center_categories (service_center_id, category_id) VALUES ($1, $2)", []any{row.ServiceCenterID, row.CategoryID}, nil
}

func BuildInsertServiceCenterBrandsQuery(row model.ServiceCenterBrand) (string, []any, error) {
	return "INSERT INTO service_center_brands (service_center_id, brand_id) VALUES ($1, $2)", []any{row.ServiceCenterID, row.BrandID}, nil
}

func BuildInsertServiceCenterContactsQuery(row model.ServiceCenterContact) (string, []any, error) {
	const query = `INSERT INTO service_center_contacts (
		id, service_center_id, position, name, phone_dial_code, phone_number, email, is_primary
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
	return query, []any{
		row.ID, row.ServiceCenterID, row.Position, row.Name, row.PhoneDialCode,
		row.PhoneNumber, row.Email, row.IsPrimary,
	}, nil
}

func BuildUpdateServiceCenterQuery(id int64, version int32, values sql_query.UpdateQuery) (string, []any, error) {
	return sql_query.NewSQLUpdateBuilder("service_centers").
		Update(values).
		Where(sql_query.WhereQuery{
			"id":      {Operator: sql_query.SQLOperatorEqual, Value: id},
			"version": {Operator: sql_query.SQLOperatorEqual, Value: version},
		}).
		Return("id").
		Build()
}

func BuildDeleteServiceCenterQuery(id int64, version int32) (string, []any, error) {
	return sql_query.NewSQLDeleteBuilder("service_centers").
		Delete("id").
		Where(sql_query.WhereQuery{
			"id":      {Operator: sql_query.SQLOperatorEqual, Value: id},
			"version": {Operator: sql_query.SQLOperatorEqual, Value: version},
		}).
		Build()
}

func BuildDeleteServiceCenterChildRowsQuery(table string, id int64) (string, []any, error) {
	switch table {
	case "service_center_groups", "service_center_categories", "service_center_brands", "service_center_contacts", "service_center_attachments":
	default:
		return "", nil, fmt.Errorf("unsupported Service Center child table %q", table)
	}
	return sql_query.NewSQLDeleteBuilder(table).
		Delete("service_center_id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		Build()
}

func BuildLockCRUDServiceCenterQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.ServiceCenterCRUDRow]("service_centers").
		Select(
			"id", "name", "name_key", "service_types", "is_active", "country",
			"province_state", "city", "postal_code", "address_line",
			"company_phone_dial_code", "company_phone_number", "company_email",
			"notes", "version", "created_by_id", "created_by_name", "created_at",
			"updated_at", "modified_by_id", "modified_by_name",
		).
		Where(sql_query.WhereQuery{
			"id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		ForUpdate(nil).
		Build()
}

func BuildCRUDGroupsQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.ServiceCenterCRUDGroupRow]("service_center_groups").
		Select("service_center_id", "group_id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		Build()
}

func BuildCRUDCategoriesQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.ServiceCenterCRUDCategoryRow]("service_center_categories").
		Select("service_center_id", "category_id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		Build()
}

func BuildCRUDBrandsQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.ServiceCenterCRUDBrandRow]("service_center_brands").
		Select("service_center_id", "brand_id").
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		Build()
}

func BuildCRUDContactsQuery(id int64) (string, []any, error) {
	return sql_query.NewSQLSelectBuilder[model.ServiceCenterCRUDContactRow]("service_center_contacts").
		Select(
			"id", "service_center_id", "position", "name", "phone_dial_code",
			"phone_number", "email", "is_primary",
		).
		Where(sql_query.WhereQuery{
			"service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: id},
		}).
		OrderBy([]string{"position"}, true).
		Build()
}
