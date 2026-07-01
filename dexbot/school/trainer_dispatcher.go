/******************************************************************************
 * File Name       : trainer_dispatcher.go
 * File Path       : school/trainer_dispatcher.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:50:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:50:00 (UTC+7)
 *
 * Description     :
 *   Training Mode Router — routes model training to the correct target
 *   based on configuration per myreq3.txt §42-43 and Phase 23 architecture.
 *
 *   MODE A: Go-native training (in-process, no remote configured).
 *   MODE B: Local daemon training (same machine, UDP loopback).
 *   MODE C: Dedicated remote training (worker2 via UDP/SSH).
 *   MODE D: Subprocess training (Python/TensorFlow/Rust/C++ binaries).
 *
 *   The dispatcher also tracks training progress and records results
 *   to the centralized Model Registry.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:50 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"fmt"
)

// ==============================
// DISPATCHER
// ==============================

// Dispatcher routes training requests to Go-native or remote schools.
type Dispatcher struct {
	remote     *RemoteClient
	spawner    *ProcessSpawner // §91: MODE_D subprocess trainer
	localOnly  bool
	goTrainers map[string]bool // model types with registered Go trainers
}

// NewDispatcher creates a training dispatcher.
// If remote is nil, all training runs locally via Go-native trainers.
func NewDispatcher(remote *RemoteClient) *Dispatcher {
	d := &Dispatcher{
		remote:     remote,
		spawner:    NewProcessSpawner("", "", 120),
		localOnly:  remote == nil || !remote.IsEnabled(),
		goTrainers: make(map[string]bool),
	}
	for _, t := range RegisteredTrainerTypes() {
		d.goTrainers[t] = true
	}
	return d
}

// SetProcessSpawner allows injecting a custom subprocess spawner (§91).
func (d *Dispatcher) SetProcessSpawner(ps *ProcessSpawner) {
	d.spawner = ps
}

/******************************************************************************
 * Function Name : TrainModel
 *
 * Purpose :
 *   Trains a single model on the given data. Routes to Go-native trainer
 *   if available; otherwise falls back to remote (if configured).
 *
 * Inputs :
 *   model    *ModelMetadata   — Model to train (Architecture used for routing)
 *   features [][]float64      — Training features
 *   targets  []float64        — Training targets
 *   cfg      *TrainingConfig  — Training hyperparameters
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Fitness computed after training (via Backtest on same data).
 *
 * Error Cases :
 *   - No Go-native trainer and no remote: returns nil
 *   - Remote training fails: returns nil
 *
 * Complexity : Depends on trainer; remote mode adds network latency.
 * Number Of Lines : 40
 ******************************************************************************/
func (d *Dispatcher) TrainModel(model *ModelMetadata, features [][]float64, targets []float64, cfg *TrainingConfig) *FitnessHistory {
	if model == nil {
		return nil
	}

	modelType := model.Architecture

	// MODE A: Go-native
	if d.goTrainers[modelType] {
		trainer := NewTrainer(modelType)
		if trainer != nil {
			if err := trainer.Fit(features, targets, cfg); err != nil {
				return nil
			}
			fh, err := trainer.Backtest(features, targets)
			if err != nil {
				return nil
			}
			// Update model fitness
			model.Fitness = fh
			model.FitnessTimeline = append(model.FitnessTimeline, *fh)
			return fh
		}
	}

	// MODE B/C: Remote training
	if d.remote != nil && d.remote.IsEnabled() {
		results, _ := d.remote.DistributeTraining([]*ModelMetadata{model})
		if len(results) > 0 && results[0].Err == "" {
			r := results[0]
			fh := &FitnessHistory{
				SharpeRatio:       r.Sharpe,
				SortinoRatio:      r.Sortino,
				Profit:            r.Profit,
				Drawdown:          r.Drawdown,
				PredictionAccuracy: r.Accuracy,
				Consistency:       r.Consistency,
				CapitalEfficiency: r.Efficiency,
			}
			model.Fitness = fh
			model.FitnessTimeline = append(model.FitnessTimeline, *fh)
			return fh
		}
	}

	return nil
}

/******************************************************************************
 * Function Name : TrainPopulation
 *
 * Purpose :
 *   Trains all trainable (non-graduate, non-retired) models in the population.
 *   Routes each model to the appropriate training mode.
 *
 * Inputs :
 *   pop      *ModelPopulation — Model population
 *   features [][]float64      — Training features (same for all models)
 *   targets  []float64        — Training targets
 *   cfg      *TrainingConfig  — Training hyperparameters
 *
 * Return :
 *   Type        : int
 *   Description : Number of models successfully trained.
 *
 * Complexity : O(m * T) where m = models, T = per-model training time
 * Number Of Lines : 30
 ******************************************************************************/
