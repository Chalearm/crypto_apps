/******************************************************************************
 * File Name       : training.go
 * File Path       : school/training.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-29 16:00:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:00:00 (UTC+7)
 *
 * Description     :
 *   TrainingEngine interface — the contract every trainable model must
 *   implement. Each model family (statistical, supervised, DL, RL,
 *   unsupervised) provides a Go-native implementation of this interface.
 *   When remote training is configured, the dispatcher proxies calls
 *   via UDP using the Artifact Contract (§44-47).
 *
 * Responsibilities:
 *   - Define Predict / Fit / Backtest / WalkForward contracts
 *   - Define Serialize / Deserialize for model persistence
 *   - Provide a Go-native model registry for in-process dispatch
 *   - Common fitness-computation helper shared by all trainers
 *
 * Usage :
 *   Directory : school/
 *   Build     : go build ./school
 *   Test      : go test ./school -v -run Training
 *
 * Dependencies :
 *   Internal : dexbot/school (FitnessHistory)
 *   External : fmt, time (stdlib)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Interface] TrainingEngine
 *   [Struct]    TrainingConfig
 *   [Function]  NewTrainingConfig, RegisterTrainer, NewTrainer,
 *               HasGoTrainer, RegisteredTrainerTypes,
 *               ComputeFitnessFromPredictions
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-29 16:00:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add BatchPredict for vectorized inference
 *
 * Notes :
 *   - Per myreq3.txt §34-38, §42-47: each model family implements this.
 *   - The dispatcher (trainer_dispatcher.go) routes to Go-native or remote.
 ******************************************************************************/

package school

import (
	"time"
)

// ==============================
// TRAINING CONFIG
// ==============================

/*
Struct: TrainingConfig
Description:
  Per-training-run configuration passed to Fit() and WalkForward().
  Carries hyperparameter overrides, window sizes, and epoch counts.

Fields:
  - WindowSize      int     : Walk-forward rolling window length
  - MaxEpochs       int     : Maximum training epochs/iterations
  - LearningRate    float64 : Learning rate for gradient-based models
  - Regularization  float64 : L1/L2 regularization strength
  - BatchSize       int     : Mini-batch size (if applicable)
  - ValidationSplit float64 : Fraction of data for validation (0.0–1.0)
  - EarlyStopping   bool    : Whether to stop early on validation plateau
*/
type TrainingConfig struct {
	WindowSize      int
	MaxEpochs       int
	LearningRate    float64
	Regularization  float64
	BatchSize       int
	ValidationSplit float64
	EarlyStopping   bool
}

/******************************************************************************
 * Function Name : NewTrainingConfig
 *
 * Purpose :
 *   Returns sensible defaults for a training configuration.
 *
 * Inputs : None
 *
 * Return :
 *   Type        : *TrainingConfig
 *   Description : Default training configuration (window=30, epochs=100,
 *                 lr=0.01, reg=0.001, batch=32, valSplit=0.2, earlyStop=true).
 *
 * Error Cases : None
 *
 * Complexity : Time O(1), Space O(1)
 * Number Of Lines : 13
 ******************************************************************************/
func NewTrainingConfig() *TrainingConfig {
	return &TrainingConfig{
		WindowSize:      30,
		MaxEpochs:       100,
		LearningRate:    0.01,
		Regularization:  0.001,
		BatchSize:       32,
		ValidationSplit: 0.2,
		EarlyStopping:   true,
	}
}

// ==============================
// TRAINING ENGINE INTERFACE
// ==============================

/******************************************************************************
 * Interface Name : TrainingEngine
 *
 * Purpose :
 *   Core contract for any trainable model in the School daemon.
 *   Every model — from linear regression to LSTM — implements these six
 *   methods. The dispatcher routes calls to Go-native trainers or remote
 *   schools transparently.
 *
 * Methods :
 *   Fit(features, targets, cfg)       — trains model on data
 *   Predict(features)                 — scalar prediction for one sample
 *   Backtest(features, targets)       — full backtest → FitnessHistory
 *   WalkForward(features, targets, w) — rolling validation → []FitnessHistory
 *   Serialize()                       — marshal weights → []byte
 *   Deserialize(data)                 — restore weights from []byte
 *
 * Notes :
 *   - features: [nSamples][nFeatures]float64
 *   - targets:  [nSamples]float64
 *   - Implementations must be thread-safe where noted.
 ******************************************************************************/
