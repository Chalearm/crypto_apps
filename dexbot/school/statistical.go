/******************************************************************************
 * File Name       : statistical.go
 * File Path       : school/statistical.go
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
 *   Statistical model type registry for the School daemon.
 *   Provides model type constants and factory functions for all
 *   supported statistical / quantitative finance models per myreq3.txt §34.
 *
 * Responsibilities:
 *   - Define canonical model type constants (ARIMA, GARCH, VAR, etc.)
 *   - Provide AllStatisticalModels() returning all registered types
 *   - Provide NewStatisticalModel() factory creating a ModelMetadata stub
 *   - Validate model type against known statistical model names
 *   - Map each model type to its category constant
 *
 * Usage :
 *   Directory : school/
 *   Build     : go build ./school
 *   Run       : (library — imported by School daemon)
 *   Test      : go test ./school -v -run Statistical
 *
 * Dependencies :
 *   Internal : dexbot/school (ModelMetadata, category constants)
 *   External : time (stdlib)
 *
 * Configuration : None
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Constant]   ModelStatARIMA, ModelStatSARIMA, ModelStatGARCH, etc.
 *   [Function]   AllStatisticalModels, NewStatisticalModel, ValidateStatisticalModel
 *   [Function]   IsStatisticalModel, CategoryForStatisticalModel, ArchitectureMap
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-29 15:17:00   | deepseek-4.0-pro | Initial version
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add hyperparameter presets per model type
 *
 * Notes :
 *   - Per myreq3.txt §34: ARIMA, SARIMA, GARCH, EGARCH, VAR, KF, HMM,
 *     Cointegration, Monte Carlo, Black-Scholes.
 *   - Model types are constants — no runtime registration needed.
 ******************************************************************************/

package school

import "time"

// ==============================
// STATISTICAL MODEL TYPE CONSTANTS
// ==============================

const (
	ModelStatARIMA           = "ARIMA"
	ModelStatSARIMA          = "SARIMA"
	ModelStatGARCH           = "GARCH"
	ModelStatEGARCH          = "EGARCH"
	ModelStatVAR             = "VAR"
	ModelStatKalmanFilter    = "Kalman Filter"
	ModelStatHiddenMarkov    = "Hidden Markov Model"
	ModelStatCointegration   = "Cointegration Model"
	ModelStatMonteCarlo      = "Monte Carlo Simulation"
	ModelStatBlackScholes    = "Black-Scholes"
)

// statisticalModelCategories maps each model type to its category.
var statisticalModelCategories = map[string]string{
	ModelStatARIMA:         CategorySwing,
	ModelStatSARIMA:        CategorySwing,
	ModelStatGARCH:         CategoryVolatility,
	ModelStatEGARCH:        CategoryVolatility,
	ModelStatVAR:           CategoryPortfolio,
	ModelStatKalmanFilter:  CategoryIntraday,
	ModelStatHiddenMarkov:  CategoryRisk,
	ModelStatCointegration: CategoryLongTerm,
	ModelStatMonteCarlo:    CategoryOptions,
	ModelStatBlackScholes:  CategoryOptions,
}

/******************************************************************************
 * Function Name : AllStatisticalModels
 *
 * Purpose :
 *   Returns the complete list of supported statistical model type names.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : []string
 *   Range       : length = 10
 *   Description : All statistical model type constants in definition order.
 *
 * Error Cases :
 *   None
 *
 * Dependencies :
 *   None
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines : 13
 *
 * Notes :
 *   - Used by GA engine to seed initial population with statistical models.
 ******************************************************************************/
func AllStatisticalModels() []string {
	return []string{
		ModelStatARIMA,
		ModelStatSARIMA,
		ModelStatGARCH,
		ModelStatEGARCH,
		ModelStatVAR,
		ModelStatKalmanFilter,
		ModelStatHiddenMarkov,
		ModelStatCointegration,
		ModelStatMonteCarlo,
		ModelStatBlackScholes,
	}
}

