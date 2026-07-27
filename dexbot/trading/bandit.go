/******************************************************************************
 * File Name       : bandit.go
 * File Path       : trading/bandit.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:25 (UTC+7)
  *
  * Function Name : SelectAgent
 * Modified Date   : 2026-07-01 19:25:25 (UTC+7)
 *
 * Description     :
 *   Thompson Sampling and Hierarchical Multi-Armed Bandit (H-MAB) for model selection and capital allocation. Per myreq2.txt §20-21. Level 1: Thompson Sampling selects which graduate model manages a portf
 *
 * Responsibilities:
 *   - - Implement core functionality for trading package.
 *
 * Usage :
 *   Directory : trading/
 *
 *   Build :
 *     go build ./trading
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./trading
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/trading
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
 *   1.0.0   | 2026-07-01 19:25:25 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package trading

import (
	"math"
	"math/rand"
	"sync"
)

// ==============================
// THOMPSON SAMPLING
// ==============================

/*
Struct: ThompsonSampling
Description:
  Implements Thompson Sampling using Beta distribution for Bernoulli rewards.
  Each arm tracks success (alpha) and failure (beta) counts.
  SelectArm() samples from Beta(alpha, beta) for each arm and picks the max.

Fields:
  - mu     sync.RWMutex : Protects alpha/beta slices
  - alpha  []float64    : Success counts per arm
  - beta   []float64    : Failure counts per arm
  - rng    *rand.Rand   : Random number generator

Lines: ~6
*/
type ThompsonSampling struct {
	mu    sync.RWMutex
	alpha []float64
	beta  []float64
	rng   *rand.Rand
}

/******************************************************************************
 * Function Name : NewThompsonSampling
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
  * Function Name : NewThompsonSampling
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
func NewThompsonSampling(nArms int, rng *rand.Rand) *ThompsonSampling {
	alpha := make([]float64, nArms)
	beta := make([]float64, nArms)
	for i := 0; i < nArms; i++ {
		alpha[i] = 1.0
		beta[i] = 1.0
	}
	return &ThompsonSampling{
		alpha: alpha,
		beta:  beta,
		rng:   rng,
	}
}

/******************************************************************************
 * Function Name : SelectArm
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

func (ts *ThompsonSampling) SelectArm() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if len(ts.alpha) == 0 {
		return -1
	}

	bestArm := 0
	bestValue := -1.0
	for i := range ts.alpha {
		sample := sampleBeta(ts.alpha[i], ts.beta[i], ts.rng)
		if sample > bestValue {
			bestValue = sample
			bestArm = i
		}
	}
	return bestArm
}

/******************************************************************************
 * Function Name : UpdateArm
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
  * Function Name : UpdateArm
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
func (ts *ThompsonSampling) UpdateArm(arm int, reward float64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if arm < 0 || arm >= len(ts.alpha) {
		return
	}

	if reward > 0.5 {
		ts.alpha[arm]++
	} else {
		ts.beta[arm]++
	}
}

/******************************************************************************
 * Function Name : GetProbabilities
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

func (ts *ThompsonSampling) GetProbabilities() []float64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	probs := make([]float64, len(ts.alpha))
	for i := range ts.alpha {
		total := ts.alpha[i] + ts.beta[i]
		if total > 0 {
			probs[i] = ts.alpha[i] / total
		} else {
			probs[i] = 0.5
		}
	}
	return probs
}

/******************************************************************************
 * Function Name : ArmCount
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
  * Function Name : ArmCount
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
func (ts *ThompsonSampling) ArmCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.alpha)
}

// ==============================
// BETA SAMPLING (utility)
// ==============================

/******************************************************************************
 * Function Name : sampleBeta
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

func sampleBeta(alpha, beta float64, rng *rand.Rand) float64 {
	if alpha <= 0 || beta <= 0 {
		return 0.5
	}
	// Marsaglia-Tsang gamma approximation via exponential
	x := sampleGamma(alpha, rng)
	y := sampleGamma(beta, rng)
	if x+y == 0 {
		return 0.5
	}
	return x / (x + y)
}

// sampleGamma uses Marsaglia-Tsang method for Gamma(shape, 1).
/******************************************************************************
 * Function Name : sampleGamma
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

func sampleGamma(shape float64, rng *rand.Rand) float64 {
	if shape < 1 {
		// Use Gamma(shape+1)/U^(1/shape) for shape < 1
		g := sampleGamma(shape+1, rng)
		return g * math.Pow(rng.Float64(), 1.0/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9.0*d)
	for {
		x := rng.NormFloat64()
		v := 1.0 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1.0-0.0331*(x*x)*(x*x) {
			return d * v
		}
		if math.Log(u) < 0.5*x*x+d*(1.0-v+math.Log(v)) {
			return d * v
		}
	}
}

// ==============================
// HIERARCHICAL MAB
// ==============================

/*
Struct: HierarchicalMAB
Description:
  Two-level Hierarchical Multi-Armed Bandit.
  Level 1: Thompson Sampling over graduate models (which model to use).
  Level 2: Thompson Sampling over portfolio agents (capital allocation).

  Per myreq2.txt §20-21.

Fields:
  - mu        sync.RWMutex        : Protects both levels
  - modelTS   *ThompsonSampling   : Level 1 - model selection
  - agentTS   *ThompsonSampling   : Level 2 - agent capital allocation
  - modelIDs  []string            : Model IDs mapped to Level 1 arms
  - agentIDs  []string            : Agent IDs mapped to Level 2 arms

Lines: ~8
*/
type HierarchicalMAB struct {
	mu       sync.RWMutex
	modelTS  *ThompsonSampling
	agentTS  *ThompsonSampling
	modelIDs []string
	agentIDs []string
}