type TrainingEngine interface {
	Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error
	Predict(features []float64) (float64, error)
	Backtest(features [][]float64, targets []float64) (*FitnessHistory, error)
	WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error)
	Serialize() ([]byte, error)
	Deserialize(data []byte) error
}

// ==============================
// TRAINER REGISTRY (GO-NATIVE)
// ==============================

// trainerFactories maps model type constants to constructor functions.
// Populated by init() in each trainer file.
var trainerFactories = map[string]func() TrainingEngine{}

/******************************************************************************
 * Function Name : RegisterTrainer
 *
 * Purpose :
 *   Registers a constructor for a Go-native TrainingEngine by model type.
 *
 * Inputs :
 *   modelType  string                 — Canonical model type constant
 *   factory    func() TrainingEngine — Constructor returning fresh instance
 *
 * Return : None
 *
 * Error Cases :
 *   - Re-registration: overwrites (last wins)
 *
 * Complexity : Time O(1), Space O(1)
 * Number Of Lines : 5
 ******************************************************************************/
func RegisterTrainer(modelType string, factory func() TrainingEngine) {
	trainerFactories[modelType] = factory
}

/******************************************************************************
 * Function Name : NewTrainer
 *
 * Purpose :
 *   Creates a Go-native trainer for the given model type.
 *
 * Inputs :
 *   modelType  string — Registered model type constant
 *
 * Return :
 *   Type        : TrainingEngine
 *   Description : Fresh instance, or nil if modelType not registered.
 *
 * Error Cases :
 *   - Unknown modelType : returns nil
 *
 * Complexity : Time O(1), Space O(1)
 * Number Of Lines : 8
 ******************************************************************************/
func NewTrainer(modelType string) TrainingEngine {
	factory, ok := trainerFactories[modelType]
	if !ok {
		return nil
	}
	return factory()
}

/******************************************************************************
 * Function Name : HasGoTrainer
 *
 * Purpose :
 *   Checks whether a Go-native trainer is registered for the given model type.
 *
 * Inputs :
 *   modelType  string — Model type name to check
 *
 * Return :
 *   Type        : bool
 *   Description : true if a Go-native trainer exists.
 *
 * Complexity : Time O(1), Space O(1)
 * Number Of Lines : 5
 ******************************************************************************/
func HasGoTrainer(modelType string) bool {
	_, ok := trainerFactories[modelType]
	return ok
}

/******************************************************************************
 * Function Name : RegisteredTrainerTypes
 *
 * Purpose :
 *   Returns all model types that have a Go-native trainer registered.
 *
 * Return :
 *   Type        : []string
 *   Description : Model type names with registered trainers.
 *
 * Complexity : Time O(n), Space O(n)
 * Number Of Lines : 10
 ******************************************************************************/
func RegisteredTrainerTypes() []string {
	types := make([]string, 0, len(trainerFactories))
	for t := range trainerFactories {
		types = append(types, t)
	}
	return types
}

// ==============================
// COMMON EVALUATION HELPERS
// ==============================

// sqrt252 is precomputed sqrt(252) for annualized Sharpe/Sortino.
const sqrt252 = 15.874507866387544

/******************************************************************************
 * Function Name : ComputeFitnessFromPredictions
 *
 * Purpose :
 *   Computes a FitnessHistory from predictions vs actual targets.
 *   Used by every Backtest() implementation for consistent metrics.
 *
 * Inputs :
 *   predictions  []float64 — Model predictions
 *   actuals      []float64 — Ground-truth values (same length)
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Populated with Sharpe, Sortino, Profit, Drawdown,
 *                 Accuracy, Consistency, etc.
 *
 * Error Cases :
 *   - len(predictions) != len(actuals) : returns zeroed FitnessHistory
 *   - len(predictions) == 0           : returns zeroed FitnessHistory
 *
 * Dependencies : time (stdlib), math helpers below
 *
 * Complexity : Time O(n), Space O(n) for returns slice
 * Number Of Lines : 90
 *
 * Notes :
 *   - Sharpe = mean(ret) / std(ret) * sqrt(252)
 *   - Sortino = mean(ret) / downside_std(ret) * sqrt(252)
 *   - Drawdown = max peak-to-trough cumulative loss
 *   - Accuracy = 1.0 - MAE/range (0–100)
 ******************************************************************************/
