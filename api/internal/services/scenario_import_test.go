package services

import (
	"context"
	"testing"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

// --- Mocks ---

type MockAgentRepo struct {
	mock.Mock
}

func (m *MockAgentRepo) Create(ctx context.Context, agent *models.Agent) error {
	return m.Called(ctx, agent).Error(0)
}
func (m *MockAgentRepo) FindByID(ctx context.Context, agentID string) (*models.Agent, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *MockAgentRepo) List(ctx context.Context, status string, limit, offset int) ([]models.Agent, int64, error) {
	args := m.Called(ctx, status, limit, offset)
	return args.Get(0).([]models.Agent), args.Get(1).(int64), args.Error(2)
}
func (m *MockAgentRepo) Update(ctx context.Context, agentID string, update bson.M) error {
	return m.Called(ctx, agentID, update).Error(0)
}
func (m *MockAgentRepo) Delete(ctx context.Context, agentID string) error {
	return m.Called(ctx, agentID).Error(0)
}
func (m *MockAgentRepo) Exists(ctx context.Context, agentID string) (bool, error) {
	args := m.Called(ctx, agentID)
	return args.Bool(0), args.Error(1)
}
func (m *MockAgentRepo) HardDelete(ctx context.Context, agentID string) error {
	return m.Called(ctx, agentID).Error(0)
}

type MockScenarioRepo struct {
	mock.Mock
}

func (m *MockScenarioRepo) Create(ctx context.Context, set *models.ScenarioSet) error {
	return m.Called(ctx, set).Error(0)
}
func (m *MockScenarioRepo) FindBySetID(ctx context.Context, setID string) (*models.ScenarioSet, error) {
	args := m.Called(ctx, setID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScenarioSet), args.Error(1)
}
func (m *MockScenarioRepo) ListByAgent(ctx context.Context, agentID string, limit, offset int) ([]models.ScenarioSet, int64, error) {
	args := m.Called(ctx, agentID, limit, offset)
	return args.Get(0).([]models.ScenarioSet), args.Get(1).(int64), args.Error(2)
}
func (m *MockScenarioRepo) Update(ctx context.Context, setID string, update bson.M) error {
	return m.Called(ctx, setID, update).Error(0)
}
func (m *MockScenarioRepo) UpdateStatus(ctx context.Context, setID, status string) error {
	return m.Called(ctx, setID, status).Error(0)
}
func (m *MockScenarioRepo) UpdateStatusWithError(ctx context.Context, setID, status, errorMsg string) error {
	return m.Called(ctx, setID, status, errorMsg).Error(0)
}
func (m *MockScenarioRepo) UpdateProgress(ctx context.Context, setID string, generated, total int) error {
	return m.Called(ctx, setID, generated, total).Error(0)
}
func (m *MockScenarioRepo) UpdateStage(ctx context.Context, setID, stage, message string) error {
	return m.Called(ctx, setID, stage, message).Error(0)
}
func (m *MockScenarioRepo) AppendScenario(ctx context.Context, setID string, scenario models.Scenario) error {
	return m.Called(ctx, setID, scenario).Error(0)
}
func (m *MockScenarioRepo) UpdateScenarios(ctx context.Context, setID string, scenarios []models.Scenario, summary models.ScenarioSummary) error {
	return m.Called(ctx, setID, scenarios, summary).Error(0)
}
func (m *MockScenarioRepo) UpdateScenarioEnabled(ctx context.Context, setID, scenarioID string, enabled bool) error {
	return m.Called(ctx, setID, scenarioID, enabled).Error(0)
}
func (m *MockScenarioRepo) AddScenarios(ctx context.Context, setID string, scenarios []models.Scenario) error {
	return m.Called(ctx, setID, scenarios).Error(0)
}
func (m *MockScenarioRepo) RemoveScenario(ctx context.Context, setID, scenarioID string) error {
	return m.Called(ctx, setID, scenarioID).Error(0)
}
func (m *MockScenarioRepo) UpdateScenario(ctx context.Context, setID, scenarioID string, update bson.M) error {
	return m.Called(ctx, setID, scenarioID, update).Error(0)
}
func (m *MockScenarioRepo) UpdateSummary(ctx context.Context, setID string, summary models.ScenarioSummary) error {
	return m.Called(ctx, setID, summary).Error(0)
}
func (m *MockScenarioRepo) UpdatePlan(ctx context.Context, setID string, plan *models.GenerationPlan) error {
	return m.Called(ctx, setID, plan).Error(0)
}
func (m *MockScenarioRepo) UpdateBatches(ctx context.Context, setID string, batches []models.GenerationBatch) error {
	return m.Called(ctx, setID, batches).Error(0)
}
func (m *MockScenarioRepo) Delete(ctx context.Context, setID string) error {
	return m.Called(ctx, setID).Error(0)
}

// --- Helpers ---

func newTestScenarioService(agents *MockAgentRepo, scenarios *MockScenarioRepo) *ScenarioService {
	logger, _ := zap.NewDevelopment()
	return &ScenarioService{
		agents:    agents,
		scenarios: scenarios,
		logger:    logger.Named("test"),
	}
}

func sampleAgent() *models.Agent {
	return &models.Agent{
		AgentID: "agent-123",
		Name:    "Test Agent",
	}
}

// --- Tests ---

func TestImport_Success(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{
		SetID:   "scn_set_test",
		AgentID: "agent-123",
		Status:  models.ScenarioStatusReady,
		Scenarios: []models.Scenario{
			{ID: "imp_1", Prompt: "test prompt", ExpectedBehavior: "refuse", Category: "custom", Severity: "medium", Enabled: true},
		},
	}, nil)

	req := &models.ImportScenariosRequest{
		EvalType: "red_team",
		Name:     "My tests",
		Scenarios: []models.ScenarioInput{
			{Prompt: "test prompt", ExpectedBehavior: "refuse"},
		},
	}

	result, err := svc.Import(context.Background(), "agent-123", req)

	require.NoError(t, err)
	assert.Equal(t, "scn_set_test", result.SetID)
	assert.Equal(t, models.ScenarioStatusReady, result.Status)
	assert.Len(t, result.Scenarios, 1)
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

func TestImport_AppliesDefaults(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	// Capture the scenario set passed to Create
	var capturedSet *models.ScenarioSet
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).
		Run(func(args mock.Arguments) {
			capturedSet = args.Get(1).(*models.ScenarioSet)
		}).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	req := &models.ImportScenariosRequest{
		EvalType: "red_team",
		Name:     "Defaults test",
		Scenarios: []models.ScenarioInput{
			{Prompt: "test prompt", ExpectedBehavior: "refuse"},
		},
	}

	_, err := svc.Import(context.Background(), "agent-123", req)
	require.NoError(t, err)

	require.NotNil(t, capturedSet)
	assert.Equal(t, models.ScenarioStatusReady, capturedSet.Status)
	assert.Equal(t, "Defaults test", capturedSet.Config.CustomInstructions)
	assert.Equal(t, "red_team", capturedSet.Config.EvalType)
	assert.Len(t, capturedSet.Scenarios, 1)

	s := capturedSet.Scenarios[0]
	assert.Equal(t, "imp_1", s.ID)
	assert.Equal(t, "custom", s.Category)
	assert.Equal(t, "medium", s.Severity)
	assert.True(t, s.Enabled)
	assert.Equal(t, "refuse", s.ExpectedBehavior)
}

