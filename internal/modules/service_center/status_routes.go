package service_center

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/handler"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/repository"
	"gitea.tagsamurai.local/BETS-V2/asset-transaction-service/internal/modules/service_center/service"
	analyticsTemporal "gitea.tagsamurai.local/BETS-V2/service-contracts/services/analytics_reporting/temporal"
	"github.com/gofiber/fiber/v3"
	temporalclient "go.temporal.io/sdk/client"
)

const serviceCenterStatusPath = "/service-centers/:id/status"

// SetupStatusRoutes mounts the status operation with an optional best-effort
// analytics change-log dispatcher.
func SetupStatusRoutes(app *fiber.App, dbName string, dispatcher service.StatusChangeLogDispatcher) {
	statusRepository := repository.NewStatusRepository(dbName)
	statusService := service.NewStatusOperationService(statusRepository, dispatcher)
	statusHandler := handler.NewStatusHandler(statusService)

	app.Patch(serviceCenterStatusPath, statusHandler.PatchStatus)
}

type temporalStatusChangeLogDispatcher struct {
	client    temporalclient.Client
	taskQueue string
}

// NewTemporalStatusChangeLogDispatcher builds a best-effort adapter for the
// analytics-reporting RecordChangeLogsWorkflow. A nil client or blank queue
// disables dispatch without preventing local status writes.
func NewTemporalStatusChangeLogDispatcher(
	client temporalclient.Client,
	taskQueue string,
) service.StatusChangeLogDispatcher {
	if client == nil || strings.TrimSpace(taskQueue) == "" {
		return nil
	}
	return &temporalStatusChangeLogDispatcher{client: client, taskQueue: taskQueue}
}

func (d *temporalStatusChangeLogDispatcher) RecordChangeLogs(
	ctx context.Context,
	param analyticsTemporal.RecordChangeLogsParam,
) error {
	if len(param.ChangeLogs) == 0 {
		return nil
	}
	startCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	workflowID := fmt.Sprintf("service-center-changelog-%s-%d", param.ClientID, time.Now().UnixNano())
	_, err := d.client.ExecuteWorkflow(
		startCtx,
		temporalclient.StartWorkflowOptions{ID: workflowID, TaskQueue: d.taskQueue},
		analyticsTemporal.RecordChangeLogsWorkflowName,
		param,
	)
	return err
}
