package query_builder

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/sql_query"
)

var ErrUsageSourceUnavailable = errors.New("service center usage data source is not configured")
var ErrReferenceNamesUnavailable = errors.New("service center master reference names are not configured")

type ReadCenterRow struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	ServiceTypes         []string  `json:"service_types"`
	IsActive             bool      `json:"is_active"`
	Country              string    `json:"country"`
	ProvinceState        *string   `json:"province_state"`
	City                 string    `json:"city"`
	PostalCode           *string   `json:"postal_code"`
	AddressLine          *string   `json:"address_line"`
	CompanyPhoneDialCode *string   `json:"company_phone_dial_code"`
	CompanyPhoneNumber   *string   `json:"company_phone_number"`
	CompanyEmail         *string   `json:"company_email"`
	Notes                *string   `json:"notes"`
	Version              int32     `json:"version"`
	CreatedByID          int64     `json:"created_by_id"`
	CreatedByName        string    `json:"created_by_name"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	ModifiedByName       *string   `json:"modified_by_name"`
	GroupIDs             []int64   `json:"group_ids"`
	CategoryIDs          []int64   `json:"category_ids"`
	BrandIDs             []int64   `json:"brand_ids"`
}

func (row *ReadCenterRow) UnmarshalJSON(data []byte) error {
	type readCenterRowAlias ReadCenterRow
	var decoded struct {
		*readCenterRowAlias
		ID          json.RawMessage   `json:"id"`
		CreatedByID json.RawMessage   `json:"created_by_id"`
		GroupIDs    []json.RawMessage `json:"group_ids"`
		CategoryIDs []json.RawMessage `json:"category_ids"`
		BrandIDs    []json.RawMessage `json:"brand_ids"`
	}
	decoded.readCenterRowAlias = (*readCenterRowAlias)(row)
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var err error
	if row.ID, err = parseReadDecimalID(decoded.ID); err != nil {
		return fmt.Errorf("parse Service Center row id: %w", err)
	}
	if row.CreatedByID, err = parseReadDecimalID(decoded.CreatedByID); err != nil {
		return fmt.Errorf("parse Service Center created_by_id: %w", err)
	}
	if row.GroupIDs, err = parseReadDecimalIDs(decoded.GroupIDs); err != nil {
		return fmt.Errorf("parse Service Center group_ids: %w", err)
	}
	if row.CategoryIDs, err = parseReadDecimalIDs(decoded.CategoryIDs); err != nil {
		return fmt.Errorf("parse Service Center category_ids: %w", err)
	}
	if row.BrandIDs, err = parseReadDecimalIDs(decoded.BrandIDs); err != nil {
		return fmt.Errorf("parse Service Center brand_ids: %w", err)
	}
	return nil
}

func parseReadDecimalID(raw json.RawMessage) (int64, error) {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return 0, nil
	}
	if strings.HasPrefix(text, "\"") {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return 0, err
		}
		text = strings.TrimSpace(value)
	}
	return strconv.ParseInt(text, 10, 64)
}

func parseReadDecimalIDs(rawIDs []json.RawMessage) ([]int64, error) {
	ids := make([]int64, 0, len(rawIDs))
	for index, rawID := range rawIDs {
		id, err := parseReadDecimalID(rawID)
		if err != nil {
			return nil, fmt.Errorf("id at index %d: %w", index, err)
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func BuildListServiceCentersQuery(request dto.ServiceCenterReadFilters, visibleColumns []string, groupIDs []int64, totalControl, readAllNonEmpty bool, page, limit int) (string, []any, error) {
	if request.SortBy == "usage" || request.UsageMin != nil || request.UsageMax != nil {
		return "", nil, ErrUsageSourceUnavailable
	}
	builder := readCenterBuilder()
	applyReadFilters(builder, request, groupIDs, totalControl, readAllNonEmpty)
	if search := request.SearchTerm(); search != "" {
		fields := visibleSearchFields(visibleColumns)
		if len(fields) > 0 {
			builder.Search(search, fields)
		}
	}
	sortRules := readSortRules(request.SortBy, request.SortOrder)
	builder.Paginate(sql_query.Pagination{
		Page:      page,
		Limit:     limit,
		SortBy:    sortRules[0].SortBy,
		SortOrder: request.SortOrder,
		MultiSort: sortRules,
	})
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center list query: %w", err)
	}
	return query, args, nil
}

func BuildExportServiceCentersQuery(request dto.ServiceCenterReadFilters, visibleColumns []string, groupIDs []int64, totalControl, readAllNonEmpty bool) (string, []any, error) {
	if request.SortBy == "usage" || request.UsageMin != nil || request.UsageMax != nil {
		return "", nil, ErrUsageSourceUnavailable
	}
	builder := readCenterBuilder()
	applyReadFilters(builder, request, groupIDs, totalControl, readAllNonEmpty)
	if search := request.SearchTerm(); search != "" {
		fields := visibleSearchFields(visibleColumns)
		if len(fields) > 0 {
			builder.Search(search, fields)
		}
	}
	applyReadSort(builder, request.SortBy, request.SortOrder)
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center export query: %w", err)
	}
	return query, args, nil
}

type ContactPersonRow struct {
	Name          string  `json:"name"`
	PhoneDialCode *string `json:"phone_dial_code"`
	PhoneNumber   *string `json:"phone_number"`
	Email         *string `json:"email"`
	IsPrimary     bool    `json:"is_primary"`
}

func BuildContactPersonsQuery(serviceCenterID int64) (string, []any, error) {
	builder := sql_query.NewSQLSelectBuilder[ContactPersonRow]("service_center_contacts", "c").
		Select("c.name", "c.phone_dial_code", "c.phone_number", "c.email", "c.is_primary").
		Where(sql_query.WhereQuery{"c.service_center_id": {Operator: sql_query.SQLOperatorEqual, Value: serviceCenterID}}).
		OrderBy([]string{"c.position"}, true)
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center contact query: %w", err)
	}
	return query, args, nil
}

func BuildCountServiceCentersQuery(request dto.ServiceCenterReadFilters, visibleColumns []string, groupIDs []int64, totalControl, readAllNonEmpty bool) (string, []any, error) {
	if request.SortBy == "usage" || request.UsageMin != nil || request.UsageMax != nil {
		return "", nil, ErrUsageSourceUnavailable
	}
	if request.SortBy == "specializations" || request.SortBy == "availableTo" || request.SortBy == "brands" {
		return "", nil, ErrReferenceNamesUnavailable
	}
	if request.SearchTerm() != "" && hasUnresolvedReferenceSearch(visibleColumns) {
		return "", nil, ErrReferenceNamesUnavailable
	}
	builder := sql_query.NewSQLCountBuilder("service_centers", "s")
	applyReadFilters(builder, request, groupIDs, totalControl, readAllNonEmpty)
	if search := request.SearchTerm(); search != "" {
		fields := visibleSearchFields(visibleColumns)
		if len(fields) > 0 {
			builder.Search(search, fields)
		}
	}
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center count query: %w", err)
	}
	return query, args, nil
}

func BuildDetailServiceCenterQuery(id int64, groupIDs []int64, totalControl, readAllNonEmpty bool) (string, []any, error) {
	builder := readCenterBuilder()
	builder.Where(sql_query.WhereQuery{"s.id": {Operator: sql_query.SQLOperatorEqual, Value: id}})
	if readAllNonEmpty {
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id)"}})
	} else if !totalControl {
		if len(groupIDs) == 0 {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "FALSE"}})
		} else {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id AND scope_scg.group_id = ANY(?))", ExtraArgs: []any{groupIDs}}})
		}
	}
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center detail query: %w", err)
	}
	return query, args, nil
}

func readCenterBuilder() sql_query.Selector {
	return sql_query.NewSQLSelectBuilder[ReadCenterRow]("service_centers", "s").
		Select(
			"s.name", "s.service_types", "s.is_active", "s.country", "s.province_state",
			"s.city", "s.postal_code", "s.address_line", "s.company_phone_dial_code",
			"s.company_phone_number", "s.company_email", "s.notes", "s.version",
			"s.created_by_name", "s.created_at", "s.updated_at", "s.modified_by_name",
		).
		SelectRaw("s.id::text AS id").
		SelectRaw("s.created_by_id::text AS created_by_id").
		SelectRaw("COALESCE((SELECT array_agg(scg.group_id::text ORDER BY scg.group_id) FROM service_center_groups scg WHERE scg.service_center_id = s.id), '{}'::text[]) AS group_ids").
		SelectRaw("COALESCE((SELECT array_agg(scc.category_id::text ORDER BY scc.category_id) FROM service_center_categories scc WHERE scc.service_center_id = s.id), '{}'::text[]) AS category_ids").
		SelectRaw("COALESCE((SELECT array_agg(scb.brand_id::text ORDER BY scb.brand_id) FROM service_center_brands scb WHERE scb.service_center_id = s.id), '{}'::text[]) AS brand_ids")
}

func applyReadFilters(builder sql_query.Selector, request dto.ServiceCenterReadFilters, groupIDs []int64, totalControl, readAllNonEmpty bool) {
	if readAllNonEmpty {
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id)"}})
	} else if !totalControl {
		if len(groupIDs) == 0 {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "FALSE"}})
		} else {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id AND scope_scg.group_id = ANY(?))", ExtraArgs: []any{groupIDs}}})
		}
	}
	if len(request.ServiceType) > 0 {
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "s.service_types && ?::text[]", ExtraArgs: []any{request.ServiceType}}})
	}
	if len(request.Specialization) > 0 {
		ids, _ := request.PositiveIDs(request.Specialization)
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "(NOT EXISTS (SELECT 1 FROM service_center_categories f_scc WHERE f_scc.service_center_id = s.id) OR EXISTS (SELECT 1 FROM service_center_categories f_scc WHERE f_scc.service_center_id = s.id AND f_scc.category_id = ANY(?)))", ExtraArgs: []any{ids}}})
	}
	if len(request.AvailableTo) > 0 {
		ids, _ := request.PositiveIDs(request.AvailableTo)
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups f_scg WHERE f_scg.service_center_id = s.id AND f_scg.group_id = ANY(?))", ExtraArgs: []any{ids}}})
	}
	if len(request.Brand) > 0 {
		ids, _ := request.PositiveIDs(request.Brand)
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "(NOT EXISTS (SELECT 1 FROM service_center_brands f_scb WHERE f_scb.service_center_id = s.id) OR EXISTS (SELECT 1 FROM service_center_brands f_scb WHERE f_scb.service_center_id = s.id AND f_scb.brand_id = ANY(?)))", ExtraArgs: []any{ids}}})
	}
	if len(request.Status) > 0 {
		statuses := make([]bool, 0, len(request.Status))
		for _, status := range request.Status {
			statuses = append(statuses, status == "Active")
		}
		builder.Where(sql_query.WhereQuery{"s.is_active": {Operator: sql_query.SQLOperatorAny, Value: statuses}})
	}
	if len(request.Country) > 0 {
		builder.Where(sql_query.WhereQuery{"s.country": {Operator: sql_query.SQLOperatorAny, Value: request.Country}})
	}
	if len(request.City) > 0 {
		builder.Where(sql_query.WhereQuery{"s.city": {Operator: sql_query.SQLOperatorAny, Value: request.City}})
	}
	if len(request.ModifiedBy) > 0 {
		builder.Where(sql_query.WhereQuery{"COALESCE(s.modified_by_name, 'System')": {Operator: sql_query.SQLOperatorAny, Value: request.ModifiedBy}})
	}
	if len(request.UpdatedAt) == 2 {
		builder.Where(sql_query.WhereQuery{"s.updated_at": {Operator: sql_query.SQLOperatorBetween, Value: request.UpdatedAt, IsEpochTime: true}})
	}
}

func visibleSearchFields(columns []string) []string {
	visible := make(map[string]struct{}, len(columns))
	for _, key := range columns {
		visible[key] = struct{}{}
	}
	var fields []string
	if _, ok := visible["name"]; ok {
		fields = append(fields, "s.name")
	}
	if _, ok := visible["serviceTypes"]; ok {
		fields = append(fields, "s.service_types:array")
	}
	if _, ok := visible["city"]; ok {
		fields = append(fields, "s.city", "(s.city || ', ' || s.country)")
	}
	if _, ok := visible["isActive"]; ok {
		fields = append(fields, "CASE WHEN s.is_active THEN 'Active' ELSE 'Inactive' END")
	}
	if _, ok := visible["modifiedBy"]; ok {
		fields = append(fields, "COALESCE(s.modified_by_name, 'System')")
	}
	return fields
}

func hasUnresolvedReferenceSearch(columns []string) bool {
	for _, column := range columns {
		if column == "specializations" || column == "availableTo" || column == "brands" {
			return true
		}
	}
	return false
}

func applyReadSort(builder sql_query.Selector, sortBy string, sortOrder int) {
	ascending := sortOrder == 1
	expressions := map[string]string{
		"name":            "LOWER(BTRIM(s.name))",
		"serviceTypes":    "COALESCE((SELECT string_agg(service_type, ', ' ORDER BY service_type) FROM unnest(s.service_types) AS service_type), '')",
		"specializations": "COALESCE((SELECT string_agg(category_id::text, ',' ORDER BY category_id) FROM service_center_categories WHERE service_center_id = s.id), '')",
		"availableTo":     "COALESCE((SELECT string_agg(group_id::text, ',' ORDER BY group_id) FROM service_center_groups WHERE service_center_id = s.id), '')",
		"brands":          "COALESCE((SELECT string_agg(brand_id::text, ',' ORDER BY brand_id) FROM service_center_brands WHERE service_center_id = s.id), '')",
		"city":            "NULLIF(LOWER(BTRIM(s.city)), '')",
		"isActive":        "s.is_active",
		"modifiedBy":      "NULLIF(LOWER(BTRIM(COALESCE(s.modified_by_name, 'System'))), '')",
		"updatedAt":       "s.updated_at",
	}
	if sortBy == "usage" {
		expressions[sortBy] = "NULL"
	}
	expression := expressions[sortBy]
	if expression == "" {
		expression = expressions["name"]
	}
	builder.OrderBy([]string{expression}, ascending, true)
	if sortBy != "name" {
		builder.OrderBy([]string{"LOWER(BTRIM(s.name))"}, true, true)
	}
	builder.OrderBy([]string{"s.id"}, ascending, true)
}

func readSortRules(sortBy string, sortOrder int) []sql_query.Sort {
	columns := map[string]string{
		"name":            "LOWER(BTRIM(s.name))",
		"serviceTypes":    "COALESCE((SELECT string_agg(service_type, ', ' ORDER BY service_type) FROM unnest(s.service_types) AS service_type), '')",
		"specializations": "COALESCE((SELECT string_agg(category_id::text, ',' ORDER BY category_id) FROM service_center_categories WHERE service_center_id = s.id), '')",
		"availableTo":     "COALESCE((SELECT string_agg(group_id::text, ',' ORDER BY group_id) FROM service_center_groups WHERE service_center_id = s.id), '')",
		"brands":          "COALESCE((SELECT string_agg(brand_id::text, ',' ORDER BY brand_id) FROM service_center_brands WHERE service_center_id = s.id), '')",
		"city":            "NULLIF(LOWER(BTRIM(s.city)), '')",
		"isActive":        "s.is_active",
		"modifiedBy":      "NULLIF(LOWER(BTRIM(COALESCE(s.modified_by_name, 'System'))), '')",
		"updatedAt":       "s.updated_at",
	}
	expression := columns[sortBy]
	if expression == "" {
		expression = columns["name"]
	}
	rules := []sql_query.Sort{{SortBy: expression, SortOrder: sortOrder, UseNullsHandling: true}}
	if sortBy != "name" {
		rules = append(rules, sql_query.Sort{SortBy: columns["name"], SortOrder: 1, UseNullsHandling: true})
	}
	return append(rules, sql_query.Sort{SortBy: "s.id", SortOrder: sortOrder, UseNullsHandling: true})
}

func BuildOptionsValueQuery(facet string, groupIDs []int64, totalControl, readAllNonEmpty bool) (string, []any, error) {
	var table, expression string
	var lateralQuery, lateralAlias string
	var expandServiceTypes bool
	switch facet {
	case "serviceTypeOptions":
		table, expression, expandServiceTypes = "service_centers", "service_type", true
	case "statusOptions":
		table, expression = "service_centers", "CASE WHEN s.is_active THEN 'Active' ELSE 'Inactive' END"
	case "countryOptions":
		table, expression = "service_centers", "s.country"
	case "cityOptions":
		table, expression = "service_centers", "s.city"
	case "modifiedByOptions":
		table, expression = "service_centers", "COALESCE(s.modified_by_name, 'System')"
	case "availableToOptions":
		table, expression = "service_centers", "service_center_reference.value"
		lateralQuery, lateralAlias = "SELECT group_id::text AS value FROM service_center_groups WHERE service_center_id = s.id", "service_center_reference"
	case "specializationOptions":
		table, expression = "service_centers", "service_center_reference.value"
		lateralQuery, lateralAlias = "SELECT category_id::text AS value FROM service_center_categories WHERE service_center_id = s.id", "service_center_reference"
	case "brandOptions":
		table, expression = "service_centers", "service_center_reference.value"
		lateralQuery, lateralAlias = "SELECT brand_id::text AS value FROM service_center_brands WHERE service_center_id = s.id", "service_center_reference"
	default:
		return "", nil, ErrReferenceNamesUnavailable
	}
	builder := sql_query.NewSQLSelectBuilder[struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}](table, "s")
	if expandServiceTypes {
		builder.CrossJoinLateral("SELECT unnest(s.service_types) AS service_type", "service_type")
	}
	if lateralQuery != "" {
		builder.CrossJoinLateral(lateralQuery, lateralAlias)
	}
	builder.SelectRaw(expression + " AS label").SelectRaw(expression + " AS value").GroupBy(expression)
	if readAllNonEmpty {
		builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id)"}})
	} else if !totalControl {
		if len(groupIDs) == 0 {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "FALSE"}})
		} else {
			builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "EXISTS (SELECT 1 FROM service_center_groups scope_scg WHERE scope_scg.service_center_id = s.id AND scope_scg.group_id = ANY(?))", ExtraArgs: []any{groupIDs}}})
		}
	}
	builder.Where(sql_query.WhereQuery{"": {Operator: sql_query.SQLOperatorRaw, Value: "NULLIF(BTRIM(" + expression + "), '') IS NOT NULL"}})
	builder.OrderBy([]string{expression}, true)
	query, args, err := builder.Build()
	if err != nil {
		return "", nil, fmt.Errorf("build service center options query: %w", err)
	}
	return query, args, nil
}

func SortOptions(items []dto.OptionItem) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].Label < items[j].Label })
}
