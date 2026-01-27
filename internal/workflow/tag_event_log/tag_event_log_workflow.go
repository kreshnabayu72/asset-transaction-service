package tag_event_log

import (
	tag_event_log "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/activity/tag_event_log"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func RegisterWorkflows(temporalService service.TemporalServicer, serviceManager service.Managerer) {
	logActivities := tag_event_log.NewActivities(serviceManager)
	temporalService.RegisterActivity(logActivities.CreateTagEventLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteTagEventLogActivity)
	temporalService.RegisterActivity(logActivities.CreateTagEventLogAuditActivity)
	temporalService.RegisterActivity(logActivities.DeleteTagEventLogAuditActivity)
}
