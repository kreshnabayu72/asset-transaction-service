package hardware_change_log

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/db"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/module/analytics_reporting_service/dto"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/module/analytics_reporting_service/model"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/provider"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func TestCreateIOTReaderUsageLogActivity(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockServiceManager := service.MakeManager()
		mockDBProvider := new(provider.MockDBProvider)
		mockMongoDBService := new(service.MockMongoDBService)

		activities := NewActivities(mockServiceManager)

		param := dto.CreateIOTReaderUsageLogActivityParam{
			ClientID: "client_id",
			UsageLogs: []model.IOTReaderUsageLog{
				{ID: primitive.NewObjectID()},
				{ID: primitive.NewObjectID()},
			},
		}

		mockServiceManager.Register("DBProvider", mockDBProvider)
		mockDBProvider.On("MakeMongoDBService", mock.Anything, db.IOTReaderUsageLogModelName).
			Return(mockMongoDBService).
			Once()
		mockMongoDBService.On("InsertMany", mock.Anything, mock.MatchedBy(func(docs []interface{}) bool {
			return len(docs) == 2
		}), mock.Anything).Return([]interface{}{}, nil).Once()

		err := activities.CreateIOTReaderUsageLogActivity(context.Background(), param)
		assert.NoError(t, err)

		mockDBProvider.AssertExpectations(t)
		mockMongoDBService.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockServiceManager := service.MakeManager()
		mockDBProvider := new(provider.MockDBProvider)
		mockMongoDBService := new(service.MockMongoDBService)

		activities := NewActivities(mockServiceManager)

		param := dto.CreateIOTReaderUsageLogActivityParam{
			ClientID: "client_id",
			UsageLogs: []model.IOTReaderUsageLog{
				{ID: primitive.NewObjectID()},
			},
		}

		mockServiceManager.Register("DBProvider", mockDBProvider)
		mockDBProvider.On("MakeMongoDBService", mock.Anything, db.IOTReaderUsageLogModelName).
			Return(mockMongoDBService).
			Once()
		mockMongoDBService.On("InsertMany", mock.Anything, mock.Anything, mock.Anything).
			Return([]interface{}{}, errors.New("insert error")).
			Once()

		err := activities.CreateIOTReaderUsageLogActivity(context.Background(), param)
		assert.Error(t, err)
		assert.Equal(t, "insert error", err.Error())

		mockDBProvider.AssertExpectations(t)
		mockMongoDBService.AssertExpectations(t)
	})
}

func TestDeleteIOTReaderUsageLogActivity(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mockServiceManager := service.MakeManager()
		mockDBProvider := new(provider.MockDBProvider)
		mockMongoDBService := new(service.MockMongoDBService)

		activities := NewActivities(mockServiceManager)

		param := dto.DeleteIOTReaderUsageLogActivityParam{
			ClientID: "client_id",
			UsageLogs: []model.IOTReaderUsageLog{
				{ID: primitive.NewObjectID()},
			},
		}

		mockServiceManager.Register("DBProvider", mockDBProvider)
		mockDBProvider.On("MakeMongoDBService", mock.Anything, db.IOTReaderUsageLogModelName).
			Return(mockMongoDBService).
			Once()
		mockMongoDBService.On("DeleteMany", mock.Anything, mock.Anything, mock.Anything).Return(int64(1), nil).Once()

		err := activities.DeleteIOTReaderUsageLogActivity(context.Background(), param)
		assert.NoError(t, err)

		mockDBProvider.AssertExpectations(t)
		mockMongoDBService.AssertExpectations(t)
	})

	t.Run("Error", func(t *testing.T) {
		mockServiceManager := service.MakeManager()
		mockDBProvider := new(provider.MockDBProvider)
		mockMongoDBService := new(service.MockMongoDBService)

		activities := NewActivities(mockServiceManager)

		param := dto.DeleteIOTReaderUsageLogActivityParam{
			ClientID: "client_id",
			UsageLogs: []model.IOTReaderUsageLog{
				{ID: primitive.NewObjectID()},
			},
		}

		mockServiceManager.Register("DBProvider", mockDBProvider)
		mockDBProvider.On("MakeMongoDBService", mock.Anything, db.IOTReaderUsageLogModelName).
			Return(mockMongoDBService).
			Once()
		mockMongoDBService.On("DeleteMany", mock.Anything, mock.Anything, mock.Anything).
			Return(int64(0), errors.New("delete error")).
			Once()

		err := activities.DeleteIOTReaderUsageLogActivity(context.Background(), param)
		assert.Error(t, err)
		assert.Equal(t, "delete error", err.Error())

		mockDBProvider.AssertExpectations(t)
		mockMongoDBService.AssertExpectations(t)
	})
}
