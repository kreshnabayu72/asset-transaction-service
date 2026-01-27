package workflow

import (
	change_log_workflow "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/workflow/change_log"
	hardware_change_log_workflow "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/workflow/hardware_change_log"
	session_log_workflow "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/workflow/session_log"
	tag_event_log_workflow "gitea.tagsamurai.local/BETS-V2/analytics-reporting-service/internal/workflow/tag_event_log"
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

func RegisterWorkflows(
	temporalService service.TemporalServicer,
	serviceManager service.Managerer,
) {
	change_log_workflow.RegisterWorkflows(temporalService, serviceManager)
	hardware_change_log_workflow.RegisterWorkflows(temporalService, serviceManager)
	session_log_workflow.RegisterWorkflows(temporalService, serviceManager)
	tag_event_log_workflow.RegisterWorkflows(temporalService, serviceManager)
}
