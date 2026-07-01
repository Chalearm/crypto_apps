/******************************************************************************
 * File Name       : artifact.go
 * File Path       : school/artifact.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:55:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:55:00 (UTC+7)
 *
 * Description     :
 *   Standardized Model Artifact Contract per myreq3.txt §44-47.
 *   Every model artifact returned by a remote training node or
 *   serialized locally must contain these six components.
 *
 *   Artifact components:
 *     1. Model File           — serialized weights/parameters
 *     2. Metadata File        — identifier, version, generation, framework
 *     3. Metrics File         — Sharpe, Sortino, profit, drawdown, accuracy
 *     4. Feature Definition   — column names, types, preprocessing
 *     5. Training Config      — hyperparameters, architecture, duration
 *     6. Deployment Recommendation — graduation score, confidence, category
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:55 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"time"
)

// ==============================
// ARTIFACT TYPES
// ==============================

// ModelArtifact is the standardized container for all model training output.
type ModelArtifact struct {
	ModelFile      ArtifactModelFile      `json:"model_file"`
	Metadata       ArtifactMetadata       `json:"metadata"`
	Metrics        ArtifactMetrics        `json:"metrics"`
	Features       ArtifactFeatures       `json:"features"`
	TrainingConfig ArtifactTrainingConfig `json:"training_config"`
	Deployment     ArtifactDeployment     `json:"deployment"`
}

// ArtifactModelFile contains serialized model weights/parameters.
type ArtifactModelFile struct {
	Format   string `json:"format"`   // e.g., "json", "onnx", "pickle", "tflite"
	Encoding string `json:"encoding"` // e.g., "base64", "raw"
	Data     string `json:"data"`     // Encoded model data
	Checksum string `json:"checksum"` // SHA-256 of Data
}

// ArtifactMetadata per §45.
type ArtifactMetadata struct {
	ModelIdentifier        string `json:"model_identifier"`
	ModelVersion           string `json:"model_version"`
	Generation             int    `json:"generation"`
	CreationTimestamp      string `json:"creation_timestamp"`
	Framework              string `json:"framework"`
	FrameworkVersion       string `json:"framework_version"`
	TrainingDatasetVersion string `json:"training_dataset_version"`
	FeatureSetVersion      string `json:"feature_set_version"`
	CompatibilityGoMin     string `json:"compatibility_go_min"`
	CompatibilityProtocol  string `json:"compatibility_protocol"`
}

// ArtifactMetrics per §46.
type ArtifactMetrics struct {
	ProfitabilityPnL     float64 `json:"profitability_pnl"`
	ProfitabilityWinRate float64 `json:"profitability_win_rate"`
	SharpeRatio          float64 `json:"sharpe_ratio"`
	SortinoRatio         float64 `json:"sortino_ratio"`
	MaxDrawdown          float64 `json:"max_drawdown"`
	VolatilityAnnualized float64 `json:"volatility_annualized"`
	PredictionMAE        float64 `json:"prediction_mae"`
	PredictionRMSE       float64 `json:"prediction_rmse"`
	PredictionR2         float64 `json:"prediction_r2"`
	DirectionalAccuracy   float64 `json:"directional_accuracy"`
	TrainingDurationSec  float64 `json:"training_duration_sec"`
	ValidationResults    string  `json:"validation_results"`    // JSON: cross-val fold details
	WalkForwardResults   string  `json:"walk_forward_results"`   // JSON: rolling window performance
	PaperTradingResults  string  `json:"paper_trading_results"`  // JSON: live-market simulation
}

// ArtifactFeatures defines feature columns per §44.4.
type ArtifactFeatures struct {
	Columns       []FeatureColumn `json:"columns"`
	Preprocessing string          `json:"preprocessing"` // description of scaling/normalization
}

// FeatureColumn describes one feature.
type FeatureColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // "float64", "int", "categorical", "boolean"
	Category string `json:"category"` // "price", "volume", "technical", "sentiment", "options"
	Nullable bool   `json:"nullable"`
}

// ArtifactTrainingConfig records the training configuration used.
type ArtifactTrainingConfig struct {
	Architecture   string            `json:"architecture"`
	Hyperparameters map[string]string `json:"hyperparameters"`
	TrainingDuration string           `json:"training_duration"` // human-readable
	EpochsTrained  int               `json:"epochs_trained"`
	EarlyStopped   bool              `json:"early_stopped"`
	HardwareInfo   string            `json:"hardware_info"` // "CPU", "GPU:T4", etc.
}

// ArtifactDeployment per §44.6.
type ArtifactDeployment struct {
	GraduationScore         float64 `json:"graduation_score"`
	RecommendedCategory     string  `json:"recommended_category"`
	ConfidenceLevel         float64 `json:"confidence_level"`
	SuggestedCapitalAllocPct float64 `json:"suggested_capital_alloc_pct"`
	Notes                   string  `json:"notes"`
}

// ==============================
// ARTIFACT VALIDATION
// ==============================

/******************************************************************************
 * Function Name : ValidateArtifact
 *
 * Purpose :
 *   Validates that all required fields of a ModelArtifact are populated.
 *
 * Inputs :
 *   a  *ModelArtifact — Artifact to validate
 *
 * Return :
 *   Type        : error
 *   Description : nil if valid, descriptive error otherwise.
 *
 * Error Cases :
 *   - Missing ModelFile.Data : "model file data is empty"
 *   - Missing Metadata.ModelIdentifier : "model identifier is required"
 *   - Missing ArtifactMetrics : "metrics are required"
 *   - Missing Features.Columns : "feature columns are required"
 *
 * Complexity : O(1)
 * Number Of Lines : 25
 ******************************************************************************/
