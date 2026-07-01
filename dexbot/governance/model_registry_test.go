/******************************************************************************
 * File Name       : model_registry_test.go
 * File Path       : governance/model_registry_test.go
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
 *   Unit tests for ModelRegistry. 6 positive + 2 negative test cases.
 *
 * Responsibilities:
 *   - Validate Register/Get round-trip
 *   - Test Graduate/Retire lifecycle
 *   - Verify ListByStatus filtering
 *   - Validate DeploymentRecord + PerformancePoint persistence
 *   - Test FitnessHistory multi-generation snapshot
 *   - Confirm model version independence from software version
 *   - Negative: Graduate/RecordPerformance for nonexistent models
 *
 * Usage :
 *   Directory : governance/
 *   Build     : go test ./governance -v -run ModelRegistry
 *   Test      : go test ./governance -v -run ModelRegistry
 *
 * Dependencies :
 *   Internal : dexbot/governance
 *   External : testing, time (stdlib)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Function] TestModelRegistry_RegisterAndGet
 *   [Function] TestModelRegistry_GraduateAndRetire
 *   [Function] TestModelRegistry_ListByStatus
 *   [Function] TestModelRegistry_DeploymentAndPerformance
 *   [Function] TestModelRegistry_FitnessHistory
 *   [Function] TestModelRegistry_VersionIndependence
 *   [Function] TestModelRegistry_GraduateNonexistent (negative)
 *   [Function] TestModelRegistry_RecordPerformanceNonexistent (negative)
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-28 15:00:00   | deepseek-4.0-pro | 6 pos + 2 neg tests
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add concurrent access stress test
 *
 * Notes :
 *   - All tests use in-memory registry (no DB dependency).
 ******************************************************************************/

package governance

import (
	"testing"
	"time"
)

// helper: create a test model record
func testModel(id string, status string) *ModelRecord {
	return &ModelRecord{
		ID:           id,
		ModelVersion: "v1.0",
		Generation:   1,
		Category:     "Options Prediction",
		Architecture: "LSTM",
		Framework:    "PyTorch",
		Status:       status,
		Hyperparameters: map[string]string{"lr": "0.001", "layers": "3"},
		FeatureSet:   []string{"price", "volume", "rsi"},
		TrainingDataset: "dataset_v2",
		CreatedAt:    time.Now(),
	}
}

// ==============================
// 6 POSITIVE TESTS
// ==============================

func TestModelRegistry_RegisterAndGet(t *testing.T) {
	reg := NewModelRegistry()
	m := testModel("LSTM_v2", ModelStatusExperimental)
	reg.Register(m)

	got := reg.Get("LSTM_v2")
	if got == nil {
		t.Fatal("Expected model to be registered")
	}
	if got.Architecture != "LSTM" {
		t.Errorf("Expected Architecture=LSTM, got %s", got.Architecture)
	}
	if reg.Count() != 1 {
		t.Errorf("Expected Count=1, got %d", reg.Count())
	}
}

func TestModelRegistry_GraduateAndRetire(t *testing.T) {
	reg := NewModelRegistry()
	reg.Register(testModel("XGBoost_v1", ModelStatusExperimental))

	if err := reg.Graduate("XGBoost_v1"); err != nil {
		t.Fatalf("Graduate failed: %v", err)
	}
	m := reg.Get("XGBoost_v1")
	if m.Status != ModelStatusGraduated {
		t.Errorf("Expected graduated status, got %s", m.Status)
	}
	if m.GraduatedAt == nil {
		t.Error("Expected GraduatedAt to be set")
	}

	if err := reg.Retire("XGBoost_v1"); err != nil {
		t.Fatalf("Retire failed: %v", err)
	}
	m = reg.Get("XGBoost_v1")
	if m.Status != ModelStatusRetired {
		t.Errorf("Expected retired status, got %s", m.Status)
	}
	if m.RetiredAt == nil {
		t.Error("Expected RetiredAt to be set")
	}
}

