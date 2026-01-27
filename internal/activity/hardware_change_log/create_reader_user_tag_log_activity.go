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

func (a *Activities) CreateReaderUserTagLogActivity(
	ctx context.Context,
	param dto.CreateReaderUserTagLogActivityParam,
) error {
	slog.Info("CreateReaderUserTagLogActivity", "LogData", param.Logs)
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ReaderUserTagLogsModelName)

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

func (a *Activities) DeleteReaderUserTagLogActivity(
	ctx context.Context,
	param dto.DeleteReaderUserTagLogActivityParam,
) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ReaderUserTagLogsModelName)

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
