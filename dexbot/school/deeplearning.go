/******************************************************************************
 * File Name       : deeplearning.go
 * File Path       : school/deeplearning.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : Deep learning model type registry per myreq3.txt §36.
 *   MLP, CNN, LSTM, GRU, Transformer, TFT, N-BEATS, AutoEncoder, VAE.
 * Usage           : go test ./school -v -run DeepLearning
 *
 * New Parts :
 *   [Constant]   ModelDLMLP, ModelDLCNN, etc.
 *   [Function]   AllDeepLearningModels, NewDeepLearningModel,
 *               ValidateDeepLearningModel, IsDeepLearningModel,
 *               CategoryForDeepLearningModel
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-29 15:17:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
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
