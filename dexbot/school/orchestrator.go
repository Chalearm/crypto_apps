/******************************************************************************
 * File Name       : orchestrator.go
 * File Path       : school/orchestrator.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:49 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:49 (UTC+7)
 *
 * Description     :
  *
  * Return :
  *   Type        : varies
  *   Description : Result of computation.
 *   Training orchestrator: splits model population across local GA engine and remote school sub-daemons. Merges remote fitness results back into the local model population. If no remote schools configured
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
 *   1.0.0   | 2026-07-01 19:25:49 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"fmt"
	"time"

	"dexbot/infra"
)

// ==============================
// ORCHESTRATOR
// ==============================

/*
Struct: Orchestrator
Description:
  Coordinates local GA evolution with remote school training.
  Single point of control for the School daemon's training cycles.

Fields:
  - ga     *GAEngine      : Local genetic algorithm engine
  - remote *RemoteClient  : Remote school client (nil if no remotes)
  - pop    *ModelPopulation : The model population

Lines: ~5
*/
type Orchestrator struct {
	ga         *GAEngine
	remote     *RemoteClient
	pop        *ModelPopulation
	dispatcher *Dispatcher      // §91: training dispatcher for MODE_D subprocess
	tierMgr    *SchoolTierManager // §90: 4-tier school manager
}

/******************************************************************************
 * Function Name : NewOrchestrator
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

func NewOrchestrator(ga *GAEngine, remote *RemoteClient, pop *ModelPopulation) *Orchestrator {
	infra.FnTrace("entering")
	defer infra.FnTrace("OK")
	return &Orchestrator{
		ga:         ga,
		remote:     remote,
		pop:        pop,
		dispatcher: NewDispatcher(remote),
		tierMgr:    NewSchoolTierManager(),
	}
}

// SetTierManager links the 4-tier school manager (§90).
/******************************************************************************
 * Function Name : SetTierManager
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

func (o *Orchestrator) SetTierManager(stm *SchoolTierManager) {
	o.tierMgr = stm
}

// SetDispatcher sets a custom training dispatcher (§91).
/******************************************************************************
 * Function Name : SetDispatcher
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

func (o *Orchestrator) SetDispatcher(d *Dispatcher) {
	o.dispatcher = d
}

/******************************************************************************
 * Function Name : RunCycle
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

func (o *Orchestrator) RunCycle(weights FitnessWeights) string {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	// 1. Collect trainable models (active + training, not graduated, not retired)
	trainable := o.collectTrainable()
	infra.FnTrace(fmt.Sprintf("trainable=%d remote=%v", len(trainable), o.remote != nil && o.remote.IsEnabled()))

	if len(trainable) == 0 {
		return "no trainable models"
	}

	// 2. Remote training
	remoteResults := 0
	if o.remote != nil && o.remote.IsEnabled() {
		results, remaining := o.remote.DistributeTraining(trainable)
		trainable = remaining
		remoteResults = o.applyRemoteResults(results)
		infra.FnTrace(fmt.Sprintf("remoteResults=%d remainingLocal=%d", remoteResults, len(trainable)))
	}

	// 3. Local GA on remaining models
	localGraduated, localRetired, summary := o.ga.Evolve(weights)
	_ = localRetired

	return fmt.Sprintf("cycle: remote=%d local=%s graduated=%d",
		remoteResults, summary, localGraduated)
}

/******************************************************************************
 * Function Name : collectTrainable
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

func (o *Orchestrator) collectTrainable() []*ModelMetadata {
	infra.FnTrace("entering")
	categories := []string{
		CategoryOptions, CategoryRisk, CategoryIntraday, CategorySwing,
		CategoryLongTerm, CategoryVolatility, CategoryLiquidity, CategoryPortfolio,
	}
	seen := make(map[string]bool)
	var models []*ModelMetadata
	for _, cat := range categories {
		for _, m := range o.pop.ListByCategory(cat) {
			if m.Status != StatusRetired && m.Status != StatusGraduate && !seen[m.Name] {
				seen[m.Name] = true
				models = append(models, m)
			}
		}
	}
	infra.FnTrace(fmt.Sprintf("%d trainable models found", len(models)))
	return models
}

/******************************************************************************
 * Function Name : applyRemoteResults
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

func (o *Orchestrator) applyRemoteResults(results []TrainingResult) int {
	infra.FnTrace(fmt.Sprintf("applying %d remote results", len(results)))
	applied := 0
	for _, r := range results {
		if r.Err != "" {
			infra.Warn(fmt.Sprintf("Remote result failed: %s from %s: %s", r.ModelName, r.RemoteAddr, r.Err))
			continue
		}
		m := o.pop.Get(r.ModelName)
		if m == nil {
			infra.Warn(fmt.Sprintf("Remote result for unknown model: %s", r.ModelName))
			continue
		}
		now := time.Now()
		m.Fitness = &FitnessHistory{
			Timestamp:          now,
			SharpeRatio:        r.Sharpe,
			SortinoRatio:       r.Sortino,
			Profit:             r.Profit,
			Drawdown:           r.Drawdown,
			PredictionAccuracy: r.Accuracy,
			Consistency:        r.Consistency,
			CapitalEfficiency:  r.Efficiency,
		}
		m.FitnessTimeline = append(m.FitnessTimeline, *m.Fitness)
		// Cap timeline at 20 entries
		if len(m.FitnessTimeline) > 20 {
			m.FitnessTimeline = m.FitnessTimeline[len(m.FitnessTimeline)-20:]
		}
		applied++
	}
	return applied
}
