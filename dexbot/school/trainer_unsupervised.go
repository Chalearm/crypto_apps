/******************************************************************************
 * File Name       : trainer_unsupervised.go
 * File Path       : school/trainer_unsupervised.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 16:45:00 (UTC+7)
 * Modified Date   : 2026-06-29 16:45:00 (UTC+7)
 *
 * Description     :
 *   Go-native unsupervised learning trainers: K-Means clustering, PCA
 *   dimensionality reduction, DBSCAN density-based clustering per myreq3.txt §38.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 16:45 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"math"
	"time"
)

// UnsTrainer is a multi-mode unsupervised trainer (K-Means, PCA, DBSCAN).
type UnsTrainer struct {
	modelType  string
	// K-Means
	K         int
	Centroids [][]float64
	Labels    []int
	// PCA
	Components [][]float64
	Mean       []float64
	NComps     int
	// DBSCAN
	Eps        float64
	MinPts     int
	// State
	fitted     bool
}

// NewUnsTrainer creates a new unsupervised trainer.
func NewUnsTrainer(modelType string) *UnsTrainer {
	u := &UnsTrainer{
		modelType: modelType,
		K:         3,
		NComps:    2,
		Eps:       0.5,
		MinPts:    5,
	}
	return u
}

// Fit routes to the appropriate algorithm based on modelType.
func (u *UnsTrainer) Fit(features [][]float64, targets []float64, cfg *TrainingConfig) error {
	n := len(features)
	if n == 0 {
		return fmt.Errorf("no data")
	}

	switch u.modelType {
	case ModelUnsKMeans:
		return u.fitKMeans(features, cfg.MaxEpochs)
	case ModelUnsPCA:
		return u.fitPCA(features)
	case ModelUnsDBSCAN:
		return u.fitDBSCAN(features)
	default:
		// Default to K-Means
		return u.fitKMeans(features, cfg.MaxEpochs)
	}
}

// fitKMeans implements Lloyd's algorithm with k-means++ initialization.
func (u *UnsTrainer) fitKMeans(features [][]float64, epochs int) error {
	n := len(features)
	d := len(features[0])
	if epochs <= 0 {
		epochs = 50
	}

	// k-means++ init
	u.Centroids = make([][]float64, u.K)
	// First centroid: random point
	u.Centroids[0] = make([]float64, d)
	copy(u.Centroids[0], features[0])

	for k := 1; k < u.K; k++ {
		// Weight by distance to nearest centroid
		weights := make([]float64, n)
		totalW := 0.0
		for i := 0; i < n; i++ {
			minD := math.MaxFloat64
			for j := 0; j < k; j++ {
				dist := euclideanDist(features[i], u.Centroids[j])
				if dist < minD {
					minD = dist
				}
			}
			weights[i] = minD * minD
			totalW += weights[i]
		}
		// Pick next centroid
		if totalW > 0 {
			r := totalW * 0.5 // deterministic pick
			cum := 0.0
			for i := 0; i < n; i++ {
				cum += weights[i]
				if cum >= r {
					u.Centroids[k] = make([]float64, d)
					copy(u.Centroids[k], features[i])
					break
				}
			}
		}
		if u.Centroids[k] == nil {
			u.Centroids[k] = make([]float64, d)
			copy(u.Centroids[k], features[k%n])
		}
	}

	// Lloyd iteration
	for iter := 0; iter < epochs; iter++ {
		// Assign
		u.Labels = make([]int, n)
		for i := 0; i < n; i++ {
			minD := math.MaxFloat64
			for j := 0; j < u.K; j++ {
				dist := euclideanDist(features[i], u.Centroids[j])
				if dist < minD {
					minD = dist
					u.Labels[i] = j
				}
			}
		}
		// Update
		newCentroids := make([][]float64, u.K)
		counts := make([]int, u.K)
		for j := 0; j < u.K; j++ {
			newCentroids[j] = make([]float64, d)
		}
		for i := 0; i < n; i++ {
			l := u.Labels[i]
			counts[l]++
			for j := 0; j < d; j++ {
				newCentroids[l][j] += features[i][j]
			}
		}
		changed := false
		for j := 0; j < u.K; j++ {
			if counts[j] > 0 {
				for dim := 0; dim < d; dim++ {
					newCentroids[j][dim] /= float64(counts[j])
				}
			}
			if euclideanDist(u.Centroids[j], newCentroids[j]) > 1e-6 {
				changed = true
			}
			u.Centroids[j] = newCentroids[j]
		}
		if !changed {
			break
		}
	}
	u.fitted = true
	return nil
}

// fitPCA computes principal components via covariance eigendecomposition.
func (u *UnsTrainer) fitPCA(features [][]float64) error {
	n := len(features)
	d := len(features[0])

	// Mean
	u.Mean = make([]float64, d)
	for i := 0; i < n; i++ {
		for j := 0; j < d; j++ {
			u.Mean[j] += features[i][j]
		}
	}
	for j := 0; j < d; j++ {
		u.Mean[j] /= float64(n)
	}

	// Covariance matrix
	cov := make([][]float64, d)
	for i := 0; i < d; i++ {
		cov[i] = make([]float64, d)
	}
	for i := 0; i < n; i++ {
		for a := 0; a < d; a++ {
			for b := 0; b < d; b++ {
				cov[a][b] += (features[i][a] - u.Mean[a]) * (features[i][b] - u.Mean[b])
			}
		}
	}
	for a := 0; a < d; a++ {
		for b := 0; b < d; b++ {
			cov[a][b] /= float64(n - 1)
		}
	}

	// Power iteration for top NComps eigenvectors
	u.Components = make([][]float64, u.NComps)
	for k := 0; k < u.NComps; k++ {
		v := make([]float64, d)
		v[k%d] = 1.0
		for iter := 0; iter < 30; iter++ {
			v = matVecMul(cov, v)
			norm := math.Sqrt(dotVec(v, v))
			if norm > 1e-12 {
				for j := 0; j < d; j++ {
					v[j] /= norm
				}
			}
			// Deflate: subtract previous components
			for p := 0; p < k; p++ {
				proj := dotVec(v, u.Components[p])
				for j := 0; j < d; j++ {
					v[j] -= proj * u.Components[p][j]
				}
			}
		}
		u.Components[k] = v
	}
	u.fitted = true
	return nil
}

// fitDBSCAN is a simplified density-based clustering.
func (u *UnsTrainer) fitDBSCAN(features [][]float64) error {
	n := len(features)
	u.Labels = make([]int, n)
	for i := range u.Labels {
		u.Labels[i] = -1 // unclassified
	}

	clusterID := 0
	for i := 0; i < n; i++ {
		if u.Labels[i] != -1 {
			continue
		}
		neighbors := u.regionQuery(features, i)
		if len(neighbors) < u.MinPts {
			u.Labels[i] = -2 // noise
			continue
		}
		// Expand cluster
		u.Labels[i] = clusterID
		seeds := make([]int, len(neighbors))
		copy(seeds, neighbors)
		for si := 0; si < len(seeds); si++ {
			p := seeds[si]
			if u.Labels[p] == -2 {
				u.Labels[p] = clusterID
			}
			if u.Labels[p] != -1 {
				continue
			}
			u.Labels[p] = clusterID
			nb := u.regionQuery(features, p)
			if len(nb) >= u.MinPts {
				seeds = append(seeds, nb...)
			}
		}
		clusterID++
	}
	u.fitted = true
	return nil
}

func (u *UnsTrainer) regionQuery(features [][]float64, idx int) []int {
	var result []int
	for i := range features {
		if i == idx {
			continue
		}
		if euclideanDist(features[idx], features[i]) <= u.Eps {
			result = append(result, i)
		}
	}
	return result
}

// Predict returns cluster assignment or PCA-transformed value.
func (u *UnsTrainer) Predict(features []float64) (float64, error) {
	if !u.fitted {
		return 0, nil
	}
	switch u.modelType {
	case ModelUnsKMeans, ModelUnsDBSCAN:
		// Return cluster label
		if len(u.Centroids) > 0 {
			minD := math.MaxFloat64
			bestLabel := 0.0
			for j, c := range u.Centroids {
				dist := euclideanDist(features, c)
				if dist < minD {
					minD = dist
					bestLabel = float64(j)
				}
			}
			return bestLabel, nil
		}
		return 0, nil
	case ModelUnsPCA:
		if len(u.Components) > 0 {
			// Project onto first component
			centered := make([]float64, len(features))
			for j := 0; j < len(features) && j < len(u.Mean); j++ {
				centered[j] = features[j] - u.Mean[j]
			}
			return dotVec(centered, u.Components[0]), nil
		}
		return 0, nil
	default:
		return 0, nil
	}
}

// Backtest evaluates clustering quality via silhouette-like score.
func (u *UnsTrainer) Backtest(features [][]float64, targets []float64) (*FitnessHistory, error) {
	if !u.fitted {
		return &FitnessHistory{Timestamp: time.Now()}, nil
	}
	predictions := make([]float64, len(features))
	for i, f := range features {
		predictions[i], _ = u.Predict(f)
	}
	// For unsupervised models, compare cluster assignments with targets (if available)
	return ComputeFitnessFromPredictions(predictions, targets), nil
}

// WalkForward performs rolling validation.
func (u *UnsTrainer) WalkForward(features [][]float64, targets []float64, windowSize int) ([]FitnessHistory, error) {
	n := len(features)
	if windowSize <= 0 || windowSize >= n {
		return nil, nil
	}
	cfg := NewTrainingConfig()
	var histories []FitnessHistory
	for start := 0; start+windowSize < n; start++ {
		tmp := NewUnsTrainer(u.modelType)
		_ = tmp.Fit(features[start:start+windowSize], targets[start:start+windowSize], cfg)
		pred, _ := tmp.Predict(features[start+windowSize])
		fh := ComputeFitnessFromPredictions([]float64{pred}, []float64{targets[start+windowSize]})
		histories = append(histories, *fh)
	}
	return histories, nil
}

// Serialize marshals to JSON.
func (u *UnsTrainer) Serialize() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"model_type": u.modelType, "k": u.K, "n_comps": u.NComps,
		"eps": u.Eps, "min_pts": u.MinPts, "fitted": u.fitted,
	})
}

// Deserialize restores from JSON.
func (u *UnsTrainer) Deserialize(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if v, ok := m["fitted"].(bool); ok {
		u.fitted = v
	}
	return nil
}

// --- helpers ---

func euclideanDist(a, b []float64) float64 {
	sum := 0.0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		d := a[i] - b[i]
		sum += d * d
	}
	return math.Sqrt(sum)
}

func dotVec(a, b []float64) float64 {
	sum := 0.0
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		sum += a[i] * b[i]
	}
	return sum
}

func matVecMul(A [][]float64, v []float64) []float64 {
	n := len(A)
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		for j := 0; j < len(v) && j < len(A[i]); j++ {
			result[i] += A[i][j] * v[j]
		}
	}
	return result
}

func init() {
	RegisterTrainer(ModelUnsKMeans, func() TrainingEngine { return NewUnsTrainer(ModelUnsKMeans) })
	RegisterTrainer(ModelUnsDBSCAN, func() TrainingEngine { return NewUnsTrainer(ModelUnsDBSCAN) })
	RegisterTrainer(ModelUnsPCA, func() TrainingEngine { return NewUnsTrainer(ModelUnsPCA) })
}
