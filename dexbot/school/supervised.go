/******************************************************************************
 * File Name       : supervised.go
 * File Path       : school/supervised.go
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
 *   Supervised machine learning model type registry per myreq3.txt §35.
 *
 * Responsibilities:
 *   - Define model type constants (Linear/Ridge/Lasso/ElasticNet, RF/ExtraTrees,
 *     XGBoost/LightGBM/CatBoost, SVM/SVR)
 *   - Provide factory + validation + category mapping
 *
 * Usage :
 *   Directory : school/
 *   Build     : go build ./school
 *   Test      : go test ./school -v -run Supervised
 *
 * Dependencies : dexbot/school (ModelMetadata, categories), time (stdlib)
 *
 * New Parts :
 *   [Constant]   ModelSupLinear, ModelSupRidge, etc.
 *   [Function]   AllSupervisedModels, NewSupervisedModel, ValidateSupervisedModel,
 *               IsSupervisedModel, CategoryForSupervisedModel
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
	ModelSupLinearReg  = "Linear Regression"
	ModelSupRidge      = "Ridge Regression"
	ModelSupLasso      = "Lasso Regression"
	ModelSupElasticNet = "Elastic Net"
	ModelSupRandomForest = "Random Forest"
	ModelSupExtraTrees = "Extra Trees"
	ModelSupXGBoost    = "XGBoost"
	ModelSupLightGBM   = "LightGBM"
	ModelSupCatBoost   = "CatBoost"
	ModelSupSVM        = "Support Vector Machine"
	ModelSupSVR        = "Support Vector Regression"
)

var supervisedModelCategories = map[string]string{
	ModelSupLinearReg: CategorySwing, ModelSupRidge: CategorySwing,
	ModelSupLasso: CategorySwing, ModelSupElasticNet: CategorySwing,
	ModelSupRandomForest: CategoryPortfolio, ModelSupExtraTrees: CategoryPortfolio,
	ModelSupXGBoost: CategoryIntraday, ModelSupLightGBM: CategoryIntraday,
	ModelSupCatBoost: CategoryIntraday, ModelSupSVM: CategoryRisk,
	ModelSupSVR: CategoryRisk,
}

/******************************************************************************
 * Function Name : AllSupervisedModels
 * Purpose       : Returns all 11 supervised model type constants.
 * Return        : []string — model type names in definition order.
 ******************************************************************************/
func AllSupervisedModels() []string {
	return []string{ModelSupLinearReg, ModelSupRidge, ModelSupLasso, ModelSupElasticNet,
		ModelSupRandomForest, ModelSupExtraTrees, ModelSupXGBoost, ModelSupLightGBM,
		ModelSupCatBoost, ModelSupSVM, ModelSupSVR}
}

/******************************************************************************
 * Function Name : NewSupervisedModel
 * Purpose       : Creates a ModelMetadata stub for a given supervised model type.
 * Inputs        : modelType string (ModelSup* constant), modelIndex int
 * Return        : *ModelMetadata — initialized model; falls back to CategoryOptions.
 ******************************************************************************/
func NewSupervisedModel(modelType string, modelIndex int) *ModelMetadata {
	cat, _ := supervisedModelCategories[modelType]
	if cat == "" { cat = CategoryOptions }
	return &ModelMetadata{
		Name: modelType + "_sup", Version: "v0.1", Category: cat,
		Status: StatusTraining, Generation: 0, CreatedAt: time.Now(),
		Architecture: modelType,
		Hyperparameters: map[string]string{"lr": "0.01", "estimators": "100"},
		EnsembleComposition: map[string]float64{modelType: 1.0},
		Fitness: &FitnessHistory{Timestamp: time.Now()},
	}
}

/******************************************************************************
 * Function Name : ValidateSupervisedModel
 * Purpose       : Returns true if modelType is a known supervised model.
 ******************************************************************************/
func ValidateSupervisedModel(modelType string) bool {
	_, ok := supervisedModelCategories[modelType]; return ok
}

/******************************************************************************
 * Function Name : IsSupervisedModel
 * Purpose       : Returns true if m's Architecture is a known supervised model.
 ******************************************************************************/
func IsSupervisedModel(m *ModelMetadata) bool {
	return m != nil && ValidateSupervisedModel(m.Architecture)
}

/******************************************************************************
 * Function Name : CategoryForSupervisedModel
 * Purpose       : Returns category for a supervised model type; falls back to CategoryOptions.
 ******************************************************************************/
func CategoryForSupervisedModel(modelType string) string {
	if cat, ok := supervisedModelCategories[modelType]; ok { return cat }
	return CategoryOptions
}
