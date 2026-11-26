package repository

import (
	"context"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"
)

type ServiceCenterRepository struct {
	dbName string
}

func NewServiceCenterRepository(dbName string) *ServiceCenterRepository {
	return &ServiceCenterRepository{dbName: dbName}
}

func (r *ServiceCenterRepository) CheckNameAvailability(ctx context.Context, clientID, name string, excludeID *int64) (bool, error) {
	query, args, err := query_builder.BuildCheckAvailabilityQuery(name, excludeID)
	if err != nil {
		return false, err
	}

	svc := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	var result query_builder.CheckAvailabilityRow
	if err := svc.SelectOne(&result, ctx, query, args...); err != nil {
		return false, err
	}
	return result.Available, nil
}
