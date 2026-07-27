/******************************************************************************
 * File Name       : model_registry.go
 * File Path       : governance/model_registry.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:45 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:45 (UTC+7)
 * Description     : Centralized Model Registry for all model lifecycle tracking.
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
	Deployments        []DeploymentRecord
	PerformanceHistory []PerformancePoint
}

/******************************************************************************
 * Function Name : IsGraduated
 * Purpose       : Checks if the model is in graduated status.
 ******************************************************************************/
func (mr *ModelRecord) IsGraduated() bool { return mr.Status == ModelStatusGraduated }

/******************************************************************************
 * Function Name : IsRetired
 * Purpose       : Checks if the model is in retired status.
 ******************************************************************************/
func (mr *ModelRecord) IsRetired() bool { return mr.Status == ModelStatusRetired }

/******************************************************************************
 * Function Name : LatestFitness
 * Purpose       : Retrieves the latest recorded fitness snapshot.
 ******************************************************************************/
func (mr *ModelRecord) LatestFitness() *FitnessSnapshot {
	if len(mr.FitnessScores) == 0 {
		return nil
	}
	latest := mr.FitnessScores[len(mr.FitnessScores)-1]
	return &latest
}

// ==============================
// FITNESS SNAPSHOT & ENSEMBLE
// ==============================

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

type WeightEntry struct {
	Timestamp time.Time
	ModelID   string
	Weight    float64
	Reason    string
}

type DeploymentRecord struct {
	Timestamp time.Time
	AgentID   string
	Capital   float64
	Status    string
}

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

type ModelRegistry struct {
	mu      sync.RWMutex
	models  map[string]*ModelRecord
	history []ModelRecord
}

/******************************************************************************
 * Function Name : NewModelRegistry
 * Purpose       : Constructs a new ModelRegistry instance.
 ******************************************************************************/
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]*ModelRecord),
	}
}

/******************************************************************************
 * Function Name : Register
 * Purpose       : Registers or updates a model record.
 ******************************************************************************/
func (r *ModelRegistry) Register(mr *ModelRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()

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

	r.history = append(r.history, *mr)
	if len(r.history) > 500 {
		r.history = r.history[len(r.history)-500:]
	}
}

/******************************************************************************
 * Function Name : Get
 * Purpose       : Retrieves a copy of a model record by ID.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : Graduate
 * Purpose       : Promotes a model to graduated status.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : Retire
 * Purpose       : Sets a model status to retired.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : ListByStatus
 * Purpose       : Returns all model records matching a status.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : ListByCategory
 * Purpose       : Returns all model records matching a category.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : Count
 * Purpose       : Returns total models registered.
 ******************************************************************************/
func (r *ModelRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

/******************************************************************************
 * Function Name : CountByStatus
 * Purpose       : Returns count of models matching status.
 ******************************************************************************/
func (r *ModelRegistry) CountByStatus(status string) int {
	return len(r.ListByStatus(status))
}

/******************************************************************************
 * Function Name : Remove
 * Purpose       : Removes a model record by ID.
 ******************************************************************************/
func (r *ModelRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.models, id)
}

/******************************************************************************
 * Function Name : RecordDeployment
 * Purpose       : Appends a deployment record to a model.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : RecordPerformance
 * Purpose       : Records a live-trading performance point.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : RecordFitness
 * Purpose       : Records a fitness evaluation snapshot.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : AllIDs
 * Purpose       : Returns a list of all model IDs in registry.
 ******************************************************************************/
func (r *ModelRegistry) AllIDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.models))
	for id := range r.models {
		ids = append(ids, id)
	}
	return ids
}