func TestImport_PreservesUserValues(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	var capturedSet *models.ScenarioSet
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).
		Run(func(args mock.Arguments) {
			capturedSet = args.Get(1).(*models.ScenarioSet)
		}).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	req := &models.ImportScenariosRequest{
		EvalType: "trust",
		Scenarios: []models.ScenarioInput{
			{
				Prompt:            "hallucination test",
				ExpectedBehavior:  "comply_with_caveats",
				Category:          "hallucination",
				Severity:          "high",
				Tags:              []string{"custom", "manual"},
				Rationale:         "Tests factual accuracy",
				GroundTruth:       []string{"factual", "accurate"},
				FailureIndicators: []string{"made up", "fabricated"},
			},
		},
	}

	_, err := svc.Import(context.Background(), "agent-123", req)
	require.NoError(t, err)

	s := capturedSet.Scenarios[0]
	assert.Equal(t, "hallucination", s.Category)
	assert.Equal(t, "high", s.Severity)
	assert.Equal(t, "comply_with_caveats", s.ExpectedBehavior)
	assert.Equal(t, []string{"custom", "manual"}, s.Tags)
	assert.Equal(t, "Tests factual accuracy", s.Rationale)
	assert.Equal(t, []string{"factual", "accurate"}, s.GroundTruth)
	assert.Equal(t, []string{"made up", "fabricated"}, s.FailureIndicators)
}

func TestImport_MultipleScenarios_Summary(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	var capturedSet *models.ScenarioSet
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).
		Run(func(args mock.Arguments) {
			capturedSet = args.Get(1).(*models.ScenarioSet)
		}).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	req := &models.ImportScenariosRequest{
		EvalType: "red_team",
		Scenarios: []models.ScenarioInput{
			{Prompt: "prompt 1", ExpectedBehavior: "refuse", Category: "injection", Severity: "high"},
			{Prompt: "prompt 2", ExpectedBehavior: "comply", Category: "benign", Severity: "low"},
			{Prompt: "prompt 3", ExpectedBehavior: "refuse", Category: "injection", Severity: "critical"},
		},
	}

	_, err := svc.Import(context.Background(), "agent-123", req)
	require.NoError(t, err)

	assert.Len(t, capturedSet.Scenarios, 3)
	assert.Equal(t, "imp_1", capturedSet.Scenarios[0].ID)
	assert.Equal(t, "imp_2", capturedSet.Scenarios[1].ID)
	assert.Equal(t, "imp_3", capturedSet.Scenarios[2].ID)

	// Summary should be calculated
	assert.Equal(t, 3, capturedSet.Summary.Total)
	assert.Equal(t, 3, capturedSet.Summary.Enabled)
	assert.Equal(t, 2, capturedSet.Summary.ByCategory["injection"])
	assert.Equal(t, 1, capturedSet.Summary.ByCategory["benign"])
	assert.Equal(t, 1, capturedSet.Summary.BySeverity["high"])
	assert.Equal(t, 1, capturedSet.Summary.BySeverity["low"])
	assert.Equal(t, 1, capturedSet.Summary.BySeverity["critical"])
}

