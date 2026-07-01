/******************************************************************************
 * File Name       : trainer_linear.go
 * File Path       : school/trainer_linear.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-29 16:10:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:10:00 (UTC+7)
 *
 * Description     :
 *   Go-native linear model trainers: Linear Regression, Ridge Regression,
 *   Lasso Regression, and Elastic Net. Uses closed-form normal equations
 *   with optional L2 (Ridge), L1 (Lasso via coordinate descent), and
 *   combined L1+L2 (Elastic Net) regularization.
 *
 * Responsibilities:
 *   - Implement TrainingEngine for Linear/Ridge/Lasso/ElasticNet
 *   - Closed-form OLS solution via matrix inversion
 *   - Coordinate descent for Lasso soft-thresholding
 *   - Serialize/Deserialize model coefficients
 *
 * Usage :
 *   Directory : school/
 *   Build     : go build ./school
 *   Test      : go test ./school -v -run LinearTrainer
 *
 * Dependencies :
 *   Internal : dexbot/school (TrainingEngine, TrainingConfig, FitnessHistory)
 *   External : encoding/json, fmt (stdlib)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Struct]    LinearTrainer
 *   [Function]  NewLinearTrainer, (LinearTrainer).Fit, .Predict,
 *               .Backtest, .WalkForward, .Serialize, .Deserialize
 *   [Function]  solveNormalEq, softThreshold
 *   [Function]  init() — registers trainers for all 4 model types
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-29 16:10:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add QR decomposition for numerical stability on ill-conditioned matrices
 *
 * Notes :
 *   - Registered for: Linear Regression, Ridge Regression, Lasso Regression,
 *     Elastic Net (per myreq3.txt §35).
 *   - In MODE C (worker2), the dispatcher delegates to Python scikit-learn.
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"time"
)

// ==============================
// LINEAR TRAINER
// ==============================

/*
Struct: LinearTrainer
Description:
  Go-native linear model. Supports OLS, Ridge (L2), Lasso (L1), and
  Elastic Net (L1+L2). Stores coefficients and intercept.

Fields:
  - modelType    string    : Which linear variant (Linear/Ridge/Lasso/ElasticNet)
  - coefficients []float64 : Feature weights (length = nFeatures)
  - intercept    float64   : Bias term
  - l1Ratio      float64   : L1 mixing ratio for Elastic Net (0=Ridge, 1=Lasso)
  - fitted       bool      : Whether Fit() has been called
*/
type LinearTrainer struct {
	modelType    string
	coefficients []float64
	intercept    float64
	l1Ratio      float64
	fitted       bool
}

/******************************************************************************
 * Function Name : NewLinearTrainer
 *
 * Purpose :
 *   Creates a new linear model trainer for the given variant.
 *
 * Inputs :
 *   modelType  string — One of "Linear Regression", "Ridge Regression",
 *                        "Lasso Regression", "Elastic Net"
 *
 * Return :
 *   Type        : *LinearTrainer
 *   Description : Initialized trainer (not yet fitted).
 *
 * Error Cases :
 *   - Unknown modelType : returns a LinearTrainer with modelType="Linear Regression"
 *
 * Complexity : Time O(1), Space O(1)
 * Number Of Lines : 17
 ******************************************************************************/
func NewLinearTrainer(modelType string) *LinearTrainer {
	lt := &LinearTrainer{modelType: modelType}
	switch modelType {
	case ModelSupRidge:
		lt.l1Ratio = 0.0 // pure L2
	case ModelSupLasso:
		lt.l1Ratio = 1.0 // pure L1
	case ModelSupElasticNet:
		lt.l1Ratio = 0.5 // 50/50 mix
	default:
		lt.modelType = ModelSupLinearReg
		lt.l1Ratio = 0.0
	}
	return lt
}

// ==============================
// FIT
// ==============================

