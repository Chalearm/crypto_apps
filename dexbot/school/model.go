/******************************************************************************
 * File Name       : model.go
 * File Path       : school/model.go
  *
  * Function Name : Count
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
  *
  * Function Name : Graduate
  * Purpose :
  *   Performs its designated operation.
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:48 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:48 (UTC+7)
 *
 * Description     :
 *   Shared School daemon types: model metadata, fitness history, model population management (graduate, retire, rank). Extracted from apps/school/main.go during Phase 7 reorganization per myreq2.txt §1, §
 *
 * Responsibilities:
 *   - - Implement core functionality for school package.
 *
 * Usage :
 *   Directory : school/
 *
 *   Build :
 *     go build ./school
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./school
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/school
 *
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Functions] All exported functions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:48 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package school

import (
	"sort"
	"sync"
	"time"

	"dexbot/infra"
)

// ==============================
// MODEL CATEGORIES
// ==============================

// Model categories per myreq2.txt §9.
const (
	CategoryOptions        = "Options Prediction"
	CategoryRisk           = "Risk Management"
	CategoryIntraday       = "Intraday Trading"
	CategorySwing          = "Swing Trading"
	CategoryLongTerm       = "Long-Term Investment"
	CategoryVolatility     = "Volatility Forecasting"
	CategoryLiquidity      = "Liquidity Analysis"
	CategoryPortfolio      = "Portfolio Optimization"
)

// Model statuses.
const (
	StatusTraining  = "training"
	StatusActive    = "active"
	StatusGraduate  = "graduate"
	StatusRetired   = "retired"
)

// ==============================
// MODEL METADATA
// ==============================

/*
Struct: ModelMetadata
Description:
  Complete metadata for a machine learning model in the School daemon.
  Per myreq2.txt §11, every model carries version, generation, dataset version,
  feature set, hyperparameters, architecture, and ensemble composition.

Fields:
  - Name                   string                    : Model name
  - Version                string                    : Version string
  - Generation             int                       : Evolutionary generation number
  - Category               string                    : Category constant (e.g., CategoryOptions)
  - TrainingDatasetVersion string                    : Dataset version used for training
  - FeatureSet             []string                  : Feature names used
  - Hyperparameters        map[string]string         : Hyperparameter key-value pairs
  - Architecture           string                    : Model architecture description
  - EnsembleComposition    map[string]float64        : Sub-model voting weights (sum to 1.0)
  - Fitness                *FitnessHistory           : Latest fitness snapshot
  - FitnessTimeline        []FitnessHistory          : Historical fitness records
  - Status                 string                    : training/active/graduate/retired
  - CreatedAt              time.Time                 : Creation timestamp
  - GraduatedAt            *time.Time                : When promoted to graduate (nil if not)

Lines: ~15
*/
type ModelMetadata struct {
	Name                   string
	Version                string
	Generation             int
	Category               string
	TrainingDatasetVersion string
	FeatureSet             []string
	Hyperparameters        map[string]string
	Architecture           string
	EnsembleComposition    map[string]float64
	Fitness                *FitnessHistory
	FitnessTimeline        []FitnessHistory
	Status                 string
	CreatedAt              time.Time
	GraduatedAt            *time.Time
}

/******************************************************************************
 * Function Name : ValidateEnsembleWeights
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : ValidateEnsembleWeights
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (m *ModelMetadata) ValidateEnsembleWeights() bool {
	if len(m.EnsembleComposition) == 0 {
		return false
	}
	sum := 0.0
	for _, w := range m.EnsembleComposition {
		sum += w
	}
	return sum >= 0.99 && sum <= 1.01
}

// ==============================
// FITNESS HISTORY
// ==============================

/*
Struct: FitnessHistory
Description:
  Records a fitness evaluation snapshot for a model at a point in time.
  Per myreq2.txt §16, fitness functions include Sharpe, Sortino, profit,
  drawdown, accuracy, capital efficiency, volatility control, execution
  quality, and consistency.

Fields:
  - Timestamp          time.Time : When fitness was evaluated
  - SharpeRatio        float64   : Sharpe ratio
  - SortinoRatio       float64   : Sortino ratio (downside deviation)
  - Profit             float64   : Total profit
  - Drawdown           float64   : Maximum drawdown (positive = worst)
  - PredictionAccuracy float64   : Prediction accuracy (0-100)
  - CapitalEfficiency  float64   : Capital efficiency score
  - VolatilityControl  float64   : Volatility control score
  - ExecutionQuality   float64   : Execution quality score
  - Consistency        float64   : Performance consistency score

Lines: ~12
*/
type FitnessHistory struct {
	Timestamp          time.Time
	SharpeRatio        float64
	SortinoRatio       float64
	Profit             float64
	Drawdown           float64
	PredictionAccuracy float64
	CapitalEfficiency  float64
	VolatilityControl  float64
	ExecutionQuality   float64
	Consistency        float64
}