/******************************************************************************
 * Function Name : NewStatisticalModel
 *
 * Purpose :
 *   Creates a ModelMetadata stub for a given statistical model type.
 *   The model is initialized with StatusTraining, Generation 0, and
 *   a default ensemble composition.
 *
 * Inputs :
 *   modelType
 *     Type        : string
 *     Range       : One of the ModelStat* constants
 *     Description : The statistical model type name.
 *
 *   modelIndex
 *     Type        : int
 *     Range       : >= 0
 *     Description : Index number for unique naming.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : *ModelMetadata
 *   Range       : never nil
 *   Description : Initialized model metadata ready for GA population.
 *
 * Error Cases :
 *   - Unknown modelType : returns a model with Architecture = "Unknown"
 *     and CategoryOptions as fallback.
 *
 * Dependencies :
 *   - CategoryForStatisticalModel
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines : 23
 *
 * Notes :
 *   - Hyperparameters are default stubs; real values come from training.
 *   - Ensemble composition defaults to single-model (self-weight 1.0).
 ******************************************************************************/
func NewStatisticalModel(modelType string, modelIndex int) *ModelMetadata {
	cat, ok := statisticalModelCategories[modelType]
	if !ok {
		cat = CategoryOptions
	}
	return &ModelMetadata{
		Name:           modelType + "_stat",
		Version:        "v0.1",
		Category:       cat,
		Status:         StatusTraining,
		Generation:     0,
		CreatedAt:      time.Now(),
		Architecture:   modelType,
		Hyperparameters: map[string]string{"order": "1", "seasonal": "false"},
		EnsembleComposition: map[string]float64{modelType: 1.0},
		Fitness: &FitnessHistory{
			Timestamp: time.Now(),
		},
	}
}

/******************************************************************************
 * Function Name : ValidateStatisticalModel
 *
 * Purpose :
 *   Checks whether a given model type name is a known statistical model.
 *
 * Inputs :
 *   modelType
 *     Type        : string
 *     Range       : any
 *     Description : Model type name to validate.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : bool
 *   Range       : true or false
 *   Description : true if modelType is a recognized statistical model.
 *
 * Error Cases :
 *   None
 *
 * Dependencies :
 *   None
 *
 * Complexity :
 *   Time  : O(1) — map lookup
 *   Space : O(1)
 *
 * Number Of Lines : 5
 *
 * Notes :
 *   - Case-sensitive comparison.
 ******************************************************************************/
func ValidateStatisticalModel(modelType string) bool {
	_, ok := statisticalModelCategories[modelType]
	return ok
}

/******************************************************************************
 * Function Name : IsStatisticalModel
 *
 * Purpose :
 *   Checks whether a ModelMetadata has a statistical model Architecture.
 *
 * Inputs :
 *   m
 *     Type        : *ModelMetadata
 *     Range       : non-nil
 *     Description : Model to check.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : bool
 *   Description : true if Architecture is a known statistical model.
 *
 * Error Cases :
 *   - nil m : returns false
 *
 * Dependencies :
 *   - ValidateStatisticalModel
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines : 7
 *
 * Notes :
 *   None.
 ******************************************************************************/
func IsStatisticalModel(m *ModelMetadata) bool {
	if m == nil {
		return false
	}
	return ValidateStatisticalModel(m.Architecture)
}

/******************************************************************************
 * Function Name : CategoryForStatisticalModel
 *
 * Purpose :
 *   Returns the category constant for a given statistical model type.
 *
 * Inputs :
 *   modelType
 *     Type        : string
 *     Range       : any
 *     Description : Model type name.
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : string
 *   Description : Category constant, or CategoryOptions for unknown types.
 *
 * Error Cases :
 *   - Unknown type : returns CategoryOptions as fallback.
 *
 * Dependencies :
 *   None
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Number Of Lines : 7
 *
 * Notes :
 *   None.
 ******************************************************************************/
func CategoryForStatisticalModel(modelType string) string {
	if cat, ok := statisticalModelCategories[modelType]; ok {
		return cat
	}
	return CategoryOptions
}

/******************************************************************************
 * Function Name : ArchitectureMap
 *
 * Purpose :
 *   Returns the full map of statistical model types to their categories.
 *
 * Inputs :
 *   None
 *
 * Outputs :
 *   None
 *
 * Return :
 *   Type        : map[string]string
 *   Description : Model type → category mapping (shallow copy).
 *
 * Error Cases :
 *   None
 *
 * Dependencies :
 *   None
 *
 * Complexity :
 *   Time  : O(n) where n = number of statistical models
 *   Space : O(n)
 *
 * Number Of Lines : 8
 *
 * Notes :
 *   - Returns a copy to prevent mutation of the internal map.
 ******************************************************************************/
func ArchitectureMap() map[string]string {
	out := make(map[string]string, len(statisticalModelCategories))
	for k, v := range statisticalModelCategories {
		out[k] = v
	}
	return out
}
