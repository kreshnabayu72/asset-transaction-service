package change_log

import (
	"context"
	"log/slog"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/db"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/module/analytics_reporting_service/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/provider"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *Activities) CreateChangeLogActivity(ctx context.Context, param dto.CreateChangeLogActivityParam) error {
	slog.Info("CreateChangeLogActivity", "changeLogData", param.ChangeLogs)
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ChangelogModelName)

	changeLogs := make([]any, len(param.ChangeLogs))
	for i, changelog := range param.ChangeLogs {
		changeLogs[i] = changelog
	}

	_, err := mongoService.InsertMany(
		ctx,
		changeLogs,
	)
	if err != nil {
		return err
	}

	return nil
}

func (a *Activities) DeleteChangeLogActivity(ctx context.Context, param dto.DeleteChangeLogActivityParam) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	analyticsReportingDBName := db.AnalyticsReportingDBName.CompanyDBName(param.ClientID)
	mongoService := dbProvider.MakeMongoDBService(analyticsReportingDBName, db.ChangelogModelName)

	ids := make([]primitive.ObjectID, len(param.ChangeLogs))
	for i, changelog := range param.ChangeLogs {
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
