/******************************************************************************
 * File Name       : model_test.go
 * File Path       : school/model_test.go
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
 *   Unit tests for school model types and fitness scoring. 5 positive + 2 negative test cases per coding rule §2. go test ./school -v - Created during Phase 7 reorganization. - All test functions below.
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
 *   [Test Functions] Test suite: TestModelMetadataValidation, TestModelMetadataInvalidWeights, TestModelMetadataBadWeights, TestPopulationAddAndGet
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
	"testing"
	"time"
)

// ==============================
// MODEL METADATA TESTS
// ==============================

/*
Function: TestModelMetadataValidation
Description:
  Positive: ValidateEnsembleWeights returns true for valid weights summing to 1.0.
Lines: ~15
*/
func TestModelMetadataValidation(t *testing.T) {
	m := &ModelMetadata{
		Name: "LSTM_v2",
		EnsembleComposition: map[string]float64{
			"SVM": 0.30, "ARIMA": 0.20, "LSTM": 0.25, "RL": 0.15, "BS": 0.10,
		},
	}
	if !m.ValidateEnsembleWeights() {
		t.Error("Expected valid ensemble weights summing to 1.0")
	}
}

/*
Function: TestModelMetadataInvalidWeights
Description:
  Negative: Empty ensemble composition returns false.
Lines: ~8
*/
func TestModelMetadataInvalidWeights(t *testing.T) {
	m := &ModelMetadata{Name: "Empty"}
	if m.ValidateEnsembleWeights() {
		t.Error("Expected invalid for empty ensemble composition")
	}
}

/*
Function: TestModelMetadataBadWeights
Description:
  Negative: Weights not summing to 1.0 return false.
Lines: ~12
*/
func TestModelMetadataBadWeights(t *testing.T) {
	m := &ModelMetadata{
		Name: "Bad",
		EnsembleComposition: map[string]float64{
			"A": 0.5, "B": 0.3, // sums to 0.8
		},
	}
	if m.ValidateEnsembleWeights() {
		t.Error("Expected invalid for weights not summing to 1.0")
	}
}

// ==============================
// MODEL POPULATION TESTS
// ==============================

/*
Function: TestPopulationAddAndGet
Description:
  Positive: AddModel adds, Get returns, Count reflects.
Lines: ~15
*/
func TestPopulationAddAndGet(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "A", Status: StatusTraining})
	p.AddModel(&ModelMetadata{Name: "B", Status: StatusTraining})

	if p.Count() != 2 {
		t.Errorf("Expected Count=2, got %d", p.Count())
	}
	if p.Get("A") == nil {
		t.Error("Expected to get model A")
	}
	if p.Get("C") != nil {
		t.Error("Expected nil for nonexistent model")
	}
}

/*
Function: TestPopulationGraduate
Description:
  Positive: Graduate promotes model and sets GraduatedAt.
Lines: ~15
*/
func TestPopulationGraduate(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "X", Status: StatusTraining})

	if !p.Graduate("X") {
		t.Error("Expected Graduate to succeed")
	}
	m := p.Get("X")
	if m.Status != StatusGraduate {
		t.Errorf("Expected StatusGraduate, got %s", m.Status)
	}
	if m.GraduatedAt == nil {
		t.Error("Expected GraduatedAt to be set")
	}
	if p.GraduateCount() != 1 {
		t.Errorf("Expected GraduateCount=1, got %d", p.GraduateCount())
	}
}

/*
Function: TestPopulationRetire
Description:
  Positive: Retire marks model as retired.
Lines: ~12
*/
func TestPopulationRetire(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "Y", Status: StatusActive})

	p.Retire("Y")
	if p.Get("Y").Status != StatusRetired {
		t.Errorf("Expected StatusRetired, got %s", p.Get("Y").Status)
	}
	if p.ActiveCount() != 0 {
		t.Errorf("Expected ActiveCount=0 after retire, got %d", p.ActiveCount())
	}
}

/*
Function: TestPopulationGraduateNonexistent
Description:
  Negative: Graduate returns false for nonexistent model.
Lines: ~8
*/
func TestPopulationGraduateNonexistent(t *testing.T) {
	p := NewModelPopulation()
	if p.Graduate("ghost") {
		t.Error("Expected Graduate to fail for nonexistent model")
	}
}

