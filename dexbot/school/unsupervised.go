/******************************************************************************
 * File Name       : unsupervised.go
 * File Path       : school/unsupervised.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : Unsupervised learning model type registry per myreq3.txt §38.
 *   K-Means, DBSCAN, HDBSCAN, GMM, PCA, ICA, UMAP, Isolation Forest, LOF.
 * Usage           : go test ./school -v -run Unsupervised
 ******************************************************************************/

package school

import "time"

const (
	ModelUnsKMeans       = "K-Means"
	ModelUnsDBSCAN       = "DBSCAN"
	ModelUnsHDBSCAN      = "HDBSCAN"
	ModelUnsGMM          = "Gaussian Mixture Model"
	ModelUnsPCA          = "PCA"
	ModelUnsICA          = "ICA"
	ModelUnsUMAP         = "UMAP"
	ModelUnsIsolationForest = "Isolation Forest"
	ModelUnsLOF          = "Local Outlier Factor"
)

var unsupervisedModelCategories = map[string]string{
	ModelUnsKMeans: CategoryLiquidity, ModelUnsDBSCAN: CategoryLiquidity,
	ModelUnsHDBSCAN: CategoryLiquidity, ModelUnsGMM: CategoryVolatility,
	ModelUnsPCA: CategoryRisk, ModelUnsICA: CategoryRisk,
	ModelUnsUMAP: CategoryRisk, ModelUnsIsolationForest: CategoryRisk,
	ModelUnsLOF: CategoryRisk,
}

func AllUnsupervisedModels() []string {
	return []string{ModelUnsKMeans, ModelUnsDBSCAN, ModelUnsHDBSCAN, ModelUnsGMM,
		ModelUnsPCA, ModelUnsICA, ModelUnsUMAP, ModelUnsIsolationForest, ModelUnsLOF}
}

func NewUnsupervisedModel(modelType string, modelIndex int) *ModelMetadata {
	cat, _ := unsupervisedModelCategories[modelType]
	if cat == "" { cat = CategoryRisk }
	return &ModelMetadata{
		Name: modelType + "_uns", Version: "v0.1", Category: cat,
		Status: StatusTraining, Generation: 0, CreatedAt: time.Now(),
		Architecture: modelType,
		Hyperparameters: map[string]string{"clusters": "5", "eps": "0.5", "min_samples": "10"},
		EnsembleComposition: map[string]float64{modelType: 1.0},
		Fitness: &FitnessHistory{Timestamp: time.Now()},
	}
}

func ValidateUnsupervisedModel(modelType string) bool {
	_, ok := unsupervisedModelCategories[modelType]; return ok
}

func IsUnsupervisedModel(m *ModelMetadata) bool {
	return m != nil && ValidateUnsupervisedModel(m.Architecture)
}

func CategoryForUnsupervisedModel(modelType string) string {
	if cat, ok := unsupervisedModelCategories[modelType]; ok { return cat }
	return CategoryRisk
}