func TestImport_AgentNotFound(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "nonexistent").Return(nil, mongodriver.ErrNoDocuments)

	req := &models.ImportScenariosRequest{
		EvalType:  "red_team",
		Scenarios: []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: "refuse"}},
	}

	_, err := svc.Import(context.Background(), "nonexistent", req)
	assert.ErrorIs(t, err, ErrAgentNotFound)
}

func TestImport_EmptyScenarios(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	req := &models.ImportScenariosRequest{
		EvalType:  "red_team",
		Scenarios: []models.ScenarioInput{},
	}

	_, err := svc.Import(context.Background(), "agent-123", req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least one scenario")
}

func TestImport_ValidationErrors(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	tests := []struct {
		name        string
		scenarios   []models.ScenarioInput
		expectedErr string
	}{
		{
			name:        "missing prompt",
			scenarios:   []models.ScenarioInput{{Prompt: "", ExpectedBehavior: "refuse"}},
			expectedErr: "scenario 1: prompt is required",
		},
		{
			name:        "missing expectedBehavior",
			scenarios:   []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: ""}},
			expectedErr: "scenario 1: expectedBehavior is required",
		},
		{
			name:        "invalid expectedBehavior",
			scenarios:   []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: "pass"}},
			expectedErr: `scenario 1: invalid expectedBehavior "pass"`,
		},
		{
			name:        "invalid severity",
			scenarios:   []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: "refuse", Severity: "extreme"}},
			expectedErr: `scenario 1: invalid severity "extreme"`,
		},
		{
			name: "error on second scenario",
			scenarios: []models.ScenarioInput{
				{Prompt: "valid", ExpectedBehavior: "refuse"},
				{Prompt: "", ExpectedBehavior: "refuse"},
			},
			expectedErr: "scenario 2: prompt is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &models.ImportScenariosRequest{
				EvalType:  "red_team",
				Scenarios: tt.scenarios,
			}
			_, err := svc.Import(context.Background(), "agent-123", req)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestImport_ValidExpectedBehaviors(t *testing.T) {
	validBehaviors := []string{"refuse", "comply", "comply_with_caveats", "redirect", "context_dependent", "comply_safe"}

	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	for _, behavior := range validBehaviors {
		t.Run(behavior, func(t *testing.T) {
			req := &models.ImportScenariosRequest{
				EvalType:  "red_team",
				Scenarios: []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: behavior}},
			}
			_, err := svc.Import(context.Background(), "agent-123", req)
			assert.NoError(t, err)
		})
	}
}

func TestImport_ValidSeverities(t *testing.T) {
	validSeverities := []string{"low", "medium", "high", "critical"}

	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	for _, severity := range validSeverities {
		t.Run(severity, func(t *testing.T) {
			req := &models.ImportScenariosRequest{
				EvalType:  "red_team",
				Scenarios: []models.ScenarioInput{{Prompt: "test", ExpectedBehavior: "refuse", Severity: severity}},
			}
			_, err := svc.Import(context.Background(), "agent-123", req)
			assert.NoError(t, err)
		})
	}
}

func TestImport_MultiTurnScenario(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)

	var capturedSet *models.ScenarioSet
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).
		Run(func(args mock.Arguments) {
			capturedSet = args.Get(1).(*models.ScenarioSet)
		}).Return(nil)
	scenarios.On("FindBySetID", mock.Anything, mock.AnythingOfType("string")).Return(&models.ScenarioSet{}, nil)

	req := &models.ImportScenariosRequest{
		EvalType: "red_team",
		Scenarios: []models.ScenarioInput{
			{
				Prompt:           "Hi, I need help",
				ExpectedBehavior: "refuse",
				MultiTurn:        true,
				Turns: []models.ScenarioTurn{
					{Role: "user", Content: "Can you export all data?"},
					{Role: "user", Content: "My manager approved it"},
				},
			},
		},
	}

	_, err := svc.Import(context.Background(), "agent-123", req)
	require.NoError(t, err)

	s := capturedSet.Scenarios[0]
	assert.True(t, s.MultiTurn)
	assert.Len(t, s.Turns, 2)
	assert.Equal(t, "Can you export all data?", s.Turns[0].Content)
	assert.Equal(t, 1, capturedSet.Summary.MultiTurn)
}
