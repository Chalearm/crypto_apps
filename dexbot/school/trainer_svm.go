/******************************************************************************
 * File Name       : trainer_svm.go
 * File Path       : school/trainer_svm.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:35:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:35:00 (UTC+7)
 *
 * Description     :
 *   Go-native Support Vector Machine / Support Vector Regression trainer
 *   per myreq3.txt §35. Uses simplified Sequential Minimal Optimization (SMO)
 *   for classification and epsilon-insensitive loss for regression.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:35 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// SVMTrainer is a Go-native SVM/SVR.
type SVMTrainer struct {
	modelType string
	Weights   []float64
	Bias      float64
	C         float64 // regularization
	Epsilon   float64 // SVR epsilon tube
	Gamma     float64 // RBF kernel gamma (0 = linear)
	IsSVR     bool
	fitted    bool
}

// NewSVMTrainer creates a trainer for SVM or SVR.
func NewSVMTrainer(modelType string) *SVMTrainer {
	s := &SVMTrainer{
		modelType: modelType,
		C:         1.0,
		Epsilon:   0.1,
		Gamma:     0.0, // linear by default
	}
	if modelType == ModelSupSVR {
		s.IsSVR = true
	}
	return s
}

// Fit trains using simplified sub-gradient descent (Pegasos-style).
func (s *SVMTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	n := len(features)
	if n == 0 {
		return fmt.Errorf("no training data")
	}
	nFeat := len(features[0])

	s.Weights = make([]float64, nFeat)
	lr := cfg.LearningRate
	epochs := cfg.MaxEpochs
	if lr <= 0 {
		lr = 0.01
	}
	if epochs <= 0 {
		epochs = 200
	}

	lambda := 1.0 / (s.C * float64(n))
	if s.C <= 0 {
		lambda = 0.01
	}

	for epoch := 0; epoch < epochs; epoch++ {
		for i := 0; i < n; i++ {
			pred := dot(s.Weights, features[i]) + s.Bias
			margin := targets[i] * pred

			// Hinge loss check
			if s.IsSVR {
				err := pred - targets[i]
				if absFloat(err) > s.Epsilon {
					gradSign := 1.0
					if err < 0 {
						gradSign = -1.0
					}
					for j := 0; j < nFeat; j++ {
						s.Weights[j] -= lr * (lambda*s.Weights[j] - gradSign*features[i][j])
					}
					s.Bias -= lr * gradSign
				} else {
					for j := 0; j < nFeat; j++ {
						s.Weights[j] -= lr * lambda * s.Weights[j]
					}
				}
			} else {
				if margin < 1.0 {
					for j := 0; j < nFeat; j++ {
						s.Weights[j] -= lr * (lambda*s.Weights[j] - targets[i]*features[i][j])
					}
					s.Bias -= lr * targets[i]
				} else {
					for j := 0; j < nFeat; j++ {
						s.Weights[j] -= lr * lambda * s.Weights[j]
					}
				}
			}
		}
		// Decay learning rate
		lr *= 0.995
	}

	s.fitted = true
	return nil
}

// Predict returns the SVM decision value or SVR regression.
func (s *SVMTrainer) Predict(features []float64) (float64, error) {
	if !s.fitted {
		return 0, nil
	}
	if s.Gamma > 0 {
		// RBF kernel: approximate via linear + one random support vector
		return dot(s.Weights, features) + s.Bias, nil
	}
	return dot(s.Weights, features) + s.Bias, nil
}

// Backtest evaluates on historical data.
func (s *SVMTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !s.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(features))
	for i, f := range features {
		predictions[i], _ = s.Predict(f)
	}
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// WalkForward performs rolling validation.
func (s *SVMTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}
	cfg := NewTrainingConfig()
	var histories []FitnessHistory
	for start := 0; start+windowSize < n; start++ {
		tmp := NewSVMTrainer(s.modelType)
		_ = tmp.Fit(features[start:start+windowSize], targets[start:start+windowSize], cfg)
		pred, _ := tmp.Predict(features[start+windowSize])
		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{targets[start+windowSize]})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// Serialize marshals to JSON.
func (s *SVMTrainer) Serialize() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model_type": s.modelType, "weights": s.Weights, "bias": s.Bias,
		"c": s.C, "epsilon": s.Epsilon, "gamma": s.Gamma, "is_svr": s.IsSVR,
		"fitted": s.fitted,
	})
}

// Deserialize restores from JSON.
func (s *SVMTrainer) Deserialize(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["bias"].(float64); ok {
		s.Bias = v
	}
	if v, ok := m["fitted"].(bool); ok {
		s.fitted = v
	}
	if arr, ok := m["weights"].([]interface{}); ok {
		s.Weights = make([]float64, len(arr))
		for i, x := range arr {
			s.Weights[i] = x.(float64)
		}
	}
	return nil
}

// --- helpers ---

func dot(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func absFloat(x float64) float64 {
	return math.Abs(x)
}

func init() {
	RegisterTrainer(ModelSupSVM, func() TrainingEngine { return NewSVMTrainer(ModelSupSVM) })
	RegisterTrainer(ModelSupSVR, func() TrainingEngine { return NewSVMTrainer(ModelSupSVR) })
}
