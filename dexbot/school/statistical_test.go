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
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     :
 *   Unit tests for statistical model type registry.
 *   6 positive + 2 negative test cases per rule1.txt §2.
 *
 * Responsibilities:
 *   - Validate AllStatisticalModels returns 10 entries
 *   - Test ValidateStatisticalModel for known/unknown types
 *   - Test NewStatisticalModel produces valid ModelMetadata
 *   - Test CategoryForStatisticalModel mapping
 *   - Test IsStatisticalModel for nil and valid models
 *   - Negative: unknown model type handling
 *   - Negative: nil model in IsStatisticalModel
 *
 * Usage :
 *   Directory : school/
 *   Build     : go test ./school -v -run Statistical
 *   Test      : go test ./school -v -run Statistical
 *
 * Dependencies :
 *   Internal : dexbot/school
 *   External : testing (stdlib)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Function] TestAllStatisticalModels_Count
 *   [Function] TestValidateStatisticalModel_Known
 *   [Function] TestNewStatisticalModel_ProducesValidMetadata
 *   [Function] TestCategoryForStatisticalModel_Mapping
 *   [Function] TestIsStatisticalModel_Valid
 *   [Function] TestArchitectureMap_ReturnsCopy
 *   [Function] TestValidateStatisticalModel_Unknown (negative)
 *   [Function] TestIsStatisticalModel_Nil (negative)
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-29 15:17:00   | deepseek-4.0-pro | 6 pos + 2 neg tests
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   None
 *
 * Notes :
 *   - All model type constants are validated against full list.
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
