/******************************************************************************
 * File Name       : deeplearning_test.go
 * File Path       : school/deeplearning_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : 6 positive + 2 negative tests for deep learning model registry.
 * Usage           : go test ./school -v -run DeepLearning
 ******************************************************************************/

package school

import "testing"

func TestAllDeepLearningModels_Count(t *testing.T) {
	if len(AllDeepLearningModels()) != 9 {
		t.Errorf("Expected 9 deep learning models, got %d", len(AllDeepLearningModels()))
	}
}

func TestValidateDeepLearningModel_Known(t *testing.T) {
	for _, mt := range AllDeepLearningModels() {
		if !ValidateDeepLearningModel(mt) {
			t.Errorf("Expected ValidateDeepLearningModel(%q)=true", mt)
		}
	}
}

func TestNewDeepLearningModel_ProducesValidMetadata(t *testing.T) {
	m := NewDeepLearningModel(ModelDLLSTM, 0)
	if m == nil || m.Architecture != ModelDLLSTM || m.Status != StatusTraining {
		t.Errorf("Invalid: %+v", m)
	}
	if m.Hyperparameters["dropout"] != "0.2" {
		t.Errorf("Expected dropout=0.2, got %s", m.Hyperparameters["dropout"])
	}
}

func TestCategoryForDeepLearningModel_Mapping(t *testing.T) {
	if c := CategoryForDeepLearningModel(ModelDLTransformer); c != CategoryPortfolio {
		t.Errorf("Expected Transformer → Portfolio, got %s", c)
	}
	if c := CategoryForDeepLearningModel(ModelDLLSTM); c != CategorySwing {
		t.Errorf("Expected LSTM → Swing, got %s", c)
	}
}

func TestIsDeepLearningModel_Valid(t *testing.T) {
	if !IsDeepLearningModel(NewDeepLearningModel(ModelDLGRU, 1)) {
		t.Error("Expected IsDeepLearningModel=true")
	}
}

func TestIsDeepLearningModel_NotSupervised(t *testing.T) {
	if IsDeepLearningModel(NewSupervisedModel(ModelSupXGBoost, 0)) {
		t.Error("Expected XGBoost not to be deep learning")
	}
}

// Negative
func TestValidateDeepLearningModel_Unknown(t *testing.T) {
	if ValidateDeepLearningModel("") || ValidateDeepLearningModel("QuantumNet") {
		t.Error("Expected false for unknown/empty")
	}
}

func TestIsDeepLearningModel_Nil(t *testing.T) {
	if IsDeepLearningModel(nil) { t.Error("Expected false for nil") }
}