func ComputeFitnessFromPredictions(predictions, actuals []float64) *FitnessHistory {
	n := len(predictions)
	now := time.Now()
	if n == 0 || n != len(actuals) {
		return &FitnessHistory{Timestamp: now}
	}

	// 1. Profit and MAE
	profit := 0.0
	directionCorrect := 0.0
	mae := 0.0
	for i := 0; i < n; i++ {
		err := predictions[i] - actuals[i]
		if err < 0 {
			err = -err
		}
		mae += err
		profit -= err
	}
	mae /= float64(n)

	// Directional accuracy (i > 0)
	if n > 1 {
		for i := 1; i < n; i++ {
			predDir := predictions[i] - predictions[i-1]
			actDir := actuals[i] - actuals[i-1]
			if (predDir >= 0 && actDir >= 0) || (predDir < 0 && actDir < 0) {
				directionCorrect++
			}
		}
		directionCorrect /= float64(n - 1)
	}

	// 2. Simulated returns: directional bet * actual movement
	returns := make([]float64, n-1)
	for i := 1; i < n; i++ {
		dir := 1.0
		if predictions[i] < predictions[i-1] {
			dir = -1.0
		}
		returns[i-1] = dir * (actuals[i] - actuals[i-1])
	}

	// 3. Mean and std of returns
	meanRet := 0.0
	for _, r := range returns {
		meanRet += r
	}
	if len(returns) > 0 {
		meanRet /= float64(len(returns))
	}

	varRet := 0.0
	for _, r := range returns {
		d := r - meanRet
		varRet += d * d
	}
	if len(returns) > 0 {
		varRet /= float64(len(returns))
	}
	stdDev := sqrt(varRet)

	// 4. Sharpe (annualized)
	sharpe := 0.0
	if stdDev > 1e-12 {
		sharpe = meanRet / stdDev * sqrt252
	}

	// 5. Sortino: downside deviation only
	downVar := 0.0
	downCount := 0
	for _, r := range returns {
		if r < meanRet {
			d := r - meanRet
			downVar += d * d
			downCount++
		}
	}
	sortino := 0.0
	if downCount > 0 {
		downStd := sqrt(downVar / float64(downCount))
		if downStd > 1e-12 {
			sortino = meanRet / downStd * sqrt252
		}
	}

	// 6. Max drawdown
	cumulative := 0.0
	peak := 0.0
	drawdown := 0.0
	for _, r := range returns {
		cumulative += r
		if cumulative > peak {
			peak = cumulative
		}
		dd := peak - cumulative
		if dd > drawdown {
			drawdown = dd
		}
	}

	// 7. Prediction accuracy (0-100)
	accuracy := 0.0
	minA, maxA := actuals[0], actuals[0]
	for _, a := range actuals {
		if a < minA {
			minA = a
		}
		if a > maxA {
			maxA = a
		}
	}
	rng := maxA - minA
	if rng > 1e-12 {
		accuracy = 1.0 - mae/rng
		if accuracy < 0 {
			accuracy = 0
		}
		if accuracy > 1.0 {
			accuracy = 1.0
		}
	}

	return &FitnessHistory{
		Timestamp:          now,
		SharpeRatio:        sharpe,
		SortinoRatio:       sortino,
		Profit:             profit,
		Drawdown:           drawdown,
		PredictionAccuracy: accuracy * 100,
		CapitalEfficiency:  0.5,
		VolatilityControl:  1.0 - stdDev,
		ExecutionQuality:   directionCorrect * 100,
		Consistency:        directionCorrect * 100,
	}
}

// sqrt computes square root via Newton's method (stdlib-avoiding).
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z = z - (z*z-x)/(2*z)
	}
	return z
}
