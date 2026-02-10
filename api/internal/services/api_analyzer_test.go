package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeProbeResults_AllUnreachable(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"message":"hello"}`, Error: "connection refused", StatusCode: 0},
		{Request: `{"prompt":"hello"}`, Error: "connection refused", StatusCode: 0},
		{Request: `{"query":"hello"}`, Error: "dial tcp: timeout", StatusCode: 0},
	}

	result := analyzeProbeResults(probeLog)

	assert.True(t, result.allUnreachable, "should detect all unreachable")
	assert.False(t, result.hasSuccessfulProbe, "should have no successful probes")
	assert.False(t, result.allAuthFailed, "should not be auth failure")
}

func TestAnalyzeProbeResults_AllAuthFailed(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"message":"hello"}`, StatusCode: 401, Response: "Unauthorized"},
		{Request: `{"prompt":"hello"}`, StatusCode: 403, Response: "Forbidden"},
		{Request: `{"query":"hello"}`, StatusCode: 401, Response: "Invalid token"},
	}

	result := analyzeProbeResults(probeLog)

	assert.True(t, result.allAuthFailed, "should detect all auth failed")
	assert.False(t, result.hasSuccessfulProbe, "should have no successful probes")
	assert.False(t, result.allUnreachable, "should not be unreachable")
	assert.Equal(t, 401, result.lastStatusCode)
}

func TestAnalyzeProbeResults_HasSuccessfulProbe(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"message":"hello"}`, StatusCode: 400, Response: "Bad Request"},
		{Request: `{"prompt":"hello"}`, StatusCode: 200, Response: `{"response":"hi"}`},
		{Request: `{"query":"hello"}`, StatusCode: 200, Response: `{"answer":"hello"}`},
	}

	result := analyzeProbeResults(probeLog)

	assert.True(t, result.hasSuccessfulProbe, "should have successful probe")
	assert.False(t, result.allUnreachable, "should not be unreachable")
	assert.False(t, result.allAuthFailed, "should not be auth failure")
}

func TestAnalyzeProbeResults_MixedFailures(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"message":"hello"}`, Error: "connection refused", StatusCode: 0},
		{Request: `{"prompt":"hello"}`, StatusCode: 401, Response: "Unauthorized"},
		{Request: `{"query":"hello"}`, StatusCode: 500, Response: "Internal Server Error"},
	}

	result := analyzeProbeResults(probeLog)

	assert.False(t, result.hasSuccessfulProbe, "should have no successful probes")
	assert.False(t, result.allUnreachable, "should not be all unreachable (mixed failures)")
	assert.False(t, result.allAuthFailed, "should not be all auth failed (mixed failures)")
	assert.Equal(t, 500, result.lastStatusCode)
}

func TestAnalyzeProbeResults_Empty(t *testing.T) {
	result := analyzeProbeResults([]ProbeAttempt{})

	assert.True(t, result.allUnreachable, "empty should be treated as unreachable")
	assert.False(t, result.hasSuccessfulProbe, "should have no successful probes")
}

func TestAnalyzeProbeResults_SingleSuccess(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"messages":[{"role":"user","content":"hello"}]}`, StatusCode: 200, Response: `{"choices":[{"message":{"content":"hi"}}]}`},
	}

	result := analyzeProbeResults(probeLog)

	assert.True(t, result.hasSuccessfulProbe, "should have successful probe")
	assert.False(t, result.allUnreachable, "should not be unreachable")
	assert.False(t, result.allAuthFailed, "should not be auth failure")
	assert.Equal(t, 200, result.lastStatusCode)
}

func TestAnalyzeProbeResults_403Only(t *testing.T) {
	probeLog := []ProbeAttempt{
		{Request: `{"message":"hello"}`, StatusCode: 403, Response: "Forbidden"},
	}

	result := analyzeProbeResults(probeLog)

	assert.True(t, result.allAuthFailed, "single 403 should be auth failure")
	assert.False(t, result.hasSuccessfulProbe, "should have no successful probes")
	assert.Equal(t, 403, result.lastStatusCode)
}

func TestMinConfidenceThreshold(t *testing.T) {
	// Verify the constant is set correctly
	assert.Equal(t, 0.7, MinConfidenceThreshold, "threshold should be 0.7")
}
