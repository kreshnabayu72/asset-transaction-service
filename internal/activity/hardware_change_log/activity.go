package hardware_change_log

import (
	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
)

// Activities holds the service dependencies for Temporal activities.
type Activities struct {
	ServiceManager service.Managerer
}

// NewActivities creates a new Activities instance with the provided service manager.
func NewActivities(serviceManager service.Managerer) *Activities {
	return &Activities{
		ServiceManager: serviceManager,
	}
}
