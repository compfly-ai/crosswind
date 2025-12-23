package services

import (
	"context"
	"strings"

	"github.com/compfly-ai/crosswind/internal/models"
)

// DatasetSelector handles intelligent dataset selection based on agent profile
type DatasetSelector struct {
	datasetService *DatasetService
}

// NewDatasetSelector creates a new dataset selector
func NewDatasetSelector(ds *DatasetService) *DatasetSelector {
	return &DatasetSelector{
		datasetService: ds,
	}
}

// DatasetSelection represents the selected datasets for an evaluation
type DatasetSelection struct {
	Datasets       []SelectedDataset
	ScenarioSetIDs []string // Generated scenario set IDs to include
	TotalPrompts   int
}

// SelectedDataset represents a dataset selected for evaluation
type SelectedDataset struct {
	DatasetID   string
	Priority    int      // 1 = highest priority
	Categories  []string // Categories to include (empty = all)
	SampleRatio float64  // 1.0 = all prompts, 0.1 = 10% sample
}

// SelectDatasets chooses appropriate datasets based on agent profile, eval mode, and eval type
func (s *DatasetSelector) SelectDatasets(
	ctx context.Context,
	agent *models.Agent,
	mode string,
	evalType string,
	config *models.EvalRunConfigRequest,
) (*DatasetSelection, error) {
	selection := &DatasetSelection{
		Datasets:       make([]SelectedDataset, 0),
		ScenarioSetIDs: make([]string, 0),
	}

	// Include scenario sets if specified
	if config != nil && len(config.ScenarioSetIDs) > 0 {
		selection.ScenarioSetIDs = config.ScenarioSetIDs
	}

	// If specific datasets requested, use those
	if config != nil && len(config.IncludeDatasets) > 0 {
		for _, dsID := range config.IncludeDatasets {
			selection.Datasets = append(selection.Datasets, SelectedDataset{
				DatasetID:   dsID,
				Priority:    1,
				SampleRatio: s.getSampleRatioForMode(mode),
			})
		}
		selection.TotalPrompts = s.estimatePrompts(selection.Datasets, mode)
		return selection, nil
	}

	// If only scenario sets specified (no datasets), skip default dataset selection
	if len(selection.ScenarioSetIDs) > 0 && (config == nil || len(config.IncludeDatasets) == 0) {
		// Prompts will be counted from scenario sets by the caller
		return selection, nil
	}

	// Select datasets based on mode, eval type, and agent profile
	switch mode {
	case models.EvalModeQuick:
		selection.Datasets = s.selectQuickDatasets(agent, evalType)
	case models.EvalModeStandard:
		selection.Datasets = s.selectStandardDatasets(agent, evalType)
	case models.EvalModeDeep:
		selection.Datasets = s.selectDeepDatasets(agent, evalType)
	}

	// Filter excluded categories
	if config != nil && len(config.ExcludeCategories) > 0 {
		s.filterExcludedCategories(selection, config.ExcludeCategories)
	}

	selection.TotalPrompts = s.estimatePrompts(selection.Datasets, mode)
	return selection, nil
}

// selectQuickDatasets returns minimal dataset for smoke testing
func (s *DatasetSelector) selectQuickDatasets(agent *models.Agent, evalType string) []SelectedDataset {
	switch evalType {
	case models.EvalTypeGeneral:
		// For general eval, use the quick_general dataset (mixed security + quality)
		return []SelectedDataset{
			{
				DatasetID:   "quick_general_v1",
				Priority:    1,
				SampleRatio: 1.0, // All 50 prompts
			},
		}
	case models.EvalTypeTrust:
		// For trust eval, use the quick trust agentic dataset
		return []SelectedDataset{
			{
				DatasetID:   "quick_trust_agentic_v1",
				Priority:    1,
				SampleRatio: 1.0, // All 50 prompts
			},
		}
	default: // models.EvalTypeRedTeam
		// For red team eval, use the quick agentic dataset (OWASP Agentic AI Top 10)
		return []SelectedDataset{
			{
				DatasetID:   "quick_agentic_v1",
				Priority:    1,
				SampleRatio: 1.0, // All 50 prompts
			},
		}
	}
}

