/******************************************************************************
 * File Name       : model_registry.go
 * File Path       : governance/model_registry.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-28 15:00:00 (UTC+7)
 * Modified Date   : 2026-06-28 17:00:00 (UTC+7)
 *
 * Description     :
 *   Centralized Model Registry for all model lifecycle tracking.
 *
 * Responsibilities:
 *   - Register experimental/graduated/retired models with independent versioning
 *   - Track fitness snapshots per generation
 *   - Store ensemble definitions with voting weights
 *   - Record deployment history per trading agent
 *   - Track live performance time-series
 *
 * Usage :
 *   Directory : governance/
 *   Build     : go build ./governance
 *   Run       : (library — imported by daemons)
 *   Test      : go test ./governance -v -run ModelRegistry
 *
 * Dependencies :
 *   Internal : None
 *   External : None (stdlib only)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Struct]  ModelRegistry, ModelRecord, FitnessSnapshot, EnsembleDef,
 *             DeploymentRecord, PerformancePoint
 *   [Function] NewModelRegistry, Register, Get, Graduate, Retire,
 *             ListByStatus, ListByCategory, Count, CountByStatus, Remove,
 *             RecordDeployment, RecordPerformance, RecordFitness, AllIDs
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-28 15:00:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add event sourcing for full audit trail
 *
 * Notes :
 *   - Per myreq3.txt §33: model versioning is independent of software versioning.
 *   - Fitness history capped at 100 entries per model.
 *   - Registry history capped at 500 entries.
 *   - Performance history capped at 200 data points per model.
 ******************************************************************************/

package governance

import (
	"fmt"
	"sync"
	"time"
)

// ==============================
// MODEL STATUS CONSTANTS
// ==============================

const (
	ModelStatusExperimental = "experimental"
	ModelStatusGraduated    = "graduated"
	ModelStatusRetired      = "retired"
	ModelStatusActive       = "active"
	ModelStatusTraining     = "training"
)

// ==============================
// MODEL RECORD
// ==============================

/*
Struct: ModelRecord
Description:
  Complete lifecycle record for a single model in the registry.
  Model versioning is independent of software versioning (§33).

Fields:
  - ID                string            : Unique model identifier (UUID or name)
  - ModelVersion      string            : Semantic model version (e.g., "v3.2")
  - Generation        int               : Evolutionary generation number
  - Category          string            : Model category (Options, Risk, etc.)
  - Architecture      string            : Model architecture (LSTM, XGBoost, etc.)
  - Framework         string            : Training framework (Python/TF, Go, etc.)
  - Status            string            : experimental/graduated/retired/active/training
  - Hyperparameters   map[string]string : Training hyperparameters
  - FeatureSet        []string          : Feature names used
  - TrainingDataset   string            : Dataset version used for training
  - FitnessScores     []FitnessSnapshot : Per-generation fitness history
  - Ensemble          *EnsembleDef      : Ensemble composition (if ensemble model)
  - CreatedAt         time.Time         : First registered
  - GraduatedAt       *time.Time        : When promoted to graduate
  - RetiredAt         *time.Time        : When retired
  - Deployments       []DeploymentRecord : Trading deployment history
  - PerformanceHistory []PerformancePoint : Live trading performance over time

Lines: ~15
*/
type ModelRecord struct {
	ID                 string
	ModelVersion       string
	Generation         int
	Category           string
	Architecture       string
	Framework          string
	Status             string
	Hyperparameters    map[string]string
	FeatureSet         []string
	TrainingDataset    string
	FitnessScores      []FitnessSnapshot
	Ensemble           *EnsembleDef
	CreatedAt          time.Time
	GraduatedAt        *time.Time
	RetiredAt          *time.Time
	Deployments         []DeploymentRecord
	PerformanceHistory  []PerformancePoint
}

/*
Function: IsGraduated
Description:
  Returns true if the model has graduated status.

Input:
  - none

Output:
  - bool

Lines: ~3
*/
func (mr *ModelRecord) IsGraduated() bool { return mr.Status == ModelStatusGraduated }

/*
Function: IsRetired
Description:
  Returns true if the model is retired.

Input:
  - none

Output:
  - bool

Lines: ~3
*/
func (mr *ModelRecord) IsRetired() bool { return mr.Status == ModelStatusRetired }

/*
Function: LatestFitness
Description:
  Returns the most recent FitnessSnapshot, or nil if none.

Input:
  - none

Output:
  - *FitnessSnapshot: Latest fitness, or nil

Lines: ~8
*/
func (mr *ModelRecord) LatestFitness() *FitnessSnapshot {
	if len(mr.FitnessScores) == 0 {
		return nil
	}
	latest := mr.FitnessScores[len(mr.FitnessScores)-1]
	return &latest
}

// ==============================
// FITNESS SNAPSHOT
// ==============================

