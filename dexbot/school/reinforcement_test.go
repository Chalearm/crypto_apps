/******************************************************************************
 * File Name       : reinforcement_test.go
 * File Path       : school/reinforcement_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : 6 positive + 2 negative tests for RL model registry.
 * Usage           : go test ./school -v -run RL
 ******************************************************************************/

package school

import "testing"

func TestAllRLModels_Count(t *testing.T) {
	if len(AllRLModels()) != 6 {
		t.Errorf("Expected 6 RL models, got %d", len(AllRLModels()))
	}
}

func TestValidateRLModel_Known(t *testing.T) {
	for _, mt := range AllRLModels() {
		if !ValidateRLModel(mt) {
			t.Errorf("Expected ValidateRLModel(%q)=true", mt)
		}
	}
}

func TestNewRLModel_ProducesValidMetadata(t *testing.T) {
	m := NewRLModel(ModelRLPPO, 0)
	if m == nil || m.Architecture != ModelRLPPO || m.Hyperparameters["gamma"] != "0.99" {
		t.Errorf("Invalid: %+v", m)
	}
}

func TestCategoryForRLModel_Mapping(t *testing.T) {
	if c := CategoryForRLModel(ModelRLSAC); c != CategoryOptions {
		t.Errorf("Expected SAC → Options, got %s", c)
	}
	if c := CategoryForRLModel(ModelRLPPO); c != CategoryPortfolio {
		t.Errorf("Expected PPO → Portfolio, got %s", c)
	}
}

func TestIsRLModel_Valid(t *testing.T) {
	if !IsRLModel(NewRLModel(ModelRLDQN, 1)) {
		t.Error("Expected IsRLModel=true")
	}
}

func TestIsRLModel_NotOther(t *testing.T) {
	if IsRLModel(NewStatisticalModel(ModelStatARIMA, 0)) ||
		IsRLModel(NewSupervisedModel(ModelSupXGBoost, 0)) ||
		IsRLModel(NewDeepLearningModel(ModelDLLSTM, 0)) {
		t.Error("RL typed models should not match other families")
	}
}

// Negative
func TestValidateRLModel_Unknown(t *testing.T) {
	if ValidateRLModel("") || ValidateRLModel("ImpossibleRL") { t.Error("Expected false") }
}

func TestIsRLModel_Nil(t *testing.T) {
	if IsRLModel(nil) { t.Error("Expected false for nil") }
}