/******************************************************************************
 * Function Name : NewHierarchicalMAB
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
  * Function Name : NewHierarchicalMAB
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
func NewHierarchicalMAB(nModels, nAgents int, rng *rand.Rand) *HierarchicalMAB {
	return &HierarchicalMAB{
		modelTS: NewThompsonSampling(nModels, rng),
		agentTS: NewThompsonSampling(nAgents, rng),
	}
}

/******************************************************************************
 * Function Name : SelectModel
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

func (hmab *HierarchicalMAB) SelectModel() int {
	return hmab.modelTS.SelectArm()
}

/******************************************************************************
 * Function Name : SelectAgent
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

func (hmab *HierarchicalMAB) SelectAgent() int {
	return hmab.agentTS.SelectArm()
}

/******************************************************************************
 * Function Name : UpdateModelEvidence
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
  * Function Name : UpdateModelEvidence
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
func (hmab *HierarchicalMAB) UpdateModelEvidence(modelIdx int, kpi float64) {
	hmab.modelTS.UpdateArm(modelIdx, kpi)
}

/******************************************************************************
 * Function Name : UpdateAgentEvidence
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

func (hmab *HierarchicalMAB) UpdateAgentEvidence(agentIdx int, kpi float64) {
	hmab.agentTS.UpdateArm(agentIdx, kpi)
}

/******************************************************************************
 * Function Name : ModelProbabilities
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

func (hmab *HierarchicalMAB) ModelProbabilities() []float64 {
	return hmab.modelTS.GetProbabilities()
}

/******************************************************************************
 * Function Name : AgentProbabilities
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
  * Function Name : AgentProbabilities
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
func (hmab *HierarchicalMAB) AgentProbabilities() []float64 {
	return hmab.agentTS.GetProbabilities()
}

/******************************************************************************
 * Function Name : ModelArmCount
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

func (hmab *HierarchicalMAB) ModelArmCount() int {
	return hmab.modelTS.ArmCount()
}

/******************************************************************************
 * Function Name : AgentArmCount
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

func (hmab *HierarchicalMAB) AgentArmCount() int {
	return hmab.agentTS.ArmCount()
}