/******************************************************************************
 * Function Name : Fit (LinearTrainer)
 *
 * Purpose :
 *   Trains the linear model on the given features and targets.
 *   Uses closed-form normal equations for OLS/Ridge.
 *   Uses coordinate descent for Lasso/ElasticNet.
 *
 * Inputs :
 *   features  [][]float64 — [nSamples][nFeatures] training data
 *   targets   []float64   — [nSamples] target values
 *   cfg       *TrainingConfig — Regularization, MaxEpochs used
 *
 * Return :
 *   Type        : error
 *   Description : nil on success, or descriptive error.
 *
 * Error Cases :
 *   - features == nil || len(features) == 0 : "no training data"
 *   - len(features) != len(targets)         : "dimension mismatch"
 *   - Singular matrix in OLS                : "matrix singular"
 *
 * Complexity : Time O(n*d²) for OLS, O(n*d*epochs) for Lasso
 * Number Of Lines : 80
 ******************************************************************************/
func (lt *LinearTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	if len(features) == 0 {
		return fmt.Errorf("no training data")
	}
	if len(features) != len(targets) {
		return fmt.Errorf("dimension mismatch: %d samples vs %d targets", len(features), len(targets))
	}
	nSamples := len(features)
	nFeatures := len(features[0])

	reg := cfg.Regularization
	if reg <= 0 {
		reg = 0.001
	}

	lt.coefficients = make([]float64, nFeatures)

	if lt.l1Ratio < 0.01 {
		// Pure L2 (Ridge / OLS): closed-form (XᵀX + λI)⁻¹ Xᵀy
		err := lt.solveRidge(features, targets, reg)
		if err != nil {
			return err
		}
	} else {
		// L1 or Elastic Net: coordinate descent
		epochs := cfg.MaxEpochs
		if epochs <= 0 {
			epochs = 100
		}
		lt.solveLassoCD(features, targets, reg, lt.l1Ratio, epochs)
	}

	// Compute intercept as residual mean
	predSum := 0.0
	for i := 0; i < nSamples; i++ {
		predSum += targets[i] - lt.predictRaw(features[i])
	}
	lt.intercept = predSum / float64(nSamples)
	lt.fitted = true
	return nil
}

// solveRidge solves (XᵀX + λI)⁻¹ Xᵀy via Gaussian elimination.
func (lt *LinearTrainer) solveRidge(features [][]float64, targets []float64, lambda float64) error {
	nSamples := len(features)
	nFeatures := len(features[0])

	// Build XᵀX + λI
	xtx := make([][]float64, nFeatures)
	for i := 0; i < nFeatures; i++ {
		xtx[i] = make([]float64, nFeatures+1) // augmented with Xᵀy
		for j := 0; j < nFeatures; j++ {
			sum := 0.0
			for k := 0; k < nSamples; k++ {
				sum += features[k][i] * features[k][j]
			}
			xtx[i][j] = sum
		}
		if i == 0 || lambda > 0 {
			xtx[i][i] += lambda
		}
	}

	// Xᵀy as last column
	for i := 0; i < nFeatures; i++ {
		sum := 0.0
		for k := 0; k < nSamples; k++ {
			sum += features[k][i] * targets[k]
		}
		xtx[i][nFeatures] = sum
	}

	// Gaussian elimination with partial pivoting
	for col := 0; col < nFeatures; col++ {
		// Pivot
		maxVal := abs(xtx[col][col])
		maxRow := col
		for row := col + 1; row < nFeatures; row++ {
			if abs(xtx[row][col]) > maxVal {
				maxVal = abs(xtx[row][col])
				maxRow = row
			}
		}
		if maxVal < 1e-12 {
			return fmt.Errorf("matrix singular at column %d", col)
		}
		if maxRow != col {
			xtx[col], xtx[maxRow] = xtx[maxRow], xtx[col]
		}

		// Eliminate below
		for row := col + 1; row < nFeatures; row++ {
			factor := xtx[row][col] / xtx[col][col]
			for j := col; j <= nFeatures; j++ {
				xtx[row][j] -= factor * xtx[col][j]
			}
		}
	}

	// Back substitution
	for i := nFeatures - 1; i >= 0; i-- {
		sum := xtx[i][nFeatures]
		for j := i + 1; j < nFeatures; j++ {
			sum -= xtx[i][j] * lt.coefficients[j]
		}
		lt.coefficients[i] = sum / xtx[i][i]
	}
	return nil
}

