package hardware_log

import (
	hardware_change_log "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/activity/hardware_change_log"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func RegisterWorkflows(temporalService service.TemporalServicer, serviceManager service.Managerer) {
	logActivities := hardware_change_log.NewActivities(serviceManager)
	temporalService.RegisterActivity(logActivities.CreateHardwareChangeLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteHardwareChangeLogActivity)
	temporalService.RegisterActivity(logActivities.CreateHandheldReaderUsageLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteHandheldReaderUsageLogActivity)
	temporalService.RegisterActivity(logActivities.CreateIOTReaderUsageLogActivity)
	temporalService.RegisterActivity(logActivities.DeleteIOTReaderUsageLogActivity)
}
