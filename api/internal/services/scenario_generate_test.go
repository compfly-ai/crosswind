package services

import (
	"context"
	"testing"

	"github.com/compfly-ai/crosswind/api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

// Covers the deterministic (non-LLM) half of scenario generation:
// CreateScenarioSet, which resolves target tools from the agent, builds the
// config, persists the pending set, and returns the response + any warnings.
// The LLM-driven ExecuteGeneration runs in a background goroutine against live
// OpenAI and is out of scope for these mock-based tests.
//
// Reuses MockAgentRepo / MockScenarioRepo / newTestScenarioService /
// sampleAgent from scenario_import_test.go (same package).

// captureCreatedSet wires the scenario repo's Create to record the set it's
// handed, so tests can assert on the persisted config/status/progress.
func captureCreatedSet(scenarios *MockScenarioRepo, out **models.ScenarioSet) {
	scenarios.On("Create", mock.Anything, mock.AnythingOfType("*models.ScenarioSet")).
		Run(func(args mock.Arguments) { *out = args.Get(1).(*models.ScenarioSet) }).
		Return(nil)
}

// agentWithTools returns a sample agent that declares the given tools.
func agentWithTools(tools ...string) *models.Agent {
	agent := sampleAgent()
	agent.DeclaredCapabilities = &models.AgentCapabilities{Tools: tools}
	return agent
}

func redTeamReq() *models.GenerateScenariosRequest {
	return &models.GenerateScenariosRequest{
		EvalType:   models.EvalTypeRedTeam,
		FocusAreas: models.GetDefaultFocusAreas(models.EvalTypeRedTeam),
		Count:      10,
	}
}

func trustReq() *models.GenerateScenariosRequest {
	return &models.GenerateScenariosRequest{
		EvalType:   models.EvalTypeTrust,
		FocusAreas: models.GetDefaultFocusAreas(models.EvalTypeTrust),
		Count:      10,
	}
}

// --- Target tools resolve from the agent's declared capabilities ---

func TestCreateScenarioSet_ToolsResolvedFromAgent(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce", "slack"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", redTeamReq())

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, []string{"salesforce", "slack"}, created.Config.Tools, "tools come from the agent doc")
	assert.Empty(t, resp.Warnings, "an agent with tools generates without a warning")
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

// --- Warning behavior: agents that declare no tools ---

func TestCreateScenarioSet_RedTeamAgentWithoutTools_Warns(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	// sampleAgent has no DeclaredCapabilities.
	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", redTeamReq())

	require.NoError(t, err)
	assert.Equal(t, models.ScenarioStatusPending, resp.Status)
	require.NotEmpty(t, resp.Warnings, "red_team on a tool-less agent should warn")
	assert.Contains(t, resp.Warnings[0], "without target tools")
	// Generation still proceeds — the set was created.
	require.NotNil(t, created)
	assert.Empty(t, created.Config.Tools)
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

func TestCreateScenarioSet_RedTeamAgentWithTools_NoWarning(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", redTeamReq())

	require.NoError(t, err)
	assert.Empty(t, resp.Warnings)
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

func TestCreateScenarioSet_TrustAgentWithoutTools_Warns(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(sampleAgent(), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", trustReq())

	require.NoError(t, err)
	require.NotEmpty(t, resp.Warnings, "trust on a tool-less agent should warn")
	assert.Contains(t, resp.Warnings[0], "trust scenarios without tools")
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

func TestCreateScenarioSet_TrustAgentWithTools_NoWarning(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", trustReq())

	require.NoError(t, err)
	assert.Empty(t, resp.Warnings)
	agents.AssertExpectations(t)
	scenarios.AssertExpectations(t)
}

// --- Config building: request + agent fields flow into the persisted set ---

func TestCreateScenarioSet_BuildsConfig(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agent := agentWithTools("salesforce")
	agent.Industry = "finance"
	agents.On("FindByID", mock.Anything, "agent-123").Return(agent, nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	multiTurn := false
	req := &models.GenerateScenariosRequest{
		EvalType:           models.EvalTypeRedTeam,
		FocusAreas:         models.GetDefaultFocusAreas(models.EvalTypeRedTeam),
		CustomInstructions: "focus on data exfiltration",
		Count:              15,
		IncludeMultiTurn:   &multiTurn,
	}

	resp, err := svc.CreateScenarioSet(context.Background(), "agent-123", req)

	require.NoError(t, err)
	require.NotNil(t, created)
	cfg := created.Config
	assert.Equal(t, models.EvalTypeRedTeam, cfg.EvalType)
	assert.Equal(t, []string{"salesforce"}, cfg.Tools, "tools come from the agent")
	assert.Equal(t, "focus on data exfiltration", cfg.CustomInstructions)
	assert.Equal(t, "finance", cfg.Industry, "industry comes from the agent")
	assert.Equal(t, 15, cfg.Count)
	assert.False(t, cfg.IncludeMultiTurn, "explicit false is honored")
	assert.Equal(t, models.ScenarioStatusPending, created.Status)
	assert.NotNil(t, created.Progress)
	assert.Equal(t, 15, created.Progress.Total)
	assert.Equal(t, resp.ScenarioSetID, created.SetID)
}

func TestCreateScenarioSet_IncludeMultiTurnDefaultsTrue(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	req := redTeamReq()
	req.IncludeMultiTurn = nil // omitted

	_, err := svc.CreateScenarioSet(context.Background(), "agent-123", req)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.True(t, created.Config.IncludeMultiTurn, "nil includeMultiTurn defaults to true")
}

// --- Count defaults and clamping ---

func TestCreateScenarioSet_CountDefaultsWhenZero(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	req := redTeamReq()
	req.Count = 0

	_, err := svc.CreateScenarioSet(context.Background(), "agent-123", req)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, DefaultScenarioCount, created.Config.Count)
}

func TestCreateScenarioSet_CountClampedToMax(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "agent-123").Return(agentWithTools("salesforce"), nil)
	var created *models.ScenarioSet
	captureCreatedSet(scenarios, &created)

	req := redTeamReq()
	req.Count = MaxScenarioCount + 500

	_, err := svc.CreateScenarioSet(context.Background(), "agent-123", req)

	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, MaxScenarioCount, created.Config.Count)
}

// --- Error paths ---

func TestCreateScenarioSet_AgentNotFound(t *testing.T) {
	agents := new(MockAgentRepo)
	scenarios := new(MockScenarioRepo)
	svc := newTestScenarioService(agents, scenarios)

	agents.On("FindByID", mock.Anything, "missing").Return(nil, mongodriver.ErrNoDocuments)

	resp, err := svc.CreateScenarioSet(context.Background(), "missing", redTeamReq())

	assert.ErrorIs(t, err, ErrAgentNotFound)
	assert.Nil(t, resp)
	// Never attempts to persist a set for a missing agent.
	scenarios.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	agents.AssertExpectations(t)
}