// ==============================
// MODEL POPULATION
// ==============================

/*
Struct: ModelPopulation
Description:
  Thread-safe collection of models managed by the School daemon.
  Supports add, graduate, retire, rank, and iteration.

Fields:
  - mu     sync.RWMutex               : Protects concurrent access
  - models map[string]*ModelMetadata  : All models by name

Lines: ~5
*/
type ModelPopulation struct {
	mu     sync.RWMutex
	models map[string]*ModelMetadata
}

/******************************************************************************
 * Function Name : NewModelPopulation
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : NewModelPopulation
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func NewModelPopulation() *ModelPopulation {
	infra.FnTrace("entering")
	return &ModelPopulation{
		models: make(map[string]*ModelMetadata),
	}
}

/******************************************************************************
 * Function Name : AddModel
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) AddModel(m *ModelMetadata) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.models[m.Name] = m
}

/******************************************************************************
 * Function Name : Graduate
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) Graduate(name string) bool {
	infra.FnTrace("entering")
	p.mu.Lock()
	defer p.mu.Unlock()

	m, ok := p.models[name]
	if !ok {
		return false
	}
	now := time.Now()
	m.Status = StatusGraduate
	m.GraduatedAt = &now
	return true
}

/******************************************************************************
 * Function Name : Retire
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : Retire
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (p *ModelPopulation) Retire(name string) bool {
	infra.FnTrace("entering")
	p.mu.Lock()
	defer p.mu.Unlock()

	m, ok := p.models[name]
	if !ok {
		return false
	}
	m.Status = StatusRetired
	return true
}

/******************************************************************************
 * Function Name : Get
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) Get(name string) *ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.models[name]
}

/******************************************************************************
 * Function Name : Count
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.models)
}

/******************************************************************************
 * Function Name : ActiveCount
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : ActiveCount
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (p *ModelPopulation) ActiveCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := 0
	for _, m := range p.models {
		if m.Status != StatusRetired {
			n++
		}
	}
	return n
}

/******************************************************************************
 * Function Name : GraduateCount
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) GraduateCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	n := 0
	for _, m := range p.models {
		if m.Status == StatusGraduate {
			n++
		}
	}
	return n
}

/******************************************************************************
 * Function Name : Graduates
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : Graduates
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (p *ModelPopulation) Graduates() []*ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*ModelMetadata
	for _, m := range p.models {
		if m.Status == StatusGraduate {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

/******************************************************************************
 * Function Name : ListByCategory
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (p *ModelPopulation) ListByCategory(category string) []*ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var result []*ModelMetadata
	for _, m := range p.models {
		if m.Category == category {
			result = append(result, m)
		}
	}
	return result
}

/******************************************************************************
 * Function Name : Remove
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

  *
  * Function Name : Remove
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (p *ModelPopulation) Remove(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.models, name)
}

/******************************************************************************
 * Function Name : RankBySharpe
 *
 * Purpose :
 *   Returns models sorted by Sharpe ratio (descending) within a category.
 *   Per myreq3.txt §41: supports multiple ranking categories.
 *
 * Inputs :
 *   category  string — Category constant ("" = all categories)
 *   limit     int    — Max results (0 = unlimited)
 *
 * Return :
 *   Type        : []*ModelMetadata
 *   Description : Sorted models by descending Sharpe ratio.
 *
 * Complexity : Time O(n log n), Space O(n)
 *
 * Error Cases :
 *   - None
 * Number Of Lines : 22
 ******************************************************************************/
  *
  * Function Name : RankBySharpe
  * Purpose :
  *   Performs its designated operation.
  * Inputs :
  *   None (see function signature)
  * Complexity :
  *   Time  : O(1)
  *   Space : O(1)
  * Error Cases :
  *   - None
  * Number Of Lines :
  *   10
