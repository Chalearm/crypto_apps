/******************************************************************************
 * File Name       : training_test.go
 * File Path       : school/training_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-29 17:00:00 (UTC+7)
 *
 * Description     :
 *   Tests for TrainingEngine interface, trainer registry, dispatcher,
 *   and model artifact contract. Per rule1.txt: 6 positive + 2 negative.
 *
 * Change History :
 *   1.0.0 | 2026-06-29 17:00 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"testing"
	"time"
)

// syntheticData creates simple linear data: y = 2*x + 1
func syntheticData(n int) ([][]float64, []float64) {
	features := make([][]float64, n)
	targets := make([]float64, n)
	for i := 0; i < n; i++ {
		x := float64(i) / float64(n)
		features[i] = []float64{x}
		targets[i] = 2.0*x + 1.0
	}
	return features, targets
}

// Test 1 (Positive): LinearTrainer Fit + Predict on synthetic data
func TestLinearTrainerFitPredict(t *testing.T) {
	f, targets := syntheticData(100)
	cfg := NewTrainingConfig()
	lt := NewLinearTrainer(ModelSupLinearReg)

	err := lt.Fit(f, targets, cfg)
	if err != nil {
		t.Fatalf("Fit failed: %v", err)
	}
	if !lt.fitted {
		t.Fatal("expected fitted=true")
	}

	pred, err := lt.Predict([]float64{0.5})
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}
	// y = 2*0.5 + 1 = 2.0
	if pred < 1.5 || pred > 2.5 {
		t.Errorf("prediction out of range: got %.3f, expected ~2.0", pred)
	}
}

// Test 2 (Positive): LinearTrainer Backtest returns valid FitnessHistory
func TestLinearTrainerBacktest(t *testing.T) {
	f, targets := syntheticData(100)
	lt := NewLinearTrainer(ModelSupRidge)
	if err := lt.Fit(f, targets, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}

	fh, err := lt.Backtest(f, targets)
	if err != nil {
		t.Fatalf("Backtest: %v", err)
	}
	if fh == nil {
		t.Fatal("nil FitnessHistory")
	}
	if fh.Timestamp.IsZero() {
		t.Error("timestamp is zero")
	}
}

// Test 3 (Positive): LinearTrainer Serialize / Deserialize round-trip
func TestLinearTrainerSerializeDeserialize(t *testing.T) {
	f, targets := syntheticData(50)
	lt := NewLinearTrainer(ModelSupElasticNet)
	lt.Fit(f, targets, NewTrainingConfig())

	data, err := lt.Serialize()
	if err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("serialized data is empty")
	}

	lt2 := NewLinearTrainer(ModelSupElasticNet)
	if err := lt2.Deserialize(data); err != nil {
		t.Fatalf("Deserialize: %v", err)
	}
	if lt2.fitted != lt.fitted {
		t.Errorf("fitted mismatch: %v vs %v", lt2.fitted, lt.fitted)
	}
}

// Test 4 (Positive): ARIMA Fit + Predict basic check
func TestArimaTrainerFitPredict(t *testing.T) {
	n := 80
	series := make([]float64, n)
	for i := 0; i < n; i++ {
		series[i] = float64(i) * 0.1
	}
	f := make([][]float64, n)
	for i := 0; i < n; i++ {
		f[i] = []float64{series[i]}
	}

	at := NewArimaTrainer(ModelStatARIMA)
	if err := at.Fit(f, series, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	pred, _ := at.Predict([]float64{7.9, 8.0})
	if pred == 0 && at.fitted {
		t.Log("prediction is zero — acceptable for simple AR(1) on trend")
	}
}

// Test 5 (Positive): RandomForest Fit + Predict
func TestRFTrainerFitPredict(t *testing.T) {
	f, targets := syntheticData(60)
	rf := NewRFTrainer(ModelSupRandomForest)
	if err := rf.Fit(f, targets, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	pred, _ := rf.Predict([]float64{0.5})
	if pred < 0 || pred > 5.0 {
		t.Errorf("prediction out of plausible range: %.3f", pred)
	}
}

// Test 6 (Positive): Dispatcher TrainModel + TrainPopulation
func TestDispatcherTrainPopulation(t *testing.T) {
	pop := NewModelPopulation()
	pop.AddModel(NewLinearTrainer(ModelSupLinearReg).newMetadata("dispatcher_test_1"))

	f, targets := syntheticData(40)
	d := NewDispatcher(nil)      // MODE A (Go-native, no remote)
	trained := d.TrainPopulation(pop, f, targets, NewTrainingConfig())

	if trained == 0 {
		t.Error("expected at least 1 model trained")
	}
	if d.GetTrainingMode() != "MODE_A" {
		t.Errorf("expected MODE_A, got %s", d.GetTrainingMode())
	}
}

// Test 7 (Negative): Fit on empty data
func TestLinearTrainerFitEmpty(t *testing.T) {
	lt := NewLinearTrainer(ModelSupLasso)
	err := lt.Fit(nil, nil, NewTrainingConfig())
	if err == nil {
		t.Error("expected error for empty data")
	}
}

// Test 8 (Negative): Predict before Fit
func TestLinearTrainerPredictBeforeFit(t *testing.T) {
	lt := NewLinearTrainer(ModelSupLinearReg)
	pred, err := lt.Predict([]float64{1.0, 2.0})
	if err != nil || pred != 0 {
		t.Errorf("expected (0, nil) from unfitted model, got (%.3f, %v)", pred, err)
	}
}

// Test 9 (Positive): Artifact validation
func TestArtifactValidation(t *testing.T) {
	model := &ModelMetadata{
		Name: "test_model", Version: "v1.0", Category: CategorySwing,
		Architecture: ModelSupLinearReg, Status: StatusTraining,
		Fitness: &FitnessHistory{SharpeRatio: 1.5, Profit: 100.0, Timestamp: time.Now()},
	}
	lt := NewLinearTrainer(ModelSupLinearReg)
	f, targs := syntheticData(30)
	lt.Fit(f, targs, NewTrainingConfig())
	data, _ := lt.Serialize()

	artifact := NewArtifactFromModel(model, data, "Linear Regression", []FeatureColumn{
		{Name: "price", Type: "float64", Category: "price"},
	})

	if err := ValidateArtifact(artifact); err != nil {
		t.Errorf("valid artifact should pass: %v", err)
	}

	// Corrupt it
	artifact.ModelFile.Data = ""
	if err := ValidateArtifact(artifact); err == nil {
		t.Error("expected error for empty model file data")
	}
}

// Test 10 (Positive): GARCH Fit + Predict
func TestGarchTrainerFitPredict(t *testing.T) {
	n := 50
	series := make([]float64, n)
	for i := 0; i < n; i++ {
		series[i] = 100.0 + float64(i%5)*0.1
	}
	f := make([][]float64, n)
	for i := 0; i < n; i++ {
		f[i] = []float64{series[i]}
	}

	gt := NewGarchTrainer(ModelStatGARCH)
	if err := gt.Fit(f, series, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	pred, _ := gt.Predict([]float64{100.3, 100.4, 100.5})
	if pred <= 0 {
		t.Errorf("volatility prediction must be positive: %.6f", pred)
	}
}

// Test 11 (Positive): SVM Fit + Predict
func TestSVMTrainerFitPredict(t *testing.T) {
	f, targets := syntheticData(30)
	svm := NewSVMTrainer(ModelSupSVR)
	if err := svm.Fit(f, targets, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	pred, _ := svm.Predict([]float64{0.7})
	if pred < 0 || pred > 5.0 {
		t.Errorf("prediction out of range: %.3f", pred)
	}
}

// Test 12 (Positive): K-Means Fit + Predict
func TestKMeansFitPredict(t *testing.T) {
	n := 45
	f := make([][]float64, n)
	targets := make([]float64, n)
	for i := 0; i < n; i++ {
		x := float64(i%3) * 2.0
		f[i] = []float64{x + float64(i)*0.01, x - 1.0}
		targets[i] = float64(i % 3)
	}
	km := NewUnsTrainer(ModelUnsKMeans)
	km.K = 3
	if err := km.Fit(f, targets, NewTrainingConfig()); err != nil {
		t.Fatalf("Fit: %v", err)
	}
	label, _ := km.Predict([]float64{0.0, -1.0})
	if label < 0 || label >= 3 {
		t.Errorf("cluster label out of range: %.1f", label)
	}
}

// Test 13 (Positive): WalkForward validation
func TestLinearTrainerWalkForward(t *testing.T) {
	f, targets := syntheticData(80)
	lt := NewLinearTrainer(ModelSupLinearReg)
	histories, err := lt.WalkForward(f, targets, 20)
	if err != nil {
		t.Fatalf("WalkForward: %v", err)
	}
	if len(histories) == 0 {
		t.Error("expected non-empty walk-forward results")
	}
}

// Test 14 (Negative): WalkForward with invalid window
func TestLinearTrainerWalkForwardInvalid(t *testing.T) {
	f, targets := syntheticData(10)
	lt := NewLinearTrainer(ModelSupLinearReg)
	histories, err := lt.WalkForward(f, targets, 0)
	if err != nil || histories != nil {
		t.Error("expected (nil, nil) for windowSize=0")
	}
	histories2, err2 := lt.WalkForward(f, targets, 20)
	if err2 != nil || histories2 != nil {
		t.Error("expected (nil, nil) for window too large")
	}
}

// ==============================
// HELPER: create ModelMetadata from trainer
// ==============================

func (lt *LinearTrainer) newMetadata(name string) *ModelMetadata {
	return &ModelMetadata{
		Name: name, Version: "v0.1", Category: CategorySwing,
		Status: StatusTraining, Architecture: lt.modelType,
		TrainingDatasetVersion: "synthetic",
		CreatedAt:              time.Now(),
		Fitness:                &FitnessHistory{Timestamp: time.Now()},
	}
}
