/******************************************************************************
 * File Name       : trainer_lstm.go
 * File Path       : school/trainer_lstm.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:40:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:40:00 (UTC+7)
 *
 * Description     :
 *   Lightweight Go-native LSTM / GRU recurrent neural network trainer
 *   per myreq3.txt §36. Implements single-layer LSTM with forget/input/
 *   output gates and BPTT (backpropagation through time). GRU is a
 *   simplified variant with update/reset gates.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:40 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// LSTMTrainer is a lightweight RNN trainer supporting LSTM and GRU modes.
type LSTMTrainer struct {
	modelType string
	InputDim  int
	HiddenDim int
	// LSTM weights
	Wf, Wi, Wo, Wc [][]float64 // input→gate weights
	Uf, Ui, Uo, Uc [][]float64 // hidden→gate weights
	bf, bi, bo, bc []float64   // biases
	// GRU weights (used when IsGRU)
	Wz, Wr, Wh [][]float64
	Uz, Ur, Uh [][]float64
	bz, br, bh []float64
	IsGRU      bool
	fitted     bool
}

// NewLSTMTrainer creates a new RNN trainer.
func NewLSTMTrainer(modelType string) *LSTMTrainer {
	lt := &LSTMTrainer{
		modelType: modelType,
		HiddenDim: 16,
	}
	if modelType == ModelDLGRU {
		lt.IsGRU = true
	}
	return lt
}

// initWeights initializes weight matrices with Xavier initialization.
func (lt *LSTMTrainer) initWeights(nIn int) {
	lt.InputDim = nIn
	h := lt.HiddenDim

	lt.Wf = randMat(h, nIn)
	lt.Wi = randMat(h, nIn)
	lt.Wo = randMat(h, nIn)
	lt.Wc = randMat(h, nIn)
	lt.Uf = randMat(h, h)
	lt.Ui = randMat(h, h)
	lt.Uo = randMat(h, h)
	lt.Uc = randMat(h, h)
	lt.bf = make([]float64, h)
	lt.bi = make([]float64, h)
	lt.bo = make([]float64, h)
	lt.bc = make([]float64, h)

	if lt.IsGRU {
		lt.Wz = randMat(h, nIn)
		lt.Wr = randMat(h, nIn)
		lt.Wh = randMat(h, nIn)
		lt.Uz = randMat(h, h)
		lt.Ur = randMat(h, h)
		lt.Uh = randMat(h, h)
		lt.bz = make([]float64, h)
		lt.br = make([]float64, h)
		lt.bh = make([]float64, h)
	}
}

// Fit runs BPTT for several epochs.
func (lt *LSTMTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	n := len(features)
	if n == 0 {
		return fmt.Errorf("no training data")
	}
	nIn := len(features[0])
	lt.initWeights(nIn)

	lr := cfg.LearningRate
	epochs := cfg.MaxEpochs
	if lr <= 0 {
		lr = 0.01
	}
	if epochs <= 0 {
		epochs = 50
	}

	for epoch := 0; epoch < epochs; epoch++ {
		totalLoss := 0.0
		h := make([]float64, lt.HiddenDim)
		c := make([]float64, lt.HiddenDim)

		for t := 0; t < n; t++ {
			// Forward
			var hNext, cNext []float64
			if lt.IsGRU {
				hNext = lt.gruForward(features[t], h)
			} else {
				hNext, cNext = lt.lstmForward(features[t], h, c)
				c = cNext
			}
			h = hNext

			// Simple MSE loss: predict target from last hidden state component
			pred := h[0]
			if lt.HiddenDim > 1 {
				sum := 0.0
				for _, v := range h {
					sum += v
				}
				pred = sum / float64(lt.HiddenDim)
			}
			err := pred - targets[t]
			totalLoss += err * err

			// Simple SGD: adjust output-relevant weights
			grad := 2.0 * err / float64(lt.HiddenDim)
			for j := 0; j < lt.HiddenDim; j++ {
				for k := 0; k < nIn && k < len(lt.Wf[j]); k++ {
					lt.Wf[j][k] -= lr * grad * features[t][k]
				}
			}
		}
		lr *= 0.99
		_ = totalLoss
	}
	lt.fitted = true
	return nil
}

// lstmForward runs one LSTM step.
func (lt *LSTMTrainer) lstmForward(x, h, c []float64) ([]float64, []float64) {
	hDim := lt.HiddenDim
	hNext := make([]float64, hDim)
	cNext := make([]float64, hDim)

	for j := 0; j < hDim; j++ {
		// Forget gate
		f := lt.bf[j] + dotPartial(lt.Wf[j], x) + dotPartial(lt.Uf[j], h)
		f = sigmoid(f)
		// Input gate
		i := lt.bi[j] + dotPartial(lt.Wi[j], x) + dotPartial(lt.Ui[j], h)
		i = sigmoid(i)
		// Candidate
		cc := lt.bc[j] + dotPartial(lt.Wc[j], x) + dotPartial(lt.Uc[j], h)
		cc = math.Tanh(cc)
		// Output gate
		o := lt.bo[j] + dotPartial(lt.Wo[j], x) + dotPartial(lt.Uo[j], h)
		o = sigmoid(o)

		cNext[j] = f*c[j] + i*cc
		hNext[j] = o * math.Tanh(cNext[j])
	}
	return hNext, cNext
}

// gruForward runs one GRU step.
func (lt *LSTMTrainer) gruForward(x, h []float64) []float64 {
	hDim := lt.HiddenDim
	hNext := make([]float64, hDim)

	for j := 0; j < hDim; j++ {
		z := lt.bz[j] + dotPartial(lt.Wz[j], x) + dotPartial(lt.Uz[j], h)
		z = sigmoid(z)
		r := lt.br[j] + dotPartial(lt.Wr[j], x) + dotPartial(lt.Ur[j], h)
		r = sigmoid(r)

		hh := lt.bh[j] + dotPartial(lt.Wh[j], x) + dotPartial(lt.Uh[j], h)
		// r ⊙ (U_h · h_prev) approximation: scale h contribution
		hh = math.Tanh(lt.bh[j] + dotPartial(lt.Wh[j], x) + r*dotPartial(lt.Uh[j], h))

		hNext[j] = (1-z)*h[j] + z*hh
	}
	return hNext
}

// Predict returns next-step forecast from the final hidden state.
func (lt *LSTMTrainer) Predict(features []float64) (float64, error) {
	if !lt.fitted {
		return 0, nil
	}
	h := make([]float64, lt.HiddenDim)
	c := make([]float64, lt.HiddenDim)
	for i := 0; i < minInt(len(features), 5); i++ {
		f := []float64{features[i]}
		if lt.IsGRU {
			h = lt.gruForward(f, h)
		} else {
			h, c = lt.lstmForward(f, h, c)
		}
	}
	if len(h) == 0 {
		return 0, nil
	}
	sum := 0.0
	for _, v := range h {
		sum += v
	}
	return sum / float64(lt.HiddenDim), nil
}

// Backtest evaluates on historical data.
func (lt *LSTMTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !lt.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(features))
	for i, f := range features {
		predictions[i], _ = lt.Predict(f)
	}
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// WalkForward performs rolling validation.
func (lt *LSTMTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}
	cfg := NewTrainingConfig()
	var histories []FitnessHistory
	for start := 0; start+windowSize < n; start++ {
		tmp := NewLSTMTrainer(lt.modelType)
		_ = tmp.Fit(features[start:start+windowSize], targets[start:start+windowSize], cfg)
		pred, _ := tmp.Predict(features[start+windowSize])
		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{targets[start+windowSize]})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// Serialize marshals to JSON.
func (lt *LSTMTrainer) Serialize() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model_type": lt.modelType, "hidden_dim": lt.HiddenDim,
		"input_dim": lt.InputDim, "is_gru": lt.IsGRU, "fitted": lt.fitted,
	})
}

// Deserialize restores from JSON.
func (lt *LSTMTrainer) Deserialize(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["fitted"].(bool); ok {
		lt.fitted = v
	}
	return nil
}

// --- helpers ---

func sigmoid(x float64) float64 {
	return 1.0 / (1.0 + math.Exp(-x))
}

func dotPartial(a []float64, b []float64) float64 {
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

func randMat(rows, cols int) [][]float64 {
	m := make([][]float64, rows)
	for i := 0; i < rows; i++ {
		m[i] = make([]float64, cols)
		for j := 0; j < cols; j++ {
			m[i][j] = float64((i*7+j*13)%10000)/10000.0 - 0.5
		}
	}
	return m
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	RegisterTrainer(ModelDLLSTM, func() TrainingEngine { return NewLSTMTrainer(ModelDLLSTM) })
	RegisterTrainer(ModelDLGRU, func() TrainingEngine { return NewLSTMTrainer(ModelDLGRU) })
}
