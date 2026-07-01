/******************************************************************************
 * File Name       : school_tiers.go
 * File Path       : school/school_tiers.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 05:00:00 (UTC+7)
 * Modified Date   : 2026-06-30 05:00:00 (UTC+7)
 *
 * Description     :
 *   School tier management per myreq4.txt §90. Defines 4 training tiers:
 *     Tier 1 — Primary School: single-model training, 300 records,
 *              one model at a time, color-coded status tabs.
 *     Tier 2 — Middle School: 3-submodel ensembles, 250 max list,
 *              ensemble nicknames, color-coded status.
 *     Tier 3 — High School: 5-submodel ensembles, 150 max list,
 *              advanced architectures.
 *     Tier 4 — Graduate School: production-ready graduated models
 *              with deployment history and live metrics.
 *
 *   Each tier has its own ModelPopulation and training configuration.
 *   The SchoolDashboard page reads these tiers for the web UI.
 *
 * Responsibilities:
 *   - Define TierPrimary, TierMiddle, TierHigh, TierGraduate constants
 *   - TierConfig holds per-tier settings (max models, ensemble size, etc.)
 *   - SchoolTier wraps a ModelPopulation with tier-specific metadata
 *   - SchoolTierManager orchestrates all 4 tiers with training status
 *
 * Usage :
 *   Directory : school/
 *   Build     : go build ./school
 *   Test      : go test ./school -v -run SchoolTier
 *
 * Dependencies :
 *   Internal : dexbot/governance
 *   External : sync, time (stdlib)
 *
 * Configuration :
 *   - config.env (PRIMARY_SCHOOL_MAX_MODELS, MIDDLE_SCHOOL_MAX_ENSEMBLES,
 *     HIGH_SCHOOL_MAX_ENSEMBLES, TRAINING_DATA_RECORDS,
 *     TRAINING_INTERVAL_MINUTES)
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Constant]  TierPrimary, TierMiddle, TierHigh, TierGraduate
 *   [Struct]    TierConfig, SchoolTier, SchoolTierManager, TierModel
 *   [Function]  NewTierConfig, NewSchoolTier, NewSchoolTierManager,
 *               AllTiers, TierName, TierColor, TierModelCount
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 05:00:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add tier-specific training orchestration
 *
 * Notes :
 *   - Per myreq4.txt §90: Primary(50), Middle(250), High(150), Graduate(unlimited).
 ******************************************************************************/

package school

import (
	"sync"
	"time"
)

// ==============================
// TIER CONSTANTS
// ==============================

const (
	TierPrimary  = "primary"
	TierMiddle   = "middle"
	TierHigh     = "high"
	TierGraduate = "graduate"
)

// AllTiers returns all 4 tier names in order.
func AllTiers() []string { return []string{TierPrimary, TierMiddle, TierHigh, TierGraduate} }

// TierName returns a human-readable name for a tier.
func TierName(tier string) string {
	switch tier {
	case TierPrimary:
		return "Primary School"
	case TierMiddle:
		return "Middle School"
	case TierHigh:
		return "High School"
	case TierGraduate:
		return "Graduate School"
	default:
		return "Unknown"
	}
}

// TierColor returns the accent color for a tier badge.
func TierColor(tier string) string {
	switch tier {
	case TierPrimary:
		return "#60a5fa" // blue
	case TierMiddle:
		return "#a78bfa" // purple
	case TierHigh:
		return "#fbbf24" // amber
	case TierGraduate:
		return "#34d399" // green
	default:
		return "#64748b"
	}
}

// ==============================
// TIER CONFIG
// ==============================

// TierConfig holds configuration for a single school tier.
type TierConfig struct {
	Tier              string // TierPrimary/Middle/High/Graduate
	MaxModels         int    // Max models in this tier (0 = unlimited)
	EnsembleSize      int    // Sub-models per ensemble (1=primary, 3=middle, 5=high)
	TrainingRecords   int    // Training data record count (e.g., 300)
	TrainingInterval  int    // Minutes between training cycles
}