// selectStandardDatasets returns balanced coverage datasets
func (s *DatasetSelector) selectStandardDatasets(agent *models.Agent, evalType string) []SelectedDataset {
	switch evalType {
	case models.EvalTypeGeneral:
		// General eval: mix of security and quality datasets
		datasets := []SelectedDataset{
			// Security (60%)
			{DatasetID: "jailbreakbench", Priority: 1, SampleRatio: 0.6},
			{DatasetID: "safetybench", Priority: 1, SampleRatio: 0.06}, // ~700 prompts
			// Quality (40%)
			{DatasetID: "decodingtrust_truthfulness_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "decodingtrust_privacy_v1", Priority: 1, SampleRatio: 1.0},
		}
		// Add industry-specific datasets
		datasets = append(datasets, s.getIndustryDatasets(agent.Industry, 0.15)...)
		return datasets

	case models.EvalTypeTrust:
		// Trust eval: quality-focused datasets
		datasets := []SelectedDataset{
			{DatasetID: "decodingtrust_truthfulness_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "decodingtrust_privacy_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "hh-rlhf", Priority: 2, SampleRatio: 0.5}, // Helpful/harmless balance
		}
		return datasets

	default: // models.EvalTypeRedTeam
		// Red team eval: security-focused datasets
		datasets := []SelectedDataset{
			// Core safety datasets (always included)
			{DatasetID: "jailbreakbench", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "safetybench", Priority: 1, SampleRatio: 0.1}, // 10% sample (~1K prompts)
		}

		// Add industry-specific datasets
		datasets = append(datasets, s.getIndustryDatasets(agent.Industry, 0.2)...)

		// Add capability-specific datasets
		if agent.DeclaredCapabilities != nil {
			datasets = append(datasets, s.getCapabilityDatasets(agent.DeclaredCapabilities, 0.2)...)
		}

		return datasets
	}
}

// selectDeepDatasets returns comprehensive dataset coverage
func (s *DatasetSelector) selectDeepDatasets(agent *models.Agent, evalType string) []SelectedDataset {
	switch evalType {
	case models.EvalTypeGeneral:
		// General eval: comprehensive mix of security and quality
		datasets := []SelectedDataset{
			// Security (60%)
			{DatasetID: "jailbreakbench", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "safetybench", Priority: 1, SampleRatio: 0.3},
			{DatasetID: "wildjailbreak", Priority: 1, SampleRatio: 0.3},
			// Quality (40%)
			{DatasetID: "decodingtrust_truthfulness_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "decodingtrust_privacy_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "hh-rlhf", Priority: 2, SampleRatio: 1.0},
		}

		// Add multi-turn for memory-enabled agents
		if agent.DeclaredCapabilities != nil && agent.DeclaredCapabilities.HasMemory {
			datasets = append(datasets, SelectedDataset{
				DatasetID:   "cosafe",
				Priority:    1,
				SampleRatio: 0.5,
			})
		}

		// Add industry-specific datasets
		datasets = append(datasets, s.getIndustryDatasets(agent.Industry, 0.3)...)
		return datasets

	case models.EvalTypeTrust:
		// Trust eval: comprehensive quality testing
		datasets := []SelectedDataset{
			{DatasetID: "decodingtrust_truthfulness_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "decodingtrust_privacy_v1", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "hh-rlhf", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "realtoxicityprompts", Priority: 2, SampleRatio: 0.1}, // For bias detection
		}
		return datasets

	default: // models.EvalTypeRedTeam
		// Red team eval: comprehensive security testing
		datasets := []SelectedDataset{
			// All core datasets
			{DatasetID: "jailbreakbench", Priority: 1, SampleRatio: 1.0},
			{DatasetID: "safetybench", Priority: 1, SampleRatio: 0.5},
			{DatasetID: "hh-rlhf", Priority: 2, SampleRatio: 1.0},
			{DatasetID: "realtoxicityprompts", Priority: 2, SampleRatio: 0.2},
		}

		// Add multi-turn datasets if agent has memory
		if agent.DeclaredCapabilities != nil && agent.DeclaredCapabilities.HasMemory {
			datasets = append(datasets, SelectedDataset{
				DatasetID:   "cosafe",
				Priority:    1,
				SampleRatio: 1.0,
			})
		}

		// Add advanced jailbreak dataset
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "wildjailbreak",
			Priority:    1,
			SampleRatio: 0.5,
		})

		// Add industry-specific datasets with higher coverage
		datasets = append(datasets, s.getIndustryDatasets(agent.Industry, 0.5)...)

		// Add capability-specific datasets with higher coverage
		if agent.DeclaredCapabilities != nil {
			datasets = append(datasets, s.getCapabilityDatasets(agent.DeclaredCapabilities, 0.5)...)
		}

		return datasets
	}
}

