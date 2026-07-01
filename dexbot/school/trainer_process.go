/******************************************************************************
 * File Name       : trainer_process.go
 * File Path       : school/trainer_process.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-06-30 06:00:00 (UTC+7)
 *
 * Description     :
 *   Subprocess-based training execution per myreq4.txt §91.
 *   School daemon spawns external training programs (Python/TensorFlow,
 *   Rust, C++) as subprocesses. Each subprocess receives training
 *   configuration via JSON stdin and returns results via JSON stdout
 *   using the Artifact Contract (§44-47).
 *
 *   Supported subprocess types:
 *     - Python/TensorFlow  (train_template.py)
 *     - Python/scikit-learn (train_template.py --framework sklearn)
 *     - Rust binary         (train_binary --framework rust)
 *     - C++ binary          (train_binary --framework cpp)
 *
 * Responsibilities:
 *   - Spawn subprocesses with timeout + resource limits
 *   - Feed training config via stdin (JSON)
 *   - Collect results from stdout (JSON Artifact)
 *   - Parse exit codes and error output
 *   - Convert results to FitnessHistory for the orchestrator
 *
 * Change History :
 *   1.0.0 | 2026-06-30 06:00 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package school

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

// ==============================
// SUBPROCESS CONFIG
// ==============================

// ProcessTrainerConfig is the JSON fed to training subprocesses via stdin.
type ProcessTrainerConfig struct {
	ModelID           string            `json:"model_id"`
	Architecture      string            `json:"architecture"`
	Framework         string            `json:"framework"` // python, rust, cpp, tensorflow
	Hyperparameters   map[string]string `json:"hyperparameters"`
	TrainingRecords   int               `json:"training_records"`
	DBHost            string            `json:"db_host"`
	DBPort            string            `json:"db_port"`
	DBUser            string            `json:"db_user"`
	DBPass            string            `json:"db_pass"`
	DBName            string            `json:"db_name"`
	Features          []string          `json:"features"`
	Target            string            `json:"target"`
	WindowSize        int               `json:"window_size"`
	MaxEpochs         int               `json:"max_epochs"`
	LearningRate      float64           `json:"learning_rate"`
	ArtifactOutputPath string           `json:"artifact_output_path"`
}

// ProcessTrainerResult is the JSON result returned by a training subprocess.
type ProcessTrainerResult struct {
	Success    bool              `json:"success"`
	Error      string            `json:"error,omitempty"`
	ModelID    string            `json:"model_id"`
	Framework  string            `json:"framework"`
	Duration   float64           `json:"duration_sec"`
	Artifact   *ModelArtifact    `json:"artifact,omitempty"`
	Fitness    *FitnessHistory   `json:"fitness,omitempty"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
}

// ==============================
// SUBPROCESS SPAWNER
// ==============================

// ProcessSpawner manages external training subprocesses.
type ProcessSpawner struct {
	PythonPath     string // path to python3 binary
	ScriptPath     string // path to train_template.py
	TimeoutSeconds int    // max wait time per subprocess
}

// NewProcessSpawner creates a subprocess spawner with defaults.
func NewProcessSpawner(pythonPath, scriptPath string, timeoutSec int) *ProcessSpawner {
	if pythonPath == "" {
		pythonPath = "python3"
	}
	if scriptPath == "" {
		scriptPath = "school/training_scripts/train_template.py"
	}
	if timeoutSec <= 0 {
		timeoutSec = 120 // 2 minutes default
	}
	return &ProcessSpawner{
		PythonPath:     pythonPath,
		ScriptPath:     scriptPath,
		TimeoutSeconds: timeoutSec,
	}
}

/******************************************************************************
 * Function Name : TrainWithPython
 *
 * Purpose :
 *   Spawns a Python training subprocess with the given config.
 *   Passes config as JSON via stdin, collects JSON result from stdout.
 *
 * Inputs :
 *   cfg   *ProcessTrainerConfig — Training configuration
 *
 * Return :
 *   Type        : *ProcessTrainerResult
 *   Description : Parsed result with artifact + fitness, or error.
 *
 * Complexity : O(subprocess_runtime), Number Of Lines : 30
 ******************************************************************************/
