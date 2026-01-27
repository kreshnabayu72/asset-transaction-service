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

func (a *Activities) CreateIOTReaderUsageLogActivity(
	ctx context.Context,
	param dto.CreateIOTReaderUsageLogActivityParam,
) error {
	slog.Info("CreateIOTReaderUsageLogActivity", "IOTReaderUsageLogData", param.UsageLogs)
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.IOTReaderUsageLogModelName)

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

func (a *Activities) DeleteIOTReaderUsageLogActivity(
	ctx context.Context,
	param dto.DeleteIOTReaderUsageLogActivityParam,
) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.IOTReaderUsageLogModelName)

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