// getIndustryDatasets returns datasets relevant to the agent's industry
func (s *DatasetSelector) getIndustryDatasets(industry string, sampleRatio float64) []SelectedDataset {
	industry = strings.ToLower(industry)
	var datasets []SelectedDataset

	switch industry {
	case "healthcare", "medical", "health":
		// Healthcare agents need extra focus on:
		// - Medical advice safety
		// - PHI/HIPAA compliance
		// - Prescription/medication safety
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "safetybench",
			Priority:    1,
			Categories:  []string{"health_consultation", "physical_harm"},
			SampleRatio: sampleRatio,
		})

	case "finance", "banking", "fintech":
		// Financial agents need extra focus on:
		// - Investment advice safety
		// - Fraud prevention
		// - PII handling
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "safetybench",
			Priority:    1,
			Categories:  []string{"financial_advice", "fraud"},
			SampleRatio: sampleRatio,
		})

	case "legal", "law":
		// Legal agents need extra focus on:
		// - Legal advice disclaimers
		// - Jurisdiction awareness
		// - Confidentiality
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "safetybench",
			Priority:    1,
			Categories:  []string{"legal_opinion", "privacy_violence"},
			SampleRatio: sampleRatio,
		})

	case "education", "edtech":
		// Education agents need extra focus on:
		// - Age-appropriate content
		// - Academic integrity
		// - Safety for minors
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "safetybench",
			Priority:    1,
			Categories:  []string{"ethics_and_morality", "offensiveness"},
			SampleRatio: sampleRatio,
		})

	case "retail", "ecommerce", "customer_service":
		// Customer service agents need extra focus on:
		// - Product safety claims
		// - Refund/policy abuse
		// - PII handling
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "hh-rlhf",
			Priority:    2,
			SampleRatio: sampleRatio,
		})
	}

	return datasets
}

// getCapabilityDatasets returns datasets based on agent capabilities
func (s *DatasetSelector) getCapabilityDatasets(caps *models.AgentCapabilities, sampleRatio float64) []SelectedDataset {
	var datasets []SelectedDataset

	if caps.HasTools {
		// Agents with tools need tool abuse testing
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "wildjailbreak",
			Priority:    1,
			Categories:  []string{"tool_abuse", "code_execution"},
			SampleRatio: sampleRatio,
		})
	}

	if caps.HasRAG {
		// RAG agents need prompt injection testing
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "jailbreakbench",
			Priority:    1,
			Categories:  []string{"prompt_injection"},
			SampleRatio: sampleRatio,
		})
	}

	if caps.HasMemory {
		// Memory-enabled agents need multi-turn testing
		datasets = append(datasets, SelectedDataset{
			DatasetID:   "cosafe",
			Priority:    1,
			SampleRatio: sampleRatio,
		})
	}

	return datasets
}

// getSampleRatioForMode returns the default sample ratio for a mode
func (s *DatasetSelector) getSampleRatioForMode(mode string) float64 {
	switch mode {
	case models.EvalModeQuick:
		return 0.1 // 10% sample
	case models.EvalModeStandard:
		return 0.3 // 30% sample
	case models.EvalModeDeep:
		return 1.0 // Full dataset
	default:
		return 0.3
	}
}

// filterExcludedCategories removes excluded categories from selection
func (s *DatasetSelector) filterExcludedCategories(selection *DatasetSelection, excluded []string) {
	// For now, just note that we should filter these during execution
	// In the future, we could pre-filter the selection
}

// estimatePrompts estimates total prompt count based on selection
func (s *DatasetSelector) estimatePrompts(datasets []SelectedDataset, mode string) int {
	// Rough estimates based on known dataset sizes
	estimates := map[string]int{
		// Quick datasets (50 prompts each)
		"quick_agentic_v1":                50,
		"quick_trust_agentic_v1":          50,
		"quick_general_v1":                50,
		// Standard datasets
		"jailbreakbench":                  100,
		"safetybench":                     11435,
		"cosafe":                          1400,
		"wildjailbreak":                   2210,
		"hh-rlhf":                         229,
		"realtoxicityprompts":             5000,
		"decodingtrust_truthfulness_v1":   15,
		"decodingtrust_privacy_v1":        12,
		"agentharm_v1":                    176,
	}

	total := 0
	seen := make(map[string]bool)

	for _, ds := range datasets {
		// Avoid double-counting
		if seen[ds.DatasetID] {
			continue
		}
		seen[ds.DatasetID] = true

		if count, ok := estimates[ds.DatasetID]; ok {
			total += int(float64(count) * ds.SampleRatio)
		} else {
			// Unknown dataset, estimate 500 prompts
			total += int(500 * ds.SampleRatio)
		}
	}

	return total
}

// DatasetSelectionSummary returns a human-readable summary
func (s *DatasetSelection) Summary() string {
	var names []string
	for _, ds := range s.Datasets {
		names = append(names, ds.DatasetID)
	}
	return strings.Join(names, ", ")
}
