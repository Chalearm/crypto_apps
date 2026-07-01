/******************************************************************************
 * File Name       : trainer_randomforest.go
 * File Path       : school/trainer_randomforest.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:30:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:30:00 (UTC+7)
 *
 * Description     :
 *   Go-native Random Forest / Extra Trees ensemble trainer per myreq3.txt §35.
 *   Implements bootstrap aggregating of decision trees with feature
 *   subsampling. Extra Trees uses random thresholds instead of optimal splits.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:30 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// decisionTree is a single tree in the forest.
type decisionTree struct {
	SplitFeature int
	SplitValue   float64
	Left         *decisionTree
	Right        *decisionTree
	LeafValue    float64
	IsLeaf       bool
}

// RFTrainer is a Go-native RandomForest/ExtraTrees ensemble.
type RFTrainer struct {
	modelType    string
	Trees        []*decisionTree
	NumTrees     int
	MaxDepth     int
	MinSamples   int
	FeatureRatio float64
	IsExtraTrees bool
	fitted       bool
}

// NewRFTrainer creates a new RF/ET trainer.
func NewRFTrainer(modelType string) *RFTrainer {
	rf := &RFTrainer{
		modelType:    modelType,
		NumTrees:     10,
		MaxDepth:     8,
		MinSamples:   5,
		FeatureRatio: 0.7,
		IsExtraTrees: modelType == ModelSupExtraTrees,
	}
	return rf
}

// Fit builds an ensemble of decision trees using bootstrap samples.
func (rf *RFTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	n := len(features)
	if n == 0 {
		return fmt.Errorf("no training data")
	}
	if n != len(targets) {
		return fmt.Errorf("dimension mismatch")
	}
	if rf.MaxDepth <= 0 {
		rf.MaxDepth = 8
	}

	rf.Trees = make([]*decisionTree, rf.NumTrees)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for t := 0; t < rf.NumTrees; t++ {
		// Bootstrap sample
		bootF := make([][]float64, n)
		bootT := make([]float64, n)
		for i := 0; i < n; i++ {
			idx := rng.Intn(n)
			bootF[i] = features[idx]
			bootT[i] = targets[idx]
		}
		rf.Trees[t] = rf.buildTree(bootF, bootT, 0, rng)
	}
	rf.fitted = true
	return nil
}

// buildTree recursively constructs a decision tree.
func (rf *RFTrainer) buildTree(features [][]float64, targets []float64, depth int, rng *rand.Rand) *decisionTree {
	n := len(features)
	if n < rf.MinSamples || depth >= rf.MaxDepth {
		// Leaf: mean target
		mean := 0.0
		for _, t := range targets {
			mean += t
		}
		return &decisionTree{IsLeaf: true, LeafValue: mean / float64(n)}
	}

	nFeat := len(features[0])
	nFeatSub := int(float64(nFeat) * rf.FeatureRatio)
	if nFeatSub < 1 {
		nFeatSub = 1
	}
	if nFeatSub > nFeat {
		nFeatSub = nFeat
	}

	// Feature subsampling
	featPool := make([]int, nFeatSub)
	perm := rng.Perm(nFeat)
	for i := 0; i < nFeatSub; i++ {
		featPool[i] = perm[i]
	}

	bestFeat := -1
	bestVal := 0.0
	bestVar := math.MaxFloat64

	for _, fi := range featPool {
		var splitVal float64
		if rf.IsExtraTrees {
			// Random threshold between min and max
			minV, maxV := features[0][fi], features[0][fi]
			for _, f := range features {
				if f[fi] < minV {
					minV = f[fi]
				}
				if f[fi] > maxV {
					maxV = f[fi]
				}
			}
			splitVal = minV + rng.Float64()*(maxV-minV)
		} else {
			// Random sample value
			splitVal = features[rng.Intn(n)][fi]
		}

		// Variance reduction
		var leftSum, rightSum float64
		leftN, rightN := 0, 0
		for i, f := range features {
			if f[fi] <= splitVal {
				leftSum += targets[i]
				leftN++
			} else {
				rightSum += targets[i]
				rightN++
			}
		}
		if leftN < rf.MinSamples || rightN < rf.MinSamples {
			continue
		}
		leftMean := leftSum / float64(leftN)
		rightMean := rightSum / float64(rightN)
		var leftVar, rightVar float64
		for i, f := range features {
			if f[fi] <= splitVal {
				d := targets[i] - leftMean
				leftVar += d * d
			} else {
				d := targets[i] - rightMean
				rightVar += d * d
			}
		}
		totalVar := leftVar + rightVar
		if totalVar < bestVar {
			bestVar = totalVar
			bestFeat = fi
			bestVal = splitVal
		}
	}

	if bestFeat < 0 {
		mean := 0.0
		for _, t := range targets {
			mean += t
		}
		return &decisionTree{IsLeaf: true, LeafValue: mean / float64(n)}
	}

	// Split
	var leftF, rightF [][]float64
	var leftT, rightT []float64
	for i, f := range features {
		if f[bestFeat] <= bestVal {
			leftF = append(leftF, f)
			leftT = append(leftT, targets[i])
		} else {
			rightF = append(rightF, f)
			rightT = append(rightT, targets[i])
		}
	}

	return &decisionTree{
		SplitFeature: bestFeat,
		SplitValue:   bestVal,
		Left:         rf.buildTree(leftF, leftT, depth+1, rng),
		Right:        rf.buildTree(rightF, rightT, depth+1, rng),
	}
}

// Predict traverses all trees and averages their predictions.
func (rf *RFTrainer) Predict(features []float64) (float64, error) {
	if !rf.fitted || len(rf.Trees) == 0 {
		return 0, nil
	}
	sum := 0.0
	for _, tree := range rf.Trees {
		sum += rf.traverse(tree, features)
	}
	return sum / float64(len(rf.Trees)), nil
}

func (rf *RFTrainer) traverse(node *decisionTree, features []float64) float64 {
	if node.IsLeaf {
		return node.LeafValue
	}
	if node.SplitFeature < len(features) && features[node.SplitFeature] <= node.SplitValue {
		return rf.traverse(node.Left, features)
	}
	return rf.traverse(node.Right, features)
}

// Backtest evaluates on historical data.
func (rf *RFTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !rf.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(features))
	for i, f := range features {
		predictions[i], _ = rf.Predict(f)
	}
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// WalkForward performs rolling validation.
func (rf *RFTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}
	cfg := NewTrainingConfig()
	var histories []FitnessHistory
	for start := 0; start+windowSize < n; start++ {
		tmp := NewRFTrainer(rf.modelType)
		_ = tmp.Fit(features[start:start+windowSize], targets[start:start+windowSize], cfg)
		pred, _ := tmp.Predict(features[start+windowSize])
		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{targets[start+windowSize]})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// Serialize marshals to JSON.
func (rf *RFTrainer) Serialize() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model_type": rf.modelType, "num_trees": rf.NumTrees,
		"max_depth": rf.MaxDepth, "fitted": rf.fitted,
	})
}

// Deserialize restores from JSON.
func (rf *RFTrainer) Deserialize(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["num_trees"].(float64); ok {
		rf.NumTrees = int(v)
	}
	if v, ok := m["fitted"].(bool); ok {
		rf.fitted = v
	}
	return nil
}

func init() {
	RegisterTrainer(ModelSupRandomForest, func() TrainingEngine { return NewRFTrainer(ModelSupRandomForest) })
	RegisterTrainer(ModelSupExtraTrees, func() TrainingEngine { return NewRFTrainer(ModelSupExtraTrees) })
}