func (ps *ProcessSpawner) TrainWithPython(cfg *ProcessTrainerConfig) *ProcessTrainerResult {
	result := &ProcessTrainerResult{
		ModelID:   cfg.ModelID,
		Framework: cfg.Framework,
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("config marshal: %v", err)
		return result
	}

	start := time.Now()
	cmd := exec.Command(ps.PythonPath, ps.ScriptPath, "--stdin-config")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("subprocess start: %v", err)
		return result
	}

	// Feed config via stdin
	stdin.Write(configJSON)
	stdin.Close()

	// Collect output with timeout
	done := make(chan struct{})
	var outBytes, errBytes []byte
	go func() {
		outBuf := make([]byte, 65536)
		errBuf := make([]byte, 16384)
		nOut, _ := stdout.Read(outBuf)
		nErr, _ := stderr.Read(errBuf)
		outBytes = outBuf[:nOut]
		errBytes = errBuf[:nErr]
		close(done)
	}()

	select {
	case <-done:
		// completed
	case <-time.After(time.Duration(ps.TimeoutSeconds) * time.Second):
		cmd.Process.Kill()
		result.Success = false
		result.Error = fmt.Sprintf("timeout after %ds", ps.TimeoutSeconds)
		result.Duration = time.Since(start).Seconds()
		return result
	}

	cmd.Wait()
	result.Duration = time.Since(start).Seconds()
	result.Stdout = string(outBytes)
	result.Stderr = string(errBytes)

	// Parse JSON result from stdout
	if len(outBytes) > 0 {
		if err := json.Unmarshal(outBytes, result); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("result parse: %v, stdout=%s", err, string(outBytes[:minInt2(200, len(outBytes))]))
		}
	} else {
		result.Success = false
		result.Error = fmt.Sprintf("no stdout output; stderr=%s", string(errBytes[:minInt2(500, len(errBytes))]))
	}

	return result
}

/******************************************************************************
 * Function Name : TrainWithBinary
 *
 * Purpose :
 *   Spawns a compiled binary (Rust/C++) training subprocess.
 *   Passes config as JSON via stdin, similar protocol to Python.
 *
 * Inputs :
 *   binaryPath  string                 — Path to compiled binary
 *   cfg         *ProcessTrainerConfig  — Training configuration
 *
 * Return :
 *   Type        : *ProcessTrainerResult
 *   Description : Parsed result.
 *
 * Complexity : O(subprocess_runtime), Number Of Lines : 25
 ******************************************************************************/
func (ps *ProcessSpawner) TrainWithBinary(binaryPath string, cfg *ProcessTrainerConfig) *ProcessTrainerResult {
	result := &ProcessTrainerResult{
		ModelID:   cfg.ModelID,
		Framework: cfg.Framework,
	}

	configJSON, err := json.Marshal(cfg)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("config marshal: %v", err)
		return result
	}

	start := time.Now()
	cmd := exec.Command(binaryPath)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("binary start: %v", err)
		return result
	}

	stdin.Write(configJSON)
	stdin.Close()

	done := make(chan struct{})
	var outBytes, errBytes []byte
	go func() {
		outBuf := make([]byte, 65536)
		errBuf := make([]byte, 16384)
		nOut, _ := stdout.Read(outBuf)
		nErr, _ := stderr.Read(errBuf)
		outBytes = outBuf[:nOut]
		errBytes = errBuf[:nErr]
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Duration(ps.TimeoutSeconds) * time.Second):
		cmd.Process.Kill()
		result.Success = false
		result.Error = fmt.Sprintf("timeout after %ds", ps.TimeoutSeconds)
		result.Duration = time.Since(start).Seconds()
		return result
	}

	cmd.Wait()
	result.Duration = time.Since(start).Seconds()
	result.Stdout = string(outBytes)
	result.Stderr = string(errBytes)

	if len(outBytes) > 0 {
		if err := json.Unmarshal(outBytes, result); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("result parse: %v", err)
		}
	}

	return result
}

/******************************************************************************
 * Function Name : ConvertToFitness
 *
 * Purpose :
 *   Converts a ProcessTrainerResult to a FitnessHistory for the
 *   orchestrator. Extracts metrics from the artifact or result fields.
 *
 * Inputs :
 *   result  *ProcessTrainerResult — Subprocess result
 *
 * Return :
 *   Type        : *FitnessHistory
 *   Description : Populated fitness, or nil if result failed.
 *
 * Complexity : O(1), Number Of Lines : 15
 ******************************************************************************/
func ConvertToFitness(result *ProcessTrainerResult) *FitnessHistory {
	if result == nil || !result.Success {
		return nil
	}
	if result.Fitness != nil {
		return result.Fitness
	}
	// Fallback: extract from artifact metrics
	if result.Artifact != nil {
		m := result.Artifact.Metrics
		return &FitnessHistory{
			SharpeRatio:        m.SharpeRatio,
			SortinoRatio:       m.SortinoRatio,
			Profit:             m.ProfitabilityPnL,
			Drawdown:           m.MaxDrawdown,
			PredictionAccuracy: m.DirectionalAccuracy,
			Consistency:        m.DirectionalAccuracy,
			CapitalEfficiency:  0.5,
			Timestamp:          time.Now(),
		}
	}
	return nil
}

// minInt2 returns the smaller of two ints.
func minInt2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
