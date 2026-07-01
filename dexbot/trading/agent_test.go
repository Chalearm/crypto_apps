/******************************************************************************
 * File Name       : agent_test.go
 * File Path       : trading/agent_test.go
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
 *   Unit tests for portfolio agents and Thompson Sampling / H-MAB. 5 positive + 2 negative test cases per coding rule §2. go test ./trading -v - Created during Phase 7 reorganization. - All test functions
 *
 * Responsibilities:
 *   - Implement core functionality for trading package.
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
 *   [Test Functions] Test suite: TestAgentCreation, TestAgentRetirement, TestAgentRetireNonexistent, TestAgentPoolRebalanceCapital
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
package trading

import (
	"math/rand"
	"testing"
)

// ==============================
// AGENT POOL TESTS
// ==============================

func TestAgentCreation(t *testing.T) {
	pool := NewAgentPool()
	a := pool.CreateAgentSimple(CapitalSmall, StrategyHedging)
	if a == nil {
		t.Fatal("Expected non-nil agent")
	}
	if a.Category != CapitalSmall {
		t.Errorf("Expected category=%s, got %s", CapitalSmall, a.Category)
	}
	if a.Strategy != StrategyHedging {
		t.Errorf("Expected strategy=%s, got %s", StrategyHedging, a.Strategy)
	}
	if !a.Active {
		t.Error("Expected agent to be active")
	}
}

func TestAgentRetirement(t *testing.T) {
	pool := NewAgentPool()
	a := pool.CreateAgentSimple(CapitalMedium, StrategyTrend)
	if !pool.RetireAgent(a.ID) {
		t.Error("Expected retire to succeed")
	}
	if pool.GetAgent(a.ID).Active {
		t.Error("Expected agent to be inactive after retirement")
	}
}

func TestAgentRetireNonexistent(t *testing.T) {
	pool := NewAgentPool()
	if pool.RetireAgent("ghost") {
		t.Error("Expected retire to fail for nonexistent agent")
	}
}

func TestAgentPoolRebalanceCapital(t *testing.T) {
	pool := NewAgentPool()
	pool.CreateAgentSimple(CapitalSmall, StrategyArbitrage)
	pool.CreateAgentSimple(CapitalSmall, StrategyLiquidity)
	pool.CreateAgentSimple(CapitalMedium, StrategyTrend)

	pool.RebalanceCapital(300.0)
	if pool.TotalAllocated() != 300.0 {
		t.Errorf("Expected total=300.0, got %.2f", pool.TotalAllocated())
	}

	for _, a := range pool.ActiveAgents() {
		if a.Capital != 100.0 {
			t.Errorf("Expected capital=100.0 for agent %s, got %.2f", a.ID, a.Capital)
		}
	}
}

func TestAgentDuplicateCreation(t *testing.T) {
	pool := NewAgentPool()
	pool.CreateAgentSimple(CapitalSmall, StrategyTrend)
	pool.CreateAgentSimple(CapitalSmall, StrategyTrend)
	if pool.Count() != 2 {
		t.Errorf("Expected 2 agents with unique IDs, got %d", pool.Count())
	}
	// Verify IDs are unique
	a1 := pool.GetAgent("agent_1")
	a2 := pool.GetAgent("agent_2")
	if a1.ID == a2.ID {
		t.Error("Expected unique agent IDs")
	}
}

func TestRebalanceEmptyPool(t *testing.T) {
	pool := NewAgentPool()
	pool.RebalanceCapital(1000.0) // should not panic
	if pool.TotalAllocated() != 0 {
		t.Errorf("Expected 0 allocated for empty pool, got %.2f", pool.TotalAllocated())
	}
}

func TestActiveVsTotalCount(t *testing.T) {
	pool := NewAgentPool()
	pool.CreateAgentSimple(CapitalLarge, StrategyDiversified)
	a := pool.CreateAgentSimple(CapitalSmall, StrategyHedging)
	pool.RetireAgent(a.ID)

	if pool.Count() != 2 {
		t.Errorf("Expected total=2, got %d", pool.Count())
	}
	if pool.ActiveCount() != 1 {
		t.Errorf("Expected active=1, got %d", pool.ActiveCount())
	}
}

// ==============================
// THOMPSON SAMPLING TESTS
// ==============================

func TestThompsonSamplingSelectArm(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ts := NewThompsonSampling(3, rng)

	arm := ts.SelectArm()
	if arm < 0 || arm >= 3 {
		t.Errorf("Expected arm in [0,2], got %d", arm)
	}
}

func TestThompsonSamplingUpdate(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ts := NewThompsonSampling(2, rng)

	// Arm 0 gets success, Arm 1 gets failure
	ts.UpdateArm(0, 1.0) // success
	ts.UpdateArm(0, 0.9) // success
	ts.UpdateArm(1, 0.1) // failure
	ts.UpdateArm(1, 0.2) // failure

	probs := ts.GetProbabilities()
	if probs[0] <= probs[1] {
		t.Errorf("Expected arm 0 (%.4f) > arm 1 (%.4f) after evidence", probs[0], probs[1])
	}
}

func TestThompsonSamplingProbabilitiesOne(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ts := NewThompsonSampling(1, rng)
	probs := ts.GetProbabilities()
	if len(probs) != 1 {
		t.Errorf("Expected 1 probability, got %d", len(probs))
	}
	// With alpha=beta=1, expected probability ≈ 0.5
	if probs[0] < 0.3 || probs[0] > 0.7 {
		t.Errorf("Expected initial prob ≈ 0.5, got %.4f", probs[0])
	}
}

func TestThompsonSamplingOutOfBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ts := NewThompsonSampling(2, rng)
	ts.UpdateArm(-1, 0.5) // should not panic
	ts.UpdateArm(99, 0.5) // should not panic
}

func TestHierarchicalMAB(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	hmab := NewHierarchicalMAB(3, 5, rng)

	model := hmab.SelectModel()
	if model < 0 || model >= 3 {
		t.Errorf("Expected model arm in [0,2], got %d", model)
	}

	agent := hmab.SelectAgent()
	if agent < 0 || agent >= 5 {
		t.Errorf("Expected agent arm in [0,4], got %d", agent)
	}

	hmab.UpdateModelEvidence(0, 0.8)
	hmab.UpdateAgentEvidence(1, 0.3)

	mp := hmab.ModelProbabilities()
	if len(mp) != 3 {
		t.Errorf("Expected 3 model probs, got %d", len(mp))
	}
	ap := hmab.AgentProbabilities()
	if len(ap) != 5 {
		t.Errorf("Expected 5 agent probs, got %d", len(ap))
	}
}

func TestThompsonSamplingZeroArms(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ts := NewThompsonSampling(0, rng)
	arm := ts.SelectArm()
	if arm != -1 {
		t.Errorf("Expected -1 for zero arms, got %d", arm)
	}
}