// solveLassoCD implements coordinate descent with soft-thresholding.
func (lt *LinearTrainer) solveLassoCD(features [][]float64, targets []float64, lambda, l1Ratio float64, epochs int) {
	nSamples := len(features)
	nFeatures := len(features[0])

	// Precompute feature norms
	norms := make([]float64, nFeatures)
	for j := 0; j < nFeatures; j++ {
		sum := 0.0
		for i := 0; i < nSamples; i++ {
			sum += features[i][j] * features[i][j]
		}
		norms[j] = sum
	}

	l1Lambda := lambda * l1Ratio
	l2Lambda := lambda * (1.0 - l1Ratio)

	for iter := 0; iter < epochs; iter++ {
		maxChange := 0.0
		for j := 0; j < nFeatures; j++ {
			// Compute residual excluding feature j
			residual := 0.0
			for i := 0; i < nSamples; i++ {
				pred := 0.0
				for k := 0; k < nFeatures; k++ {
					pred += lt.coefficients[k] * features[i][k]
				}
				residual += features[i][j] * (targets[i] - (pred - lt.coefficients[j]*features[i][j]))
			}

			oldCoef := lt.coefficients[j]
			// Soft threshold (L1) + ridge shrinkage (L2)
			denom := norms[j] + l2Lambda
			if denom < 1e-12 {
				denom = 1e-12
			}
			rho := residual / denom
			lt.coefficients[j] = softThreshold(rho, l1Lambda/denom)

			change := abs(lt.coefficients[j] - oldCoef)
			if change > maxChange {
				maxChange = change
			}
		}
		if maxChange < 1e-8 {
			break
		}
	}
}

// softThreshold is the L1 proximal operator.
func softThreshold(x, lambda float64) float64 {
	if x > lambda {
		return x - lambda
	} else if x < -lambda {
		return x + lambda
	}
	return 0
}

// ==============================
// PREDICT
// ==============================

/******************************************************************************
 * Function Name : Predict (LinearTrainer)
 *
 * Purpose :
 *   Produces a scalar prediction for one feature vector.
 *
 * Inputs :
 *   features  []float64 — Feature values (length = nFeatures)
 *
 * Return :
 *   Type        : float64
 *   Description : Predicted value (dot product + intercept).
 *
 * Error Cases :
 *   - Not fitted : returns 0 (no error)
 *   - len(features) != nFeatures : still produces dot product on available dims
 *
 * Complexity : Time O(d), Space O(1)
 * Number Of Lines : 10
 ******************************************************************************/
func (lt *LinearTrainer) Predict(features []float64) (float64, error) {
	if !lt.fitted {
		return 0, nil
	}
	return lt.predictRaw(features) + lt.intercept, nil
}

func (lt *LinearTrainer) predictRaw(features []float64) float64 {
	sum := 0.0
	n := len(features)
	if n > len(lt.coefficients) {
		n = len(lt.coefficients)
	}
	for i := 0; i < n; i++ {
		sum += lt.coefficients[i] * features[i]
	}
	return sum
}

// ==============================
// BACKTEST
// ==============================

/******************************************************************************
 * Function Name : Backtest (LinearTrainer)
 *
 * Purpose :
 *   Runs a full historical backtest and returns FitnessHistory.
 *
 * Inputs :
 *   features  [][]float64 — Historical feature matrix
 *   targets   []float64   — Historical target values
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Metrics computed via ComputeFitnessFromPredictions.
 *
 * Error Cases :
 *   - Not fitted : returns zeroed FitnessHistory
 *
 * Complexity : Time O(n*d), Space O(n)
 * Number Of Lines : 18
 ******************************************************************************/
func (lt *LinearTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !lt.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(features))
	for i, f := range features {
		predictions[i], _ = lt.Predict(f)
	}
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// ==============================
// WALK-FORWARD
// ==============================

