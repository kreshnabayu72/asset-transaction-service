package repository

import (
	"context"
	"errors"
	"fmt"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	service "gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
	"github.com/jackc/pgx/v5"
)

type ReadServiceCenterRepository struct {
	dbName string
}

func NewReadServiceCenterRepository(dbName string) *ReadServiceCenterRepository {
	return &ReadServiceCenterRepository{dbName: dbName}
}

func (r *ReadServiceCenterRepository) List(
	ctx context.Context,
	clientID string,
	request dto.ListServiceCentersRequest,
	visibleColumns []string,
	access dto.ServiceCenterReadAccess,
) ([]query_builder.ReadCenterRow, int, error) {
	query, args, err := query_builder.BuildListServiceCentersQuery(request.ServiceCenterReadFilters, visibleColumns, access.GroupIDs, access.TotalControl, access.ReadOnly, request.Page, request.Limit)
	if err != nil {
		return nil, 0, err
	}
	db := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	var result struct {
		Data         []query_builder.ReadCenterRow `json:"data"`
		TotalRecords int                           `json:"totalRecords"`
	}
	if err := db.SelectOne(&result, ctx, query, args...); err != nil {
		return nil, 0, fmt.Errorf("select service center page: %w", err)
	}
	if result.Data == nil {
		result.Data = []query_builder.ReadCenterRow{}
	}
	return result.Data, result.TotalRecords, nil
}

func (r *ReadServiceCenterRepository) ExportRows(
	ctx context.Context,
	clientID string,
	request dto.ExportServiceCentersRequest,
	visibleColumns []string,
	access dto.ServiceCenterReadAccess,
) ([]query_builder.ReadCenterRow, error) {
	query, args, err := query_builder.BuildExportServiceCentersQuery(request.ServiceCenterReadFilters, visibleColumns, access.GroupIDs, access.TotalControl, access.ReadOnly)
	if err != nil {
		return nil, err
	}
	db := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	var result []query_builder.ReadCenterRow
	if err := db.SelectMany(&result, ctx, query, args...); err != nil {
		return nil, fmt.Errorf("select service center export rows: %w", err)
	}
	if result == nil {
		result = []query_builder.ReadCenterRow{}
	}
	return result, nil
}

func (r *ReadServiceCenterRepository) Get(
	ctx context.Context,
	clientID string,
	id int64,
	access dto.ServiceCenterReadAccess,
) (service.ReadServiceCenterDetail, bool, error) {
	query, args, err := query_builder.BuildDetailServiceCenterQuery(id, access.GroupIDs, access.TotalControl, access.ReadOnly)
	if err != nil {
		return service.ReadServiceCenterDetail{}, false, err
	}
	db := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	var center query_builder.ReadCenterRow
	if err := db.SelectOne(&center, ctx, query, args...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ReadServiceCenterDetail{}, false, nil
		}
		return service.ReadServiceCenterDetail{}, false, fmt.Errorf("select service center: %w", err)
	}
	contactsQuery, contactsArgs, err := query_builder.BuildContactPersonsQuery(id)
	if err != nil {
		return service.ReadServiceCenterDetail{}, false, err
	}
	var contacts []query_builder.ContactPersonRow
	if err := db.SelectMany(&contacts, ctx, contactsQuery, contactsArgs...); err != nil {
		return service.ReadServiceCenterDetail{}, false, fmt.Errorf("select service center contacts: %w", err)
	}
	if contacts == nil {
		contacts = []query_builder.ContactPersonRow{}
	}
	return service.ReadServiceCenterDetail{Center: center, Contacts: contacts}, true, nil
}

func (r *ReadServiceCenterRepository) Options(
	ctx context.Context,
	clientID string,
	facets map[string]bool,
	access dto.ServiceCenterReadAccess,
) (map[string][]dto.OptionItem, error) {
	db := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	result := make(map[string][]dto.OptionItem, len(facets))
	for facet := range facets {
		query, args, err := query_builder.BuildOptionsValueQuery(facet, access.GroupIDs, access.TotalControl, access.ReadOnly)
		if err != nil {
			return nil, err
		}
		var options []dto.OptionItem
		if err := db.SelectMany(&options, ctx, query, args...); err != nil {
			return nil, fmt.Errorf("select service center %s: %w", facet, err)
		}
		if options == nil {
			options = []dto.OptionItem{}
		}
		result[facet] = options
	}
	return result, nil
}
