package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	coreService "gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/dto"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/query_builder"
)

type ServiceCenterEligibilityRepository struct {
	dbName string
}

func NewServiceCenterEligibilityRepository(dbName string) *ServiceCenterEligibilityRepository {
	return &ServiceCenterEligibilityRepository{dbName: dbName}
}

func (r *ServiceCenterEligibilityRepository) LoadEligibilitySnapshot(ctx context.Context, clientID string, serviceCenterID int64) (dto.EligibilitySnapshot, bool, error) {
	centerQuery, centerArgs, err := query_builder.BuildCheckEligibilityCenterQuery(serviceCenterID)
	if err != nil {
		return dto.EligibilitySnapshot{}, false, fmt.Errorf("build service center eligibility query: %w", err)
	}

	svc := coreService.MakePostgreSQLService(clientID + ":" + r.dbName)

	var center query_builder.EligibilityCenterRow
	if err := svc.SelectOne(&center, ctx, centerQuery, centerArgs...); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return dto.EligibilitySnapshot{}, false, nil
		}
		return dto.EligibilitySnapshot{}, false, fmt.Errorf("query service center eligibility: %w", err)
	}

	groupIDs, err := loadEligibilityIDs(ctx, svc, serviceCenterID, query_builder.BuildCheckEligibilityGroupIDsQuery)
	if err != nil {
		return dto.EligibilitySnapshot{}, false, fmt.Errorf("query service center groups: %w", err)
	}
	categoryIDs, err := loadEligibilityIDs(ctx, svc, serviceCenterID, query_builder.BuildCheckEligibilityCategoryIDsQuery)
	if err != nil {
		return dto.EligibilitySnapshot{}, false, fmt.Errorf("query service center categories: %w", err)
	}
	brandIDs, err := loadEligibilityIDs(ctx, svc, serviceCenterID, query_builder.BuildCheckEligibilityBrandIDsQuery)
	if err != nil {
		return dto.EligibilitySnapshot{}, false, fmt.Errorf("query service center brands: %w", err)
	}

	return dto.EligibilitySnapshot{
		Name:         center.Name,
		IsActive:     center.IsActive,
		ServiceTypes: center.ServiceTypes,
		GroupIDs:     groupIDs,
		CategoryIDs:  categoryIDs,
		BrandIDs:     brandIDs,
	}, true, nil
}

type eligibilityIDsQueryBuilder func(int64) (string, []any, error)

func loadEligibilityIDs(ctx context.Context, svc coreService.PostgreSQLServicer, serviceCenterID int64, build eligibilityIDsQueryBuilder) ([]int64, error) {
	query, args, err := build(serviceCenterID)
	if err != nil {
		return nil, fmt.Errorf("build linked ID query: %w", err)
	}
	var result query_builder.EligibilityIDsRow
	if err := svc.SelectOne(&result, ctx, query, args...); err != nil {
		return nil, err
	}
	return result.IDs, nil
}