/*
Function: TestPopulationRemove
Description:
  Positive: Remove permanently deletes model.
Lines: ~10
*/
func TestPopulationRemove(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "Z", Status: StatusTraining})
	p.Remove("Z")
	if p.Get("Z") != nil {
		t.Error("Expected nil after Remove")
	}
	if p.Count() != 0 {
		t.Errorf("Expected Count=0, got %d", p.Count())
	}
}

/*
Function: TestPopulationListByCategory
Description:
  Positive: ListByCategory returns only matching models.
Lines: ~15
*/
func TestPopulationListByCategory(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "Opt1", Category: CategoryOptions, Status: StatusTraining})
	p.AddModel(&ModelMetadata{Name: "Risk1", Category: CategoryRisk, Status: StatusTraining})
	p.AddModel(&ModelMetadata{Name: "Opt2", Category: CategoryOptions, Status: StatusTraining})

	opts := p.ListByCategory(CategoryOptions)
	if len(opts) != 2 {
		t.Errorf("Expected 2 options models, got %d", len(opts))
	}
}

// ==============================
// FITNESS TESTS
// ==============================

/*
Function: TestCompositeFitnessValid
Description:
  Positive: CompositeFitness returns a valid score with default weights.
Lines: ~20
*/
func TestCompositeFitnessValid(t *testing.T) {
	fh := &FitnessHistory{
		Timestamp:          time.Now(),
		SharpeRatio:        2.0,
		SortinoRatio:       2.5,
		Profit:             0.3,
		Drawdown:           0.15,
		PredictionAccuracy: 72.0,
		Consistency:        0.85,
		CapitalEfficiency:  0.90,
	}
	score, ok := CompositeFitness(fh, DefaultFitnessWeights())
	if !ok {
		t.Error("Expected valid composite fitness")
	}
	if score < 0 || score > 1.0 {
		t.Errorf("Expected score in [0,1], got %.4f", score)
	}
}

/*
Function: TestFitnessWeightsValidation
Description:
  Positive: Default weights validate correctly.
Lines: ~8
*/
func TestFitnessWeightsValidation(t *testing.T) {
	w := DefaultFitnessWeights()
	if !ValidateWeights(w) {
		t.Error("Expected default weights to be valid")
	}
}

/*
Function: TestFitnessWeightsInvalid
Description:
  Negative: Invalid weights (sum >> 1.0) fail validation.
Lines: ~12
*/
func TestFitnessWeightsInvalid(t *testing.T) {
	w := FitnessWeights{
		SharpeWeight:      0.5,
		SortinoWeight:     0.5,
		ProfitWeight:      0.5,
		DrawdownWeight:    -0.5,
		AccuracyWeight:    0.5,
		ConsistencyWeight: 0.5,
		EfficiencyWeight:  0.5,
	}
	if ValidateWeights(w) {
		t.Error("Expected invalid weights to fail")
	}
}

/*
Function: TestCompositeFitnessInvalidWeights
Description:
  Negative: CompositeFitness with bad weights returns ok=false.
Lines: ~12
*/
func TestCompositeFitnessInvalidWeights(t *testing.T) {
	fh := &FitnessHistory{SharpeRatio: 1.0}
	w := FitnessWeights{SharpeWeight: 99.0} // malformed
	_, ok := CompositeFitness(fh, w)
	if ok {
		t.Error("Expected ok=false for invalid weights")
	}
}

/*
Function: TestGraduatesSorted
Description:
  Positive: Graduates returns sorted list.
Lines: ~15
*/
func TestGraduatesSorted(t *testing.T) {
	p := NewModelPopulation()
	p.AddModel(&ModelMetadata{Name: "ZModel", Status: StatusGraduate})
	p.AddModel(&ModelMetadata{Name: "AModel", Status: StatusGraduate})
	p.AddModel(&ModelMetadata{Name: "MModel", Status: StatusTraining})

	grads := p.Graduates()
	if len(grads) != 2 {
		t.Errorf("Expected 2 graduates, got %d", len(grads))
	}
	if grads[0].Name != "AModel" || grads[1].Name != "ZModel" {
		t.Errorf("Expected sorted graduates [AModel, ZModel], got [%s, %s]",
			grads[0].Name, grads[1].Name)
	}
}
