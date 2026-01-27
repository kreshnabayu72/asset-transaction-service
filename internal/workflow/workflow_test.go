package workflow

import (
	"testing"

	"gitea.tagsamurai.local/BETS-V2/ts-utils/v2/service"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type WorkflowTestSuite struct {
	suite.Suite
}

func TestWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(WorkflowTestSuite))
}

func (s *WorkflowTestSuite) TestRegisterWorkflows() {
	mockTemporalService := new(service.MockTemporalService)
	mockServiceManager := service.MakeManager()

	mockTemporalService.On("RegisterActivity", mock.Anything).Times(14)

	RegisterWorkflows(mockTemporalService, mockServiceManager)

	mockTemporalService.AssertExpectations(s.T())
}