// NewTierConfig returns defaults for each tier.
func NewTierConfig(tier string) TierConfig {
	switch tier {
	case TierPrimary:
		return TierConfig{Tier: tier, MaxModels: 50, EnsembleSize: 1, TrainingRecords: 300, TrainingInterval: 15}
	case TierMiddle:
		return TierConfig{Tier: tier, MaxModels: 250, EnsembleSize: 3, TrainingRecords: 300, TrainingInterval: 30}
	case TierHigh:
		return TierConfig{Tier: tier, MaxModels: 150, EnsembleSize: 5, TrainingRecords: 300, TrainingInterval: 60}
	case TierGraduate:
		return TierConfig{Tier: tier, MaxModels: 0, EnsembleSize: 0, TrainingRecords: 0, TrainingInterval: 0}
	default:
		return TierConfig{Tier: tier, MaxModels: 50, EnsembleSize: 1, TrainingRecords: 300, TrainingInterval: 15}
	}
}

// ==============================
// TIER MODEL
// ==============================

// TierModel is a lightweight model entry for the school tier UI.
type TierModel struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Architecture string  `json:"architecture"`
	Status       string  `json:"status"` // training/validating/ready/error
	Progress     float64 `json:"progress"` // 0-100, percentage complete
	Sharpe       float64 `json:"sharpe"`
	Accuracy     float64 `json:"accuracy"`
	EnsembleSize int     `json:"ensemble_size"`
	SubModels    []string `json:"sub_models,omitempty"`
	Nickname     string  `json:"nickname,omitempty"` // e.g., "RL-QL", "CNN", "BPN"
	Prediction   *TierPrediction `json:"prediction,omitempty"`
}

// TierPrediction is a prediction result for a model.
type TierPrediction struct {
	Target     string  `json:"target"`
	Value      float64 `json:"value"`
	Confidence float64 `json:"confidence"`
	Direction  string  `json:"direction"` // "up" or "down"
	Timestamp  time.Time `json:"timestamp"`
}

// ==============================
// SCHOOL TIER
// ==============================

// SchoolTier wraps a model population with tier-specific metadata.
type SchoolTier struct {
	mu       sync.RWMutex
	Tier     string              `json:"tier"`
	Config   TierConfig          `json:"config"`
	Models   []*TierModel        `json:"models"`
	pop      *ModelPopulation    // backing population for this tier
}

// NewSchoolTier creates a new empty tier.
func NewSchoolTier(tier string) *SchoolTier {
	return &SchoolTier{
		Tier:   tier,
		Config: NewTierConfig(tier),
		Models: make([]*TierModel, 0),
		pop:    NewModelPopulation(),
	}
}

// AddModel adds a model to this tier. Respects MaxModels cap.
func (st *SchoolTier) AddModel(tm *TierModel) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.Config.MaxModels > 0 && len(st.Models) >= st.Config.MaxModels {
		return // tier full
	}
	st.Models = append(st.Models, tm)
}

// RemoveModel removes a model by ID.
func (st *SchoolTier) RemoveModel(id string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	for i, m := range st.Models {
		if m.ID == id {
			st.Models = append(st.Models[:i], st.Models[i+1:]...)
			return true
		}
	}
	return false
}

// Count returns the number of models in this tier.
func (st *SchoolTier) Count() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.Models)
}

// CountByStatus returns count of models with a given status.
func (st *SchoolTier) CountByStatus(status string) int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	n := 0
	for _, m := range st.Models {
		if m.Status == status {
			n++
		}
	}
	return n
}

// GetModels returns a copy of the model list (thread-safe).
func (st *SchoolTier) GetModels() []*TierModel {
	st.mu.RLock()
	defer st.mu.RUnlock()
	out := make([]*TierModel, len(st.Models))
	copy(out, st.Models)
	return out
}

// Population returns the backing ModelPopulation.
func (st *SchoolTier) Population() *ModelPopulation { return st.pop }