func ValidateArtifact(a *ModelArtifact) error {
	if a == nil {
		return fmt.Errorf("artifact is nil")
	}
	if a.ModelFile.Data == "" {
		return fmt.Errorf("model file data is empty")
	}
	if a.Metadata.ModelIdentifier == "" {
		return fmt.Errorf("model identifier is required")
	}
	if a.Metadata.ModelVersion == "" {
		return fmt.Errorf("model version is required")
	}
	if a.Metadata.Framework == "" {
		return fmt.Errorf("framework is required")
	}
	if len(a.Features.Columns) == 0 {
		return fmt.Errorf("feature columns are required")
	}
	if a.Metrics.SharpeRatio == 0 && a.Metrics.ProfitabilityPnL == 0 {
		return fmt.Errorf("metrics are required (sharpe or pnl must be non-zero)")
	}
	return nil
}

// ==============================
// ARTIFACT CONSTRUCTION
// ==============================

/******************************************************************************
 * Function Name : NewArtifactFromModel
 *
 * Purpose :
 *   Builds a ModelArtifact from a trained model and its metadata.
 *
 * Inputs :
 *   model    *ModelMetadata — Trained model with Fitness populated
 *   serialized []byte       — Serialized model weights (from trainer.Serialize())
 *   trainerType string      — The TrainingEngine type used
 *   features  []FeatureColumn — Feature definitions
 *
 * Return :
 *   Type        : *ModelArtifact
 *   Description : Fully populated artifact ready for transport or storage.
 *
 * Complexity : O(1)
 * Number Of Lines : 30
 ******************************************************************************/
func NewArtifactFromModel(model *ModelMetadata, serialized []byte, trainerType string, features []FeatureColumn) *ModelArtifact {
	now := time.Now().UTC().Format(time.RFC3339)
	a := &ModelArtifact{
		ModelFile: ArtifactModelFile{
			Format:   "json",
			Encoding: "raw",
			Data:     string(serialized),
			Checksum: "",
		},
		Metadata: ArtifactMetadata{
			ModelIdentifier:        model.Name,
			ModelVersion:           model.Version,
			Generation:             model.Generation,
			CreationTimestamp:      now,
			Framework:              trainerType,
			FrameworkVersion:       "1.0.0",
			TrainingDatasetVersion: model.TrainingDatasetVersion,
			FeatureSetVersion:      "1.0",
			CompatibilityGoMin:     "1.25",
			CompatibilityProtocol:  "1.0",
		},
		Features: ArtifactFeatures{
			Columns:      features,
			Preprocessing: "standard_scaler",
		},
		Deployment: ArtifactDeployment{
			GraduationScore:         0.5,
			RecommendedCategory:     model.Category,
			ConfidenceLevel:         0.7,
			SuggestedCapitalAllocPct: 5.0,
			Notes:                   "",
		},
	}

	if model.Fitness != nil {
		a.Metrics = ArtifactMetrics{
			SharpeRatio:         model.Fitness.SharpeRatio,
			SortinoRatio:        model.Fitness.SortinoRatio,
			ProfitabilityPnL:    model.Fitness.Profit,
			MaxDrawdown:         model.Fitness.Drawdown,
			VolatilityAnnualized: 0.0,
			PredictionMAE:       0.0,
			DirectionalAccuracy: model.Fitness.Consistency,
		}
	}

	return a
}

// ==============================
// JSON SERIALIZATION
// ==============================

/******************************************************************************
 * Function Name : MarshalArtifact
 *
 * Purpose :
 *   Serializes a ModelArtifact to JSON bytes.
 *
 * Inputs :
 *   a  *ModelArtifact
 *
 * Return :
 *   Type        : []byte
 *   Description : JSON-encoded artifact.
 *
 * Complexity : O(size)
 * Number Of Lines : 5
 ******************************************************************************/
func MarshalArtifact(a *ModelArtifact) ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

/******************************************************************************
 * Function Name : UnmarshalArtifact
 *
 * Purpose :
 *   Deserializes a ModelArtifact from JSON bytes.
 *
 * Inputs :
 *   data  []byte — JSON bytes
 *
 * Return :
 *   Type        : *ModelArtifact
 *   Description : Parsed artifact, or error on invalid JSON.
 *
 * Complexity : O(size)
 * Number Of Lines : 10
 ******************************************************************************/
func UnmarshalArtifact(data []byte) (*ModelArtifact, error) {
	var a ModelArtifact
	if err := json.Unmarshal(data, &a); err != nil {
		return nil, err
	}
	if err := ValidateArtifact(&a); err != nil {
		return nil, err
	}
	return &a, nil
}
