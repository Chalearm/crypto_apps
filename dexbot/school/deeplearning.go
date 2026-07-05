/******************************************************************************
 * File Name       : deeplearning.go
 * File Path       : school/deeplearning.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:47 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:47 (UTC+7)
 *
 * Description     :
 *   Dexbot component.
 *
 * Responsibilities:
 *   - Implement core functionality.
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
 *   1.0.0   | 2026-07-01 19:25:47 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package school

import "time"

const (
	ModelDLMLP         = "MLP"
	ModelDLCNN         = "CNN"
	ModelDLLSTM        = "LSTM"
	ModelDLGRU         = "GRU"
	ModelDLTransformer = "Transformer"
	ModelDLTFT         = "Temporal Fusion Transformer"
	ModelDLNBeats      = "N-BEATS"
	ModelDLAutoEncoder = "AutoEncoder"
	ModelDLVAE         = "Variational AutoEncoder"
)

var deepLearningModelCategories = map[string]string{
	ModelDLMLP: CategoryLongTerm, ModelDLCNN: CategoryIntraday,
	ModelDLLSTM: CategorySwing, ModelDLGRU: CategorySwing,
	ModelDLTransformer: CategoryPortfolio, ModelDLTFT: CategoryPortfolio,
	ModelDLNBeats: CategoryLongTerm, ModelDLAutoEncoder: CategoryRisk,
	ModelDLVAE: CategoryRisk,
}

func AllDeepLearningModels() []string {
	return []string{ModelDLMLP, ModelDLCNN, ModelDLLSTM, ModelDLGRU,
		ModelDLTransformer, ModelDLTFT, ModelDLNBeats, ModelDLAutoEncoder, ModelDLVAE}
}

func NewDeepLearningModel(modelType string, modelIndex int) *ModelMetadata {
	cat, _ := deepLearningModelCategories[modelType]
	if cat == "" { cat = CategoryLongTerm }
	return &ModelMetadata{
		Name: modelType + "_dl", Version: "v0.1", Category: cat,
		Status: StatusTraining, Generation: 0, CreatedAt: time.Now(),
		Architecture: modelType,
		Hyperparameters: map[string]string{"layers": "3", "units": "128", "dropout": "0.2"},
		EnsembleComposition: map[string]float64{modelType: 1.0},
		Fitness: &FitnessHistory{Timestamp: time.Now()},
	}
}

func ValidateDeepLearningModel(modelType string) bool {
	_, ok := deepLearningModelCategories[modelType]; return ok
}

func IsDeepLearningModel(m *ModelMetadata) bool {
	return m != nil && ValidateDeepLearningModel(m.Architecture)
}

func CategoryForDeepLearningModel(modelType string) string {
	if cat, ok := deepLearningModelCategories[modelType]; ok { return cat }
	return CategoryLongTerm
}
