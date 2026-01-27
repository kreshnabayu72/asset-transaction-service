package session_log

import (
	"gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/activity/session_log"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func RegisterWorkflows(temporalService service.TemporalServicer, serviceManager service.Managerer) {
	// Register auth activities and workflows
	logActivities := session_log.NewActivities(serviceManager)
	temporalService.RegisterActivity(logActivities.CreateSessionLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteSessionLogActivity)
}