func (p *ModelPopulation) RankBySharpe(category string, limit int) []*ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*ModelMetadata
	for _, m := range p.models {
		if category != "" && m.Category != category {
			continue
		}
		if m.Fitness != nil {
			candidates = append(candidates, m)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Fitness.SharpeRatio > candidates[j].Fitness.SharpeRatio
	})

	if limit > 0 && limit < len(candidates) {
		return candidates[:limit]
	}
	return candidates
}

/******************************************************************************
 * Function Name : RankBySortino
 *
 * Purpose :
 *   Returns models sorted by Sortino ratio (descending), optionally filtered.
 *
 * Inputs :
 *   category  string
 *   limit     int
 *
 * Return :
 *   Type        : []*ModelMetadata
 *
 * Complexity : Time O(n log n), Space O(n)
 *
 * Error Cases :
 *   - None
 * Number Of Lines : 20
 ******************************************************************************/
func (p *ModelPopulation) RankBySortino(category string, limit int) []*ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*ModelMetadata
	for _, m := range p.models {
		if category != "" && m.Category != category {
			continue
		}
		if m.Fitness != nil {
			candidates = append(candidates, m)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Fitness.SortinoRatio > candidates[j].Fitness.SortinoRatio
	})

	if limit > 0 && limit < len(candidates) {
		return candidates[:limit]
	}
	return candidates
}

/******************************************************************************
 * Function Name : RankByAccuracy
 *
 * Purpose :
 *   Returns models sorted by prediction accuracy (descending).
 *
 * Complexity : Time O(n log n), Space O(n)
 *
 * Inputs :
 *   None (see function signature)
 *
 * Error Cases :
 *   - None
 * Number Of Lines : 20
 ******************************************************************************/
func (p *ModelPopulation) RankByAccuracy(category string, limit int) []*ModelMetadata {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*ModelMetadata
	for _, m := range p.models {
		if category != "" && m.Category != category {
			continue
		}
		if m.Fitness != nil {
			candidates = append(candidates, m)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Fitness.PredictionAccuracy > candidates[j].Fitness.PredictionAccuracy
	})

	if limit > 0 && limit < len(candidates) {
		return candidates[:limit]
	}
	return candidates
}

/******************************************************************************
 * Function Name : UpdateEnsembleWeights
 *
 * Purpose :
 *   Dynamically adjusts ensemble voting weights based on live performance
 *   metrics reported by Trading (§58). Higher-performing models gain weight;
 *   underperforming models lose weight. Weights normalize to sum 1.0.
 *
 * Inputs :
 *   model       *ModelMetadata — Ensemble model to update
 *   performance map[string]float64 — model_type → recent Sharpe
 *   decay       float64 — Smoothing factor (0.0 = no change, 1.0 = full replace)
 *
 * Return :
 *   Type        : bool
 *   Description : true if weights were changed.
 *
 * Complexity : Time O(c) where c = number of sub-models
 * Number Of Lines : 30
 *****************************************************************************
  * Error Cases :
  *   - None
  *
 */
func (p *ModelPopulation) UpdateEnsembleWeights(model *ModelMetadata, performance map[string]float64, decay float64) bool {
	if model == nil || len(model.EnsembleComposition) == 0 {
		return false
	}
	if decay <= 0 {
		decay = 0.2
	}
	if decay > 1.0 {
		decay = 1.0
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Calculate new weights proportional to performance
	totalPerf := 0.0
	for _, perf := range performance {
		if perf > 0 {
			totalPerf += perf
		}
	}
	if totalPerf <= 0 {
		return false
	}

	changed := false
	newWeights := make(map[string]float64)
	for subModel, perf := range performance {
		oldW := model.EnsembleComposition[subModel]
		newW := perf / totalPerf
		// Smooth blending
		blended := oldW*(1-decay) + newW*decay
		if absFloat(blended-oldW) > 0.01 {
			changed = true
		}
		newWeights[subModel] = blended
	}

	if changed {
		model.EnsembleComposition = newWeights
	}
	return changed
}
