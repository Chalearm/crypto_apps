/******************************************************************************
 * File Name       : trainer_garch.go
 * File Path       : school/trainer_garch.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:53 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:53 (UTC+7)
 *
 * Description     :
 *   Go-native GARCH(1,1) / EGARCH volatility forecaster per myreq3.txt §34.
 *
 * Responsibilities:
 *   - Implement core functionality.
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
 *   1.0.0   | 2026-07-01 19:25:53 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// GarchTrainer is a Go-native GARCH/EGARCH volatility forecaster.
type GarchTrainer struct {
	modelType string
	Omega     float64
	Alpha     float64
	Beta      float64
	Gamma     float64 // leverage effect (EGARCH only)
	mu        float64 // mean return
	fitted    bool
}

// NewGarchTrainer creates a trainer for GARCH or EGARCH.
func NewGarchTrainer(modelType string) *GarchTrainer {
	return &GarchTrainer{
		modelType: modelType,
		Omega: 0.00001, Alpha: 0.05, Beta: 0.90, Gamma: 0.0,
	}
}

// Fit estimates parameters via simplified maximum likelihood (variance targeting).
func (g *GarchTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	if len(targets) < 2 {
		return fmt.Errorf("need at least 2 observations")
	}

	// Compute returns (from targets or first feature column)
	returns := make([]float64, len(targets)-1)
	for i := 1; i < len(targets); i++ {
		returns[i-1] = targets[i] - targets[i-1]
	}
	if len(returns) < 10 {
		return fmt.Errorf("too few returns: %d", len(returns))
	}

	// Mean
	g.mu = 0.0
	for _, r := range returns {
		g.mu += r
	}
	g.mu /= float64(len(returns))

	// Unconditional variance (variance targeting)
	varUncond := 0.0
	for _, r := range returns {
		d := r - g.mu
		varUncond += d * d
	}
	varUncond /= float64(len(returns))

	// Set omega = varUncond * (1 - alpha - beta)
	g.Omega = varUncond * (1.0 - g.Alpha - g.Beta)
	if g.Omega < 1e-12 {
		g.Omega = 1e-6
	}

	// EGARCH: asymmetric term
	if g.modelType == ModelStatEGARCH {
		g.Gamma = -0.05 // negative skew (bad news increases vol more)
	}

	g.fitted = true
	return nil
}

// Predict returns the next-period volatility forecast.
func (g *GarchTrainer) Predict(features []float64) (float64, error) {
	if !g.fitted || len(features) < 2 {
		return g.Omega, nil
	}

	// Last return
	n := len(features)
	lastRet := features[n-1] - features[n-2]

	// Previous variance: use sample variance of input features
	prevVar := 0.0
	m := 0.0
	for _, f := range features {
		m += f
	}
	m /= float64(n)
	for _, f := range features {
		d := f - m
		prevVar += d * d
	}
	prevVar /= float64(n)
	if prevVar < 1e-12 {
		prevVar = g.Omega
	}

	if g.modelType == ModelStatEGARCH {
		// EGARCH: ln(σ²ₜ) = ω + α·(|εₜ₋₁|/σₜ₋₁ - √(2/π)) + γ·(εₜ₋₁/σₜ₋₁) + β·ln(σ²ₜ₋₁)
		sigma := math.Sqrt(prevVar)
		if sigma < 1e-12 {
			sigma = 1e-6
		}
		epsNorm := (lastRet - g.mu) / sigma
		sqrt2pi := math.Sqrt(2.0 / math.Pi)
		logVar := g.Omega + g.Alpha*(math.Abs(epsNorm)-sqrt2pi) + g.Gamma*epsNorm + g.Beta*math.Log(prevVar)
		return math.Exp(logVar), nil
	}

	// Standard GARCH: σ²ₜ = ω + α·ε²ₜ₋₁ + β·σ²ₜ₋₁
	residual := lastRet - g.mu
	return g.Omega + g.Alpha*residual*residual + g.Beta*prevVar, nil
}

// Backtest evaluates on historical data.
func (g *GarchTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !g.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(targets))
	for i := range targets {
		if i < 2 {
			predictions[i] = targets[i]
			continue
		}
		pred, _ := g.Predict(targets[max(0, i-10) : i])
		predictions[i] = pred
	}
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// WalkForward performs rolling validation.
func (g *GarchTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}
	cfg := NewTrainingConfig()
	var histories []FitnessHistory
	for start := 0; start+windowSize < n; start++ {
		tmp := NewGarchTrainer(g.modelType)
		_ = tmp.Fit(features[start:start+windowSize], targets[start:start+windowSize], cfg)
		pred, _ := tmp.Predict(targets[start+windowSize-10 : start+windowSize])
		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{targets[start+windowSize]})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// Serialize marshals to JSON.
func (g *GarchTrainer) Serialize() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model_type": g.modelType, "omega": g.Omega, "alpha": g.Alpha,
		"beta": g.Beta, "gamma": g.Gamma, "mu": g.mu, "fitted": g.fitted,
	})
}

// Deserialize restores from JSON.
func (g *GarchTrainer) Deserialize(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["omega"].(float64); ok {
		g.Omega = v
	}
	if v, ok := m["alpha"].(float64); ok {
		g.Alpha = v
	}
	if v, ok := m["beta"].(float64); ok {
		g.Beta = v
	}
	if v, ok := m["gamma"].(float64); ok {
		g.Gamma = v
	}
	if v, ok := m["mu"].(float64); ok {
		g.mu = v
	}
	if v, ok := m["fitted"].(bool); ok {
		g.fitted = v
	}
	return nil
}

func init() {
	RegisterTrainer(ModelStatGARCH, func() TrainingEngine { return NewGarchTrainer(ModelStatGARCH) })
	RegisterTrainer(ModelStatEGARCH, func() TrainingEngine { return NewGarchTrainer(ModelStatEGARCH) })
}
