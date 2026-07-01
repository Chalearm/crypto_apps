/******************************************************************************
 * File Name       : remote_test.go
 * File Path       : school/remote_test.go
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
 *   Unit tests for RemoteClient and Orchestrator. 5 positive + 2 negative + 1 subsystem integration test. go test ./school -v -run "Remote|Orchestrator" Directory: dexbot/school/
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
 *   [Test Functions] Test suite: TestRemoteClient_DisabledWhenEmptyAddrs, TestRemoteClient_EnabledWhenAddrs, TestRemoteClient_DistributeTrainingEmptyAddrs, TestRemoteClient_DistributeTrainingSplits
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

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

// ==============================
// REMOTE CLIENT TESTS
// ==============================

func TestRemoteClient_DisabledWhenEmptyAddrs(t *testing.T) {
	rc := NewRemoteClient(nil, 30, 5)
	if rc.IsEnabled() {
		t.Error("Expected disabled when no addresses")
	}
	if rc.NodeCount() != 0 {
		t.Errorf("Expected 0 nodes, got %d", rc.NodeCount())
	}
}

func TestRemoteClient_EnabledWhenAddrs(t *testing.T) {
	rc := NewRemoteClient([]string{"10.0.1.5:9001", "10.0.2.3:9001"}, 30, 5)
	if !rc.IsEnabled() {
		t.Error("Expected enabled with addresses")
	}
	if rc.NodeCount() != 2 {
		t.Errorf("Expected 2 nodes, got %d", rc.NodeCount())
	}
}

func TestRemoteClient_DistributeTrainingEmptyAddrs(t *testing.T) {
	rc := NewRemoteClient(nil, 30, 5)
	models := seedPop(10, rand.New(rand.NewSource(1)))
	// collect trainable
	var names []string
	cats := []string{CategoryOptions, CategoryRisk}
	for _, cat := range cats {
		for _, m := range models.ListByCategory(cat) {
			names = append(names, m.Name)
		}
	}
	var trainable []*ModelMetadata
	for _, n := range names {
		trainable = append(trainable, models.Get(n))
	}

	results, local := rc.DistributeTraining(trainable)
	if results != nil {
		t.Error("Expected nil results when no remotes")
	}
	if len(local) != len(trainable) {
		t.Errorf("Expected all models returned for local, got %d/%d", len(local), len(trainable))
	}
}

func TestRemoteClient_DistributeTrainingSplits(t *testing.T) {
	rc := NewRemoteClient([]string{"10.0.1.1:9001", "10.0.1.2:9001"}, 1, 3) // 1s timeout
	// Create 10 fake models
	var models []*ModelMetadata
	for i := 0; i < 10; i++ {
		models = append(models, &ModelMetadata{
			Name: fmt.Sprintf("model_%d", i), Status: StatusTraining,
			Category: CategoryIntraday, CreatedAt: time.Now(),
		})
	}
	results, local := rc.DistributeTraining(models)
	// 2 nodes × 3 students = 6 remote capacity, 4 local
	// Remote will fail (fake IPs) but check local count
	if len(local) != 4 {
		t.Errorf("Expected 4 local models (10 - 6 remote), got %d", len(local))
	}
	_ = results
}

func TestParseTrainingResults(t *testing.T) {
	raw := "result:LSTM_v2:2.1:1.9:0.3:0.15:72.0:0.85:0.90"
	results := parseTrainingResults(raw, "10.0.1.5:9001")
	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.ModelName != "LSTM_v2" || r.RemoteAddr != "10.0.1.5:9001" {
		t.Errorf("Parse mismatch: name=%s addr=%s", r.ModelName, r.RemoteAddr)
	}
	if r.Sharpe != 2.1 || r.Accuracy != 72.0 {
		t.Errorf("Value mismatch: sharpe=%.2f accuracy=%.2f", r.Sharpe, r.Accuracy)
	}
}

func TestParseTrainingResults_Empty(t *testing.T) {
	r := parseTrainingResults("", "x")
	if r != nil {
		t.Error("Expected nil for empty input")
	}
}

// ==============================
// ORCHESTRATOR TESTS
// ==============================

func TestOrchestrator_RunCycleLocalOnly(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	pop := seedPop(20, rng)
	cfg := GAConfig{
		PopulationSize: 20, TopSurvivors: 5,
		MutationRate: 0.15, CrossoverRate: 0.6,
		GenerationsPerCycle: 1, GraduateTopN: 2, RetireBottomN: 2,
		GraduationThreshold: 0.35, RetirementThreshold: 0.15,
	}
	ga := NewGA(cfg, pop, rng)
	orch := NewOrchestrator(ga, nil, pop) // nil remote → local only

	summary := orch.RunCycle(DefaultFitnessWeights())
	if summary == "" {
		t.Error("Expected non-empty summary")
	}
	t.Logf("Orchestrator summary: %s", summary)
}

func TestOrchestrator_CollectTrainable(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pop := seedPop(10, rng)
	ga := NewGA(GAConfig{PopulationSize: 10}, pop, rng)
	orch := NewOrchestrator(ga, nil, pop)

	models := orch.collectTrainable()
	if len(models) == 0 {
		t.Error("Expected trainable models")
	}
	for _, m := range models {
		if m.Status == StatusRetired || m.Status == StatusGraduate {
			t.Errorf("Unexpected status %s for trainable model %s", m.Status, m.Name)
		}
	}
}

func TestOrchestrator_ApplyRemoteResults(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pop := seedPop(5, rng)
	ga := NewGA(GAConfig{PopulationSize: 5}, pop, rng)
	orch := NewOrchestrator(ga, nil, pop)

	// Get first model name
	trainable := orch.collectTrainable()
	if len(trainable) == 0 {
		t.Fatal("Need at least 1 trainable model")
	}
	modelName := trainable[0].Name

	results := []TrainingResult{{
		ModelName: modelName, RemoteAddr: "10.0.1.5:9001",
		Sharpe: 2.5, Sortino: 2.1, Profit: 0.4, Drawdown: 0.1,
		Accuracy: 80.0, Consistency: 0.9, Efficiency: 0.95,
	}}

	applied := orch.applyRemoteResults(results)
	if applied != 1 {
		t.Errorf("Expected 1 applied, got %d", applied)
	}

	m := pop.Get(modelName)
	if m.Fitness == nil || m.Fitness.SharpeRatio != 2.5 {
		t.Errorf("Fitness not updated: %+v", m.Fitness)
	}
}