/*
Struct: FitnessSnapshot
Description:
  A single fitness evaluation snapshot at a point in time.

Fields:
  - Timestamp  time.Time : When evaluated
  - Sharpe     float64   : Sharpe ratio
  - Sortino    float64   : Sortino ratio
  - Profit     float64   : Cumulative profit
  - Drawdown   float64   : Maximum drawdown
  - Accuracy   float64   : Prediction accuracy (0-100)
  - Consistency float64  : Performance consistency
  - Efficiency float64   : Capital efficiency
  - Generation int       : Which generation this snapshot belongs to

Lines: ~10
*/
type FitnessSnapshot struct {
	Timestamp   time.Time
	Sharpe      float64
	Sortino     float64
	Profit      float64
	Drawdown    float64
	Accuracy    float64
	Consistency float64
	Efficiency  float64
	Generation  int
}

// ==============================
// ENSEMBLE DEFINITION
// ==============================

/*
Struct: EnsembleDef
Description:
  Defines how sub-models compose into an ensemble. Per myreq3.txt §47.

Fields:
  - Type            string             : voting/stacking/blending/dynamic
  - SubModels       []string           : IDs of component models
  - VotingWeights   map[string]float64 : Per-model voting weight (sum to 1.0)
  - StackingMeta    string             : Meta-model ID for stacking ensembles
  - RegimeMap       map[string]string  : Market regime → active model
  - Confidence      float64            : Ensemble confidence score (0-1)
  - ContributionPct map[string]float64 : Per-model contribution percentage (§47)
  - WeightHistory   []WeightEntry      : Historical weight updates (§47)
  - UpdatedAt       time.Time          : Last weight update timestamp

Lines: ~10
*/
type EnsembleDef struct {
	Type            string
	SubModels       []string
	VotingWeights   map[string]float64
	StackingMeta    string
	RegimeMap       map[string]string
	Confidence      float64
	ContributionPct map[string]float64
	WeightHistory   []WeightEntry
	UpdatedAt       time.Time
}

// WeightEntry records a voting weight snapshot at a point in time (§47).
type WeightEntry struct {
	Timestamp time.Time
	ModelID   string
	Weight    float64
	Reason    string // "performance", "retirement", "manual"
}

// ==============================
// DEPLOYMENT RECORD
// ==============================

/*
Struct: DeploymentRecord
Description:
  Records when a model was deployed to Trading and what agent used it.

Fields:
  - Timestamp time.Time : When deployed
  - AgentID   string    : Which portfolio agent received it
  - Capital   float64   : Capital allocated to the agent at deployment
  - Status    string    : active / retired / replaced

Lines: ~5
*/
type DeploymentRecord struct {
	Timestamp time.Time
	AgentID   string
	Capital   float64
	Status    string
}

// ==============================
// PERFORMANCE POINT
// ==============================

/*
Struct: PerformancePoint
Description:
  A single live-trading performance data point (time series).

Fields:
  - Timestamp time.Time : When recorded
  - Sharpe    float64   : Running Sharpe
  - PnL       float64   : Running profit/loss
  - Drawdown  float64   : Current drawdown
  - Trades    int       : Cumulative trade count

Lines: ~5
*/
type PerformancePoint struct {
	Timestamp time.Time
	Sharpe    float64
	PnL       float64
	Drawdown  float64
	Trades    int
}

// ==============================
// MODEL REGISTRY
// ==============================

/*
Struct: ModelRegistry
Description:
  Thread-safe centralized registry for all models across their lifecycle.
  Stores experimental, graduated, and retired models. Tracks deployments
  and live performance. Per myreq3.txt §33.

Fields:
  - mu      sync.RWMutex            : Protects concurrent access
  - models  map[string]*ModelRecord : All models by ID
  - history []ModelRecord           : Full history (capped)

Lines: ~6
*/
type ModelRegistry struct {
	mu      sync.RWMutex
	models  map[string]*ModelRecord
	history []ModelRecord
}

/*
Function: NewModelRegistry
Description:
  Creates a new empty ModelRegistry.

Input:
  - none

Output:
  - *ModelRegistry

Lines: ~8
*/
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*ModelRecord),
	}
}

/*
Function: Register
Description:
  Registers a new model or updates an existing one. Appends to history.

Input:
  - mr *ModelRecord: Model to register

Output:
  - none

Lines: ~12
*/
func (r *ModelRegistry) Register(mr *ModelRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Preserve deployment history on update
	if existing, ok := r.models[mr.ID]; ok {
		if len(mr.Deployments) == 0 {
			mr.Deployments = existing.Deployments
		}
		if len(mr.PerformanceHistory) == 0 {
			mr.PerformanceHistory = existing.PerformanceHistory
		}
		if mr.CreatedAt.IsZero() {
			mr.CreatedAt = existing.CreatedAt
		}
	}
	if mr.CreatedAt.IsZero() {
		mr.CreatedAt = time.Now()
	}
	r.models[mr.ID] = mr

	// Append to history (cap at 500)
	r.history = append(r.history, *mr)
	if len(r.history) > 500 {
		r.history = r.history[len(r.history)-500:]
	}
}

