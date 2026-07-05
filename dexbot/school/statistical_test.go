/******************************************************************************
 * File Name       : statistical_test.go
 * File Path       : school/statistical_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:51 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:51 (UTC+7)
 *
 * Description     :
 *   Unit tests for statistical model type registry.
 *
 * Responsibilities:
 *   - - Validate AllStatisticalModels returns 10 entries
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
 *   1.0.0   | 2026-07-01 19:25:51 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package school

import "testing"

func TestAllStatisticalModels_Count(t *testing.T) {
	models := AllStatisticalModels()
	if len(models) != 10 {
		t.Errorf("Expected 10 statistical models, got %d: %v", len(models), models)
	}
	// Verify no duplicates
	seen := make(map[string]bool)
	for _, m := range models {
		if seen[m] {
			t.Errorf("Duplicate model type: %s", m)
		}
		seen[m] = true
	}
}

func TestValidateStatisticalModel_Known(t *testing.T) {
	for _, mt := range AllStatisticalModels() {
		if !ValidateStatisticalModel(mt) {
			t.Errorf("Expected ValidateStatisticalModel(%q) = true", mt)
		}
	}
}

func TestNewStatisticalModel_ProducesValidMetadata(t *testing.T) {
	m := NewStatisticalModel(ModelStatARIMA, 0)
	if m == nil {
		t.Fatal("Expected non-nil model")
	}
	if m.Architecture != ModelStatARIMA {
		t.Errorf("Expected Architecture=%q, got %q", ModelStatARIMA, m.Architecture)
	}
	if m.Status != StatusTraining {
		t.Errorf("Expected Status=StatusTraining, got %s", m.Status)
	}
	if m.Category == "" {
		t.Error("Expected non-empty Category")
	}
	if m.Generation != 0 {
		t.Errorf("Expected Generation=0, got %d", m.Generation)
	}
	if m.Fitness == nil {
		t.Error("Expected non-nil Fitness")
	}
}

func TestCategoryForStatisticalModel_Mapping(t *testing.T) {
	cat := CategoryForStatisticalModel(ModelStatGARCH)
	if cat != CategoryVolatility {
		t.Errorf("Expected GARCH → CategoryVolatility, got %s", cat)
	}
	cat = CategoryForStatisticalModel(ModelStatARIMA)
	if cat != CategorySwing {
		t.Errorf("Expected ARIMA → CategorySwing, got %s", cat)
	}
	cat = CategoryForStatisticalModel(ModelStatMonteCarlo)
	if cat != CategoryOptions {
		t.Errorf("Expected MC → CategoryOptions, got %s", cat)
	}
}

func TestIsStatisticalModel_Valid(t *testing.T) {
	m := NewStatisticalModel(ModelStatVAR, 1)
	if !IsStatisticalModel(m) {
		t.Error("Expected IsStatisticalModel=true for VAR model")
	}
}

func TestArchitectureMap_ReturnsCopy(t *testing.T) {
	m1 := ArchitectureMap()
	m2 := ArchitectureMap()
	if len(m1) != 10 || len(m2) != 10 {
		t.Errorf("Expected 10 entries, got %d and %d", len(m1), len(m2))
	}
	m1["FAKE"] = "fake"
	if _, ok := m2["FAKE"]; ok {
		t.Error("ArchitectureMap should return a copy (mutation of m1 leaked to m2)")
	}
}

// ==============================
// NEGATIVE TESTS
// ==============================

func TestValidateStatisticalModel_Unknown(t *testing.T) {
	if ValidateStatisticalModel("QuantumNeuralFusion") {
		t.Error("Expected false for unknown model type")
	}
	if ValidateStatisticalModel("") {
		t.Error("Expected false for empty string")
	}
}

func TestIsStatisticalModel_Nil(t *testing.T) {
	if IsStatisticalModel(nil) {
		t.Error("Expected false for nil model")
	}
}