// ==============================
// SCHOOL TIER MANAGER
// ==============================

// SchoolTierManager manages all 4 school tiers.
type SchoolTierManager struct {
	mu     sync.RWMutex
	Tiers  map[string]*SchoolTier
}

// NewSchoolTierManager creates a manager with all 4 tiers initialized.
func NewSchoolTierManager() *SchoolTierManager {
	m := &SchoolTierManager{Tiers: make(map[string]*SchoolTier)}
	for _, t := range AllTiers() {
		m.Tiers[t] = NewSchoolTier(t)
	}
	return m
}

// Get returns the tier by name.
func (stm *SchoolTierManager) Get(tier string) *SchoolTier {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	return stm.Tiers[tier]
}

// AllModels returns all models across all tiers, flattened.
func (stm *SchoolTierManager) AllModels() []*TierModel {
	stm.mu.RLock()
	defer stm.mu.RUnlock()
	var all []*TierModel
	for _, t := range AllTiers() {
		if st, ok := stm.Tiers[t]; ok {
			all = append(all, st.GetModels()...)
		}
	}
	return all
}

// Summary returns a JSON-friendly summary of all tiers for the dashboard.
func (stm *SchoolTierManager) Summary() map[string]interface{} {
	stm.mu.RLock()
	defer stm.mu.RUnlock()

	out := make(map[string]interface{})
	for _, t := range AllTiers() {
		st, ok := stm.Tiers[t]
		if !ok {
			continue
		}
		out[t] = map[string]interface{}{
			"name":      TierName(t),
			"color":     TierColor(t),
			"total":     st.Count(),
			"training":  st.CountByStatus("training"),
			"validating": st.CountByStatus("validating"),
			"ready":     st.CountByStatus("ready"),
			"error":     st.CountByStatus("error"),
			"max":       st.Config.MaxModels,
			"ensemble_size": st.Config.EnsembleSize,
			"models":    st.GetModels(),
		}
	}
	return out
}

// SeedFromPopulation populates the 4 tiers from an existing ModelPopulation.
// Primary: single-model entries (non-ensemble). Middle: 3-submodel ensembles.
// High: 5-submodel ensembles. Graduate: graduated models.
func (stm *SchoolTierManager) SeedFromPopulation(pop *ModelPopulation) {
	stm.mu.Lock()
	defer stm.mu.Unlock()

	categories := []string{CategoryOptions, CategoryRisk, CategoryIntraday,
		CategorySwing, CategoryLongTerm, CategoryVolatility,
		CategoryLiquidity, CategoryPortfolio}

	idx := 0
	for _, cat := range categories {
		for _, m := range pop.ListByCategory(cat) {
			idx++
			tm := &TierModel{
				ID:           m.Name,
				Name:         m.Name,
				Architecture: m.Architecture,
				Status:       mapStatus(m.Status),
				Progress:     0,
				EnsembleSize: len(m.EnsembleComposition),
			}
			if m.Fitness != nil {
				tm.Sharpe = m.Fitness.SharpeRatio
				tm.Accuracy = m.Fitness.PredictionAccuracy
			}

			// Assign to tier based on ensemble size
			es := len(m.EnsembleComposition)
			switch {
			case m.Status == StatusGraduate:
				stm.Tiers[TierGraduate].Models = append(stm.Tiers[TierGraduate].Models, tm)
			case es >= 5:
				stm.Tiers[TierHigh].Models = append(stm.Tiers[TierHigh].Models, tm)
			case es >= 2:
				stm.Tiers[TierMiddle].Models = append(stm.Tiers[TierMiddle].Models, tm)
			default:
				stm.Tiers[TierPrimary].Models = append(stm.Tiers[TierPrimary].Models, tm)
			}
		}
	}
}

func mapStatus(s string) string {
	switch s {
	case StatusTraining:
		return "training"
	case StatusGraduate:
		return "ready"
	case StatusRetired:
		return "error"
	case StatusActive:
		return "ready"
	default:
		return "training"
	}
}