func (d *Dispatcher) TrainPopulation(pop *ModelPopulation, features [][]float64, targets []float64, cfg *TrainingConfig) int {
	trained := 0
	categories := []string{
		CategoryOptions, CategoryRisk, CategoryIntraday, CategorySwing,
		CategoryLongTerm, CategoryVolatility, CategoryLiquidity, CategoryPortfolio,
	}

	for _, cat := range categories {
		for _, model := range pop.ListByCategory(cat) {
			if model.Status == StatusRetired || model.Status == StatusGraduate {
				continue
			}
			fh := d.TrainModel(model, features, targets, cfg)
			if fh != nil {
				trained++
			}
		}
	}
	return trained
}

/******************************************************************************
 * Function Name : BacktestModel
 *
 * Purpose :
 *   Runs a full backtest on a single model using its registered trainer.
 *
 * Inputs :
 *   model    *ModelMetadata — Model to backtest
 *   features [][]float64    — Historical features
 *   targets  []float64      — Historical targets
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Computed fitness, or nil if model type not supported.
 *
 * Complexity : Depends on trainer
 * Number Of Lines : 20
 ******************************************************************************/
func (d *Dispatcher) BacktestModel(model *ModelMetadata, features [][]float64, targets []float64) *FitnessHistory {
	if model == nil {
		return nil
	}
	modelType := model.Architecture
	trainer := NewTrainer(modelType)
	if trainer == nil {
		return nil
	}
	if err := trainer.Fit(features, targets, NewTrainingConfig()); err != nil {
		return nil
	}
	fh, err := trainer.Backtest(features, targets)
	if err != nil {
		return nil
	}
	return fh
}

/******************************************************************************
 * Function Name : WalkForwardModel
 *
 * Purpose :
 *   Runs walk-forward validation on a single model.
 *
 * Inputs :
 *   model      *ModelMetadata — Model to validate
 *   features   [][]float64    — Full feature matrix
 *   targets    []float64      — Full target vector
 *   windowSize int            — Training window length
 *
 * Return :
 *   Type        : []FitnessHistory
 *   Description : Per-fold fitness histories, or nil on failure.
 *
 * Complexity : O(folds * training_time)
 * Number Of Lines : 18
 ******************************************************************************/
func (d *Dispatcher) WalkForwardModel(model *ModelMetadata, features [][]float64, targets []float64, windowSize int) []FitnessHistory {
	if model == nil {
		return nil
	}
	modelType := model.Architecture
	trainer := NewTrainer(modelType)
	if trainer == nil {
		return nil
	}
	histories, err := trainer.WalkForward(features, targets, windowSize)
	if err != nil {
		return nil
	}
	return histories
}

/******************************************************************************
 * Function Name : GetTrainingMode
 *
 * Purpose :
 *   Returns a human-readable description of the current training mode.
 *
 * Return :
 *   Type        : string
 *   Description : "MODE_A" (Go-native), "MODE_B" (local daemon), or
 *                 "MODE_C" (remote worker2).
 *
 * Complexity : O(1)
 * Number Of Lines : 12
 ******************************************************************************/
func (d *Dispatcher) GetTrainingMode() string {
	if d.spawner != nil {
		return "MODE_D (subprocess: Python/Rust/C++)"
	}
	if d.localOnly {
		return "MODE_A"
	}
	if d.remote != nil && d.remote.NodeCount() > 0 {
		return fmt.Sprintf("MODE_C (workers=%d)", d.remote.NodeCount())
	}
	return "MODE_B"
}

/******************************************************************************
 * Function Name : TrainWithSubprocess
 *
 * Purpose :
 *   MODE_D: Delegates training to an external subprocess (§91).
 *   Uses ProcessSpawner to run Python/TensorFlow/Rust/C++.
 *
 * Inputs :
 *   model    *ModelMetadata        — Model to train
 *   cfg      *ProcessTrainerConfig — Subprocess config (DB creds etc.)
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Computed fitness from subprocess result.
 *
 * Complexity : O(subprocess_runtime), Number Of Lines : 20
 ******************************************************************************/
func (d *Dispatcher) TrainWithSubprocess(model *ModelMetadata, procCfg *ProcessTrainerConfig) *FitnessHistory {
	if d.spawner == nil {
		return nil
	}
	if procCfg.ModelID == "" {
		procCfg.ModelID = model.Name
	}
	if procCfg.Architecture == "" {
		procCfg.Architecture = model.Architecture
	}
	if procCfg.Framework == "" {
		procCfg.Framework = "sklearn"
	}

	result := d.spawner.TrainWithPython(procCfg)
	fh := ConvertToFitness(result)
	if fh != nil && model != nil {
		model.Fitness = fh
		model.FitnessTimeline = append(model.FitnessTimeline, *fh)
	}
	return fh
}

/******************************************************************************
 * Function Name : AvailableTrainers
 *
 * Purpose :
 *   Returns the list of model types with registered Go-native trainers.
 ******************************************************************************/
func (d *Dispatcher) AvailableTrainers() []string {
	return RegisteredTrainerTypes()
}
