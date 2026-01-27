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

func (a *Activities) CreateReaderTaggingLogActivity(
	ctx context.Context,
	param dto.CreateReaderTaggingLogActivityParam,
) error {
	slog.Info("CreateReaderTaggingLogActivity", "LogData", param.Logs)
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ReaderTaggingLogsModelName)

	logs := make([]any, len(param.Logs))
	for i, log := range param.Logs {
		logs[i] = log
	}

	_, err := mongoService.InsertMany(
		ctx,
		logs,
	)
	if err != nil {
		return err
	}

	return nil
}

func (a *Activities) DeleteReaderTaggingLogActivity(
	ctx context.Context,
	param dto.DeleteReaderTaggingLogActivityParam,
) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ReaderTaggingLogsModelName)

	ids := make([]primitive.ObjectID, len(param.Logs))
	for i, log := range param.Logs {
		ids[i] = log.ID
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