func TestModelRegistry_ListByStatus(t *testing.T) {
	reg := NewModelRegistry()
	reg.Register(testModel("A", ModelStatusExperimental))
	reg.Register(testModel("B", ModelStatusGraduated))
	reg.Register(testModel("C", ModelStatusExperimental))

	exps := reg.ListByStatus(ModelStatusExperimental)
	if len(exps) != 2 {
		t.Errorf("Expected 2 experimental, got %d", len(exps))
	}
	grads := reg.ListByStatus(ModelStatusGraduated)
	if len(grads) != 1 {
		t.Errorf("Expected 1 graduated, got %d", len(grads))
	}
}

func TestModelRegistry_DeploymentAndPerformance(t *testing.T) {
	reg := NewModelRegistry()
	reg.Register(testModel("D", ModelStatusGraduated))

	if err := reg.RecordDeployment("D", "agent_1", 500.0); err != nil {
		t.Fatalf("RecordDeployment: %v", err)
	}
	if err := reg.RecordPerformance("D", PerformancePoint{
		Sharpe: 1.5, PnL: 25.0, Drawdown: 0.05, Trades: 10,
	}); err != nil {
		t.Fatalf("RecordPerformance: %v", err)
	}

	m := reg.Get("D")
	if len(m.Deployments) != 1 {
		t.Errorf("Expected 1 deployment, got %d", len(m.Deployments))
	}
	if m.Deployments[0].AgentID != "agent_1" {
		t.Errorf("Expected agent_1, got %s", m.Deployments[0].AgentID)
	}
	if len(m.PerformanceHistory) != 1 {
		t.Errorf("Expected 1 perf point, got %d", len(m.PerformanceHistory))
	}
}

func TestModelRegistry_FitnessHistory(t *testing.T) {
	reg := NewModelRegistry()
	reg.Register(testModel("E", ModelStatusTraining))

	snapshots := []FitnessSnapshot{
		{Sharpe: 1.0, Sortino: 0.9, Profit: 10.0, Generation: 1},
		{Sharpe: 1.5, Sortino: 1.3, Profit: 25.0, Generation: 2},
		{Sharpe: 2.1, Sortino: 1.8, Profit: 45.0, Generation: 3},
	}
	for _, fs := range snapshots {
		if err := reg.RecordFitness("E", fs); err != nil {
			t.Fatalf("RecordFitness: %v", err)
		}
	}

	m := reg.Get("E")
	if len(m.FitnessScores) != 3 {
		t.Errorf("Expected 3 fitness snapshots, got %d", len(m.FitnessScores))
	}
	latest := m.LatestFitness()
	if latest == nil || latest.Sharpe != 2.1 {
		t.Errorf("Latest fitness mismatch: %+v", latest)
	}
}

func TestModelRegistry_VersionIndependence(t *testing.T) {
	reg := NewModelRegistry()
	m := testModel("F", ModelStatusExperimental)
	m.ModelVersion = "v5.2.1" // model version independent of software
	m.Generation = 42
	reg.Register(m)

	got := reg.Get("F")
	if got.ModelVersion != "v5.2.1" {
		t.Errorf("Expected v5.2.1, got %s", got.ModelVersion)
	}
	if got.Generation != 42 {
		t.Errorf("Expected gen 42, got %d", got.Generation)
	}
}

// ==============================
// 2 NEGATIVE TESTS
// ==============================

func TestModelRegistry_GraduateNonexistent(t *testing.T) {
	reg := NewModelRegistry()
	err := reg.Graduate("ghost")
	if err == nil {
		t.Error("Expected error for nonexistent model graduate")
	}
}

func TestModelRegistry_RecordPerformanceNonexistent(t *testing.T) {
	reg := NewModelRegistry()
	err := reg.RecordPerformance("never_registered", PerformancePoint{Sharpe: 1.0})
	if err == nil {
		t.Error("Expected error for nonexistent model performance update")
	}
}
