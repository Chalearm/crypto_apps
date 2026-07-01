/******************************************************************************
 * File Name       : reinforcement.go
 * File Path       : school/reinforcement.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 15:17:00 (UTC+7)
 * Modified Date   : 2026-06-29 15:17:00 (UTC+7)
 *
 * Description     : Reinforcement learning model type registry per myreq3.txt §37.
 *   DQN, PPO, A2C, A3C, SAC, TD3.
 * Usage           : go test ./school -v -run Reinforcement
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
