package tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/thomas-caarter-aic/agent-deployment-service/internal/adapters/container"
	"github.com/thomas-caarter-aic/agent-deployment-service/internal/model"
	"github.com/thomas-caarter-aic/agent-deployment-service/internal/orchestrator"
)

type MockAdapter struct {
	mock.Mock
}

func (m *MockAdapter) DeployAgent(ctx context.Context, task model.DeploymentTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}
func (m *MockAdapter) UpdateAgent(ctx context.Context, task model.DeploymentTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}
func (m *MockAdapter) DeleteAgent(ctx context.Context, task model.DeploymentTask) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func TestOrchestrator_SubmitTask(t *testing.T) {
	mockAdapter := new(MockAdapter)
	orchestrator := orchestrator.NewOrchestrator(mockAdapter, 2)

	task := model.DeploymentTask{
		TenantID:     "tenant-test",
		AgentID:      "agent-test",
		ContainerImg: "test-image:v1",
		Action:       "CREATE",
	}

	mockAdapter.On("DeployAgent", mock.Anything, task).Return(nil).Once()

	orchestrator.SubmitTask(task)
	orchestrator.Shutdown()

	mockAdapter.AssertExpectations(t)
}