/*
Function: Get
Description:
  Returns a model record by ID. Returns nil if not found.

Input:
  - id string: Model ID

Output:
  - *ModelRecord: Copy of the record, or nil

Lines: ~12
*/
func (r *ModelRegistry) Get(id string) *ModelRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mr, ok := r.models[id]
	if !ok {
		return nil
	}
	copy := *mr
	return &copy
}

/*
Function: Graduate
Description:
  Promotes a model to graduated status. Records graduation timestamp.

Input:
  - id string: Model ID

Output:
  - error: Non-nil if model not found

Lines: ~12
*/
func (r *ModelRegistry) Graduate(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mr, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %q not found in registry", id)
	}
	now := time.Now()
	mr.Status = ModelStatusGraduated
	mr.GraduatedAt = &now
	return nil
}

/*
Function: Retire
Description:
  Marks a model as retired. Records retirement timestamp.

Input:
  - id string: Model ID

Output:
  - error: Non-nil if model not found

Lines: ~12
*/
func (r *ModelRegistry) Retire(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mr, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %q not found in registry", id)
	}
	now := time.Now()
	mr.Status = ModelStatusRetired
	mr.RetiredAt = &now
	return nil
}

/*
Function: ListByStatus
Description:
  Returns all models matching a given status.

Input:
  - status string: ModelStatusExperimental/Graduated/Retired/etc.

Output:
  - []*ModelRecord: Matching records

Lines: ~12
*/
func (r *ModelRegistry) ListByStatus(status string) []*ModelRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ModelRecord
	for _, mr := range r.models {
		if mr.Status == status {
			copy := *mr
			result = append(result, &copy)
		}
	}
	return result
}

/*
Function: ListByCategory
Description:
  Returns all models in a given category.

Input:
  - category string: Model category

Output:
  - []*ModelRecord: Matching records

Lines: ~12
*/
func (r *ModelRegistry) ListByCategory(category string) []*ModelRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ModelRecord
	for _, mr := range r.models {
		if mr.Category == category {
			copy := *mr
			result = append(result, &copy)
		}
	}
	return result
}

/*
Function: Count
Description:
  Returns total models in registry.

Input:
  - none

Output:
  - int: Total count

Lines: ~5
*/
func (r *ModelRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

/*
Function: CountByStatus
Description:
  Returns count of models with a given status.

Input:
  - status string

Output:
  - int

Lines: ~8
*/
func (r *ModelRegistry) CountByStatus(status string) int {
	return len(r.ListByStatus(status))
}

/*
Function: Remove
Description:
  Permanently removes a model from the registry.

Input:
  - id string

Output:
  - none

Lines: ~5
*/
func (r *ModelRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.models, id)
}

/*
Function: RecordDeployment
Description:
  Records that a model was deployed to a trading agent.

Input:
  - id      string: Model ID
  - agentID string: Portfolio agent ID
  - capital float64: Capital allocated

Output:
  - error: Non-nil if model not found

Lines: ~12
*/
func (r *ModelRegistry) RecordDeployment(id, agentID string, capital float64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mr, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %q not found", id)
	}
	mr.Deployments = append(mr.Deployments, DeploymentRecord{
		Timestamp: time.Now(),
		AgentID:   agentID,
		Capital:   capital,
		Status:    "active",
	})
	return nil
}

/*
Function: RecordPerformance
Description:
  Appends a live-performance data point to a model's history.

Input:
  - id  string          : Model ID
  - pp  PerformancePoint: New data point

Output:
  - error: Non-nil if model not found

Lines: ~12
*/
func (r *ModelRegistry) RecordPerformance(id string, pp PerformancePoint) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mr, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %q not found", id)
	}
	mr.PerformanceHistory = append(mr.PerformanceHistory, pp)
	if len(mr.PerformanceHistory) > 200 {
		mr.PerformanceHistory = mr.PerformanceHistory[len(mr.PerformanceHistory)-200:]
	}
	return nil
}

/*
Function: RecordFitness
Description:
  Appends a fitness snapshot to a model's history.

Input:
  - id string        : Model ID
  - fs FitnessSnapshot: New fitness data

Output:
  - error: Non-nil if model not found

Lines: ~12
*/
func (r *ModelRegistry) RecordFitness(id string, fs FitnessSnapshot) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	mr, ok := r.models[id]
	if !ok {
		return fmt.Errorf("model %q not found", id)
	}
	fs.Timestamp = time.Now()
	mr.FitnessScores = append(mr.FitnessScores, fs)
	if len(mr.FitnessScores) > 100 {
		mr.FitnessScores = mr.FitnessScores[len(mr.FitnessScores)-100:]
	}
	return nil
}

/*
Function: AllIDs
Description:
  Returns all model IDs in the registry.

Input:
  - none

Output:
  - []string: All model IDs

Lines: ~8
*/
func (r *ModelRegistry) AllIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.models))
	for id := range r.models {
		ids = append(ids, id)
	}
	return ids
}
