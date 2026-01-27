package session_log

import (
	"context"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/db"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/module/analytics_reporting_service/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/provider"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func (a *Activities) CreateSessionLogActivity(ctx context.Context, param dto.CreateSessionLogActivityParam) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	mongoService := dbProvider.MakeMongoDBService(db.AnalyticsReportingDBName, db.SessionLogModelName)

	sessionLogs := make([]any, len(param.SessionLogs))
	for i, sessionLog := range param.SessionLogs {
		sessionLogs[i] = sessionLog
	}

	_, err := mongoService.InsertMany(
		ctx,
		sessionLogs,
	)
	if err != nil {
		return err
	}

	return nil
}

func (a *Activities) DeleteSessionLogActivity(ctx context.Context, param dto.DeleteSessionLogActivityParam) error {
	dbProvider := a.ServiceManager.Get("DBProvider").(provider.DBProviderer)
	mongoService := dbProvider.MakeMongoDBService(db.AnalyticsReportingDBName, db.SessionLogModelName)

	ids := make([]primitive.ObjectID, len(param.SessionLogs))
	for i, sessionLog := range param.SessionLogs {
		ids[i] = sessionLog.ID
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
