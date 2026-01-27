package change_log

import (
	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/activity/change_log"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func RegisterWorkflows(temporalService service.TemporalServicer, serviceManager service.Managerer) {
	logActivities := change_log.NewActivities(serviceManager)
	temporalService.RegisterActivity(logActivities.CreateChangeLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteChangeLogActivity)
}
