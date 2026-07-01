/******************************************************************************
 * File Name       : unsupervised_test.go
 * File Path       : school/unsupervised_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : 6 positive + 2 negative tests for unsupervised model registry.
 * Usage           : go test ./school -v -run Unsupervised
 ******************************************************************************/

package school

import "testing"

func TestAllUnsupervisedModels_Count(t *testing.T) {
	if len(AllUnsupervisedModels()) != 9 {
		t.Errorf("Expected 9 unsupervised models, got %d", len(AllUnsupervisedModels()))
	}
}

func TestValidateUnsupervisedModel_Known(t *testing.T) {
	for _, mt := range AllUnsupervisedModels() {
		if !ValidateUnsupervisedModel(mt) {
			t.Errorf("Expected ValidateUnsupervisedModel(%q)=true", mt)
		}
	}
}

func TestNewUnsupervisedModel_ProducesValidMetadata(t *testing.T) {
	m := NewUnsupervisedModel(ModelUnsDBSCAN, 0)
	if m == nil || m.Architecture != ModelUnsDBSCAN || m.Hyperparameters["eps"] != "0.5" {
		t.Errorf("Invalid: %+v", m)
	}
}

func TestCategoryForUnsupervisedModel_Mapping(t *testing.T) {
	if c := CategoryForUnsupervisedModel(ModelUnsKMeans); c != CategoryLiquidity {
		t.Errorf("Expected KMeans → Liquidity, got %s", c)
	}
	if c := CategoryForUnsupervisedModel(ModelUnsIsolationForest); c != CategoryRisk {
		t.Errorf("Expected IF → Risk, got %s", c)
	}
}

func TestIsUnsupervisedModel_Valid(t *testing.T) {
	if !IsUnsupervisedModel(NewUnsupervisedModel(ModelUnsUMAP, 1)) {
		t.Error("Expected IsUnsupervisedModel=true")
	}
}

func TestIsUnsupervisedModel_NotOther(t *testing.T) {
	if IsUnsupervisedModel(NewStatisticalModel(ModelStatGARCH, 0)) ||
		IsUnsupervisedModel(NewSupervisedModel(ModelSupSVM, 0)) ||
		IsUnsupervisedModel(NewRLModel(ModelRLTD3, 0)) {
		t.Error("Unsupervised models should not match other families")
	}
}

// Negative
func TestValidateUnsupervisedModel_Unknown(t *testing.T) {
	if ValidateUnsupervisedModel("") || ValidateUnsupervisedModel("NeverHeard") {
		t.Error("Expected false for unknown/empty")
	}
}

func TestIsUnsupervisedModel_Nil(t *testing.T) {
	if IsUnsupervisedModel(nil) { t.Error("Expected false for nil") }
}
