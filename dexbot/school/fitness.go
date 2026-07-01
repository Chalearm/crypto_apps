/******************************************************************************
 * File Name       : fitness.go
 * File Path       : school/fitness.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   Composite fitness scoring for model evaluation. Implements weighted multi-metric scoring per myreq2.txt §16.
 *
 * Responsibilities:
 *   - Implement core functionality for school package.
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
 *   [Types] Struct definitions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package school

// ==============================
// FITNESS WEIGHTS
// ==============================

/*
Struct: FitnessWeights
Description:
  Configurable weights for composite fitness scoring.
  Per myreq2.txt §16: Sharpe 25%, Sortino 15%, Profit 20%, Drawdown 15%,
  Accuracy 10%, Consistency 10%, Efficiency 5%.

Fields:
  - ShargeWeight      float64 : Sharpe ratio weight
  - SortinoWeight     float64 : Sortino ratio weight
  - ProfitWeight      float64 : Profit weight
  - DrawdownWeight    float64 : Drawdown weight (negative contribution)
  - AccuracyWeight    float64 : Prediction accuracy weight
  - ConsistencyWeight float64 : Consistency weight
  - EfficiencyWeight  float64 : Capital efficiency weight

Lines: ~10
*/
type FitnessWeights struct {
	SharpeWeight      float64
	SortinoWeight     float64
	ProfitWeight      float64
	DrawdownWeight    float64
	AccuracyWeight    float64
	ConsistencyWeight float64
	EfficiencyWeight  float64
}

/*
Function: DefaultFitnessWeights
Description:
  Returns the default fitness weight configuration.

Input:
  - none

Output:
  - FitnessWeights: Default weights summing to 1.0

Lines: ~12
*/
func DefaultFitnessWeights() FitnessWeights {
	return FitnessWeights{
		SharpeWeight:      0.25,
		SortinoWeight:     0.15,
		ProfitWeight:      0.20,
		DrawdownWeight:    -0.15, // negative: higher drawdown reduces score
		AccuracyWeight:    0.10,
		ConsistencyWeight: 0.10,
		EfficiencyWeight:  0.05,
	}
}

/*
Function: ValidateWeights
Description:
  Checks that weights sum to approximately 1.0 (absolute values).

Input:
  - w FitnessWeights: Weights to validate

Output:
  - bool: true if valid

Lines: ~10
*/
func ValidateWeights(w FitnessWeights) bool {
	sum := w.SharpeWeight + w.SortinoWeight + w.ProfitWeight +
		w.AccuracyWeight + w.ConsistencyWeight + w.EfficiencyWeight +
		(-w.DrawdownWeight) // DrawdownWeight is negative
	return sum >= 0.99 && sum <= 1.01
}

// ==============================
// COMPOSITE FITNESS
// ==============================

/*
Function: CompositeFitness
Description:
  Computes a weighted composite fitness score from a FitnessHistory record.
  Higher is better. Drawdown is subtracted (negative weight).
  Each metric is normalized to a 0-1 scale before weighting.

Input:
  - fh      *FitnessHistory : Fitness data to score
  - weights FitnessWeights   : Weight configuration

Output:
  - float64 : Composite score (higher = better)
  - bool    : true if weights are valid

Lines: ~20
*/
func CompositeFitness(fh *FitnessHistory, weights FitnessWeights) (float64, bool) {
	if fh == nil {
		return 0, false
	}
	if !ValidateWeights(weights) {
		return 0, false
	}

	// Normalize each metric to 0-1 range (simple clamping)
	normSharpe := clamp(fh.SharpeRatio/3.0, 0, 1)       // 3+ Sharpe is excellent
	normSortino := clamp(fh.SortinoRatio/3.0, 0, 1)
	normProfit := clamp((fh.Profit+1.0)/2.0, 0, 1)      // -1 to +1 → 0 to 1
	normDdown := clamp(1.0-fh.Drawdown, 0, 1)           // 0 drawdown = 1, 100% = 0
	normAccuracy := clamp(fh.PredictionAccuracy/100.0, 0, 1)
	normConsistency := clamp(fh.Consistency, 0, 1)
	normEfficiency := clamp(fh.CapitalEfficiency, 0, 1)

	score :=
		weights.SharpeWeight*normSharpe +
			weights.SortinoWeight*normSortino +
			weights.ProfitWeight*normProfit +
			weights.DrawdownWeight*normDdown +
			weights.AccuracyWeight*normAccuracy +
			weights.ConsistencyWeight*normConsistency +
			weights.EfficiencyWeight*normEfficiency

	return score, true
}

/*
Function: clamp
Description:
  Clamps a float64 value to [low, high] inclusive.

Input:
  - v    float64 : Value to clamp
  - low  float64 : Minimum
  - high float64 : Maximum

Output:
  - float64 : Clamped value

Lines: ~8
*/
func clamp(v, low, high float64) float64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
