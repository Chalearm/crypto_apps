/******************************************************************************
 * File Name       : supervised_test.go
 * File Path       : school/supervised_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : 6 positive + 2 negative tests for supervised ML model registry.
 * Usage           : go test ./school -v -run Supervised
 ******************************************************************************/

package school

import "testing"

func TestAllSupervisedModels_Count(t *testing.T) {
	if len(AllSupervisedModels()) != 11 {
		t.Errorf("Expected 11 supervised models, got %d", len(AllSupervisedModels()))
	}
}

func TestValidateSupervisedModel_Known(t *testing.T) {
	for _, mt := range AllSupervisedModels() {
		if !ValidateSupervisedModel(mt) {
			t.Errorf("Expected ValidateSupervisedModel(%q)=true", mt)
		}
	}
}

func TestNewSupervisedModel_ProducesValidMetadata(t *testing.T) {
	m := NewSupervisedModel(ModelSupXGBoost, 0)
	if m == nil || m.Architecture != ModelSupXGBoost || m.Status != StatusTraining {
		t.Errorf("Invalid metadata from NewSupervisedModel: %+v", m)
	}
}

func TestCategoryForSupervisedModel_Mapping(t *testing.T) {
	if CategoryForSupervisedModel(ModelSupXGBoost) != CategoryIntraday {
		t.Errorf("Expected XGBoost → CategoryIntraday, got %s", CategoryForSupervisedModel(ModelSupXGBoost))
	}
	if CategoryForSupervisedModel(ModelSupSVM) != CategoryRisk {
		t.Errorf("Expected SVM → CategoryRisk, got %s", CategoryForSupervisedModel(ModelSupSVM))
	}
}

func TestIsSupervisedModel_Valid(t *testing.T) {
	if !IsSupervisedModel(NewSupervisedModel(ModelSupRandomForest, 1)) {
		t.Error("Expected IsSupervisedModel=true")
	}
}

func TestIsSupervisedModel_NotStatistical(t *testing.T) {
	if IsSupervisedModel(NewStatisticalModel(ModelStatARIMA, 0)) {
		t.Error("Expected ARIMA not to be a supervised model")
	}
}

// Negative
func TestValidateSupervisedModel_Unknown(t *testing.T) {
	if ValidateSupervisedModel("") || ValidateSupervisedModel("FakeModel") {
		t.Error("Expected false for unknown/empty model type")
	}
}

func TestIsSupervisedModel_Nil(t *testing.T) {
	if IsSupervisedModel(nil) {
		t.Error("Expected false for nil")
	}
}