/******************************************************************************
 * Function Name : WalkForward (LinearTrainer)
 *
 * Purpose :
 *   Rolling walk-forward validation. Trains on first 'windowSize' samples,
 *   predicts the next, slides forward, repeats.
 *
 * Inputs :
 *   features   [][]float64 — Full feature matrix
 *   targets    []float64   — Full target vector
 *   windowSize int         — Training window length
 *
 * Return :
 *   Type        : []FitnessHistory
 *   Description : One FitnessHistory per walk-forward fold.
 *
 * Error Cases :
 *   - windowSize <= 0 || windowSize >= len(features) : returns nil
 *
 * Complexity : Time O(folds * n * d²), Space O(n)
 * Number Of Lines : 30
 ******************************************************************************/
func (lt *LinearTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}

	cfg := NewTrainingConfig()
	var histories []FitnessHistory

	for start := 0; start+windowSize < n; start++ {
		trainF := features[start : start+windowSize]
		trainT := targets[start : start+windowSize]
		testF := features[start+windowSize]
		testT := targets[start+windowSize]

		// Train a fresh model on this window
		tmp := NewLinearTrainer(lt.modelType)
		if err := tmp.Fit(trainF, trainT, cfg); err != nil {
			continue
		}

		// Predict the next point (single-step ahead)
		pred, _ := tmp.Predict(testF)

		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{testT})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// ==============================
// SERIALIZE / DESERIALIZE
// ==============================

// linearData is the JSON-serializable form of LinearTrainer.
type linearData struct {
	ModelType    string    `json:"model_type"`
	Coefficients []float64 `json:"coefficients"`
	Intercept    float64   `json:"intercept"`
	L1Ratio      float64   `json:"l1_ratio"`
	Fitted       bool      `json:"fitted"`
}

/******************************************************************************
 * Function Name : Serialize (LinearTrainer)
 *
 * Purpose :
 *   Marshals model coefficients to JSON bytes for the Artifact Contract.
 *
 * Return :
 *   Type        : []byte
 *   Description : JSON-encoded model state.
 *
 * Complexity : Time O(d), Space O(d)
 * Number Of Lines : 15
 ******************************************************************************/
func (lt *LinearTrainer) Serialize() ([]byte, error) {
	data := linearData{
		ModelType:    lt.modelType,
		Coefficients: lt.coefficients,
		Intercept:    lt.intercept,
		L1Ratio:      lt.l1Ratio,
		Fitted:       lt.fitted,
	}
	return json.Marshal(data)
}

/******************************************************************************
 * Function Name : Deserialize (LinearTrainer)
 *
 * Purpose :
 *   Restores model state from JSON bytes.
 *
 * Inputs :
 *   data  []byte — JSON bytes from Serialize()
 *
 * Return :
 *   Type        : error
 *   Description : nil on success.
 *
 * Error Cases :
 *   - Invalid JSON : returns error
 *
 * Complexity : Time O(d), Space O(d)
 * Number Of Lines : 10
 ******************************************************************************/
func (lt *LinearTrainer) Deserialize(data []byte) error {
	var d linearData
	if err := json.Unmarshal(data, &d); err != nil {
		return err
	}
	lt.modelType = d.ModelType
	lt.coefficients = d.Coefficients
	lt.intercept = d.Intercept
	lt.l1Ratio = d.L1Ratio
	lt.fitted = d.Fitted
	return nil
}

// ==============================
// HELPERS
// ==============================

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// ==============================
// REGISTRATION
// ==============================

func init() {
	RegisterTrainer(ModelSupLinearReg, func() TrainingEngine { return NewLinearTrainer(ModelSupLinearReg) })
	RegisterTrainer(ModelSupRidge, func() TrainingEngine { return NewLinearTrainer(ModelSupRidge) })
	RegisterTrainer(ModelSupLasso, func() TrainingEngine { return NewLinearTrainer(ModelSupLasso) })
	RegisterTrainer(ModelSupElasticNet, func() TrainingEngine { return NewLinearTrainer(ModelSupElasticNet) })
}
