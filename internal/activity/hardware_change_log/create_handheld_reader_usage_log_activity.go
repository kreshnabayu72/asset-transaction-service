package hardware_change_log

import (
	"context"
	"log/slog"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/db"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/module/analytics_reporting_service/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/provider"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *Activities) CreateHandheldReaderUsageLogActivity(
	ctx context.Context,
	param dto.CreateHandheldReaderUsageLogActivityParam,
) error {
	slog.Info("CreateHandheldReaderUsageLogActivity", "HandheldReaderUsageLogData", param.UsageLogs)
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.HandheldReaderUsageLogModelName)

	usageLogs := make([]any, len(param.UsageLogs))
	for i, changelog := range param.UsageLogs {
		usageLogs[i] = changelog
	}

	_, err := mongoService.InsertMany(
		ctx,
		usageLogs,
	)
	if err != nil {
		return err
	}

	return nil
}

func (a *Activities) DeleteHandheldReaderUsageLogActivity(
	ctx context.Context,
	param dto.DeleteHandheldReaderUsageLogActivityParam,
) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.HandheldReaderUsageLogModelName)

	ids := make([]primitive.ObjectID, len(param.UsageLogs))
	for i, changelog := range param.UsageLogs {
		ids[i] = changelog.ID
	}

	_, err := mongoService.DeleteMany(
		ctx,
		bson.M{
			"_id": bson.M{
				"$in": ids,
			},
		},
	)
	if err != nil {
		return err
	}

	return nil
}
