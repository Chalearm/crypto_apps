/******************************************************************************
 * File Name       : reinforcement.go
 * File Path       : school/reinforcement.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:49 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:49 (UTC+7)
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
 *   1.0.0   | 2026-07-01 19:25:49 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	ModelRLDQN  = "DQN"
	ModelRLPPO  = "PPO"
	ModelRLA2C  = "A2C"
	ModelRLA3C  = "A3C"
	ModelRLSAC  = "SAC"
	ModelRLTD3  = "TD3"
)

var rlModelCategories = map[string]string{
	ModelRLDQN: CategoryRisk, ModelRLPPO: CategoryPortfolio,
	ModelRLA2C: CategoryIntraday, ModelRLA3C: CategoryIntraday,
	ModelRLSAC: CategoryOptions, ModelRLTD3: CategoryOptions,
}

func AllRLModels() []string {
	return []string{ModelRLDQN, ModelRLPPO, ModelRLA2C, ModelRLA3C, ModelRLSAC, ModelRLTD3}
}

func NewRLModel(modelType string, modelIndex int) *ModelMetadata {
	cat, _ := rlModelCategories[modelType]
	if cat == "" { cat = CategoryRisk }
	return &ModelMetadata{
		Name: modelType + "_rl", Version: "v0.1", Category: cat,
		Status: StatusTraining, Generation: 0, CreatedAt: time.Now(),
		Architecture: modelType,
		Hyperparameters: map[string]string{"gamma": "0.99", "lr": "0.0003", "batch_size": "64"},
		EnsembleComposition: map[string]float64{modelType: 1.0},
		Fitness: &FitnessHistory{Timestamp: time.Now()},
	}
}

func ValidateRLModel(modelType string) bool {
	_, ok := rlModelCategories[modelType]; return ok
}

func IsRLModel(m *ModelMetadata) bool {
	return m != nil && ValidateRLModel(m.Architecture)
}

func CategoryForRLModel(modelType string) string {
	if cat, ok := rlModelCategories[modelType]; ok { return cat }
	return CategoryRisk
}
