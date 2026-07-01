/******************************************************************************
 * File Name       : main.go
 * File Path       : testdaemon/main.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   Test Daemon — embedded CI/CD service. Monitors source changes via git diff, runs dependency analysis, executes affected tests, builds affected daemons, and stores results. Per myreq2.txt §24-26. Poll 
 *
 * Responsibilities:
 *   - Implement core functionality for testdaemon package.
 *
 * Usage :
 *   Directory : testdaemon/
 *
 *   Build :
 *     go build ./testdaemon
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./testdaemon
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/testdaemon
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
 *   [Types] Struct definitions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"dexbot/config"
	"dexbot/governance"
	"dexbot/infra"
)

// ==============================
// TYPES
// ==============================

/*
Struct: TestRunResult
Description:
  Records the result of one test daemon validation cycle.
  Stored in runtime/test_history.json for historical review (§26).

Fields:
  - Timestamp  time.Time : When the cycle ran
  - Changed    []string  : Files changed
  - Packages   []string  : Affected packages
  - TestsRan   []string  : Packages where tests were executed
  - VetPassed  bool      : go vet succeeded
  - TestPassed bool      : go test succeeded
  - BuildPassed bool     : go build succeeded
  - DaemonsRestart []string : Daemons recommended for restart
  - Duration   string    : Total cycle duration
  - Error      string    : Non-empty if any step failed

Lines: ~10
*/
type TestRunResult struct {
	Timestamp      time.Time `json:"timestamp"`
	Changed        []string  `json:"changed_files"`
	Packages       []string  `json:"affected_packages"`
	TestsRan       []string  `json:"tests_ran"`
	VetPassed      bool      `json:"vet_passed"`
	TestPassed     bool      `json:"test_passed"`
	BuildPassed    bool      `json:"build_passed"`
	DaemonsRestart []string  `json:"daemons_needing_restart"`
	Duration       string    `json:"duration"`
	Error          string    `json:"error,omitempty"`
}

// ==============================
// GLOBALS
// ==============================

var (
	projectRoot  string
	runtimeCfg   config.RuntimeConfig
	historyFile  string
	govUDPConn   *net.UDPConn // UDP connection to governance daemon (Phase 24)
)

// ==============================
func main() {
	fs := flag.NewFlagSet("testdaemon", flag.ContinueOnError)
	action := fs.String("action", "start", "Action: start, history")
	_ = fs.Parse(os.Args[1:])

	infra.InitLogger()
	infra.FnTrace("entering")

	projectRoot = findProjectRoot()
	runtimeCfg = config.Defaults()
	runtimeCfg.LoadFromEnv()
	initGovernanceUDP()
	historyFile = filepath.Join(projectRoot, "runtime", "test_history.json")

	if *action == "history" {
		printHistory()
		return
	}

	// Phase 24: Run full test suite on startup, report to governance
	runFullTestsAndReport()

	runLoop()
}

func findProjectRoot() string {
	// Try common locations
	candidates := []string{
		".",
		"/workspace/crypto_apps/dexbot",
	}
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(c, "go.mod")); err == nil {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	wd, _ := os.Getwd()
	return wd
}

// ==============================
// MAIN LOOP
// ==============================

/*
Function: runLoop
Description:
  Polls for changes and runs validation pipeline.
  Interval from TEST_POLL_INTERVAL_SECONDS config.

Input:
  - none

Output:
  - none

Lines: ~25
*/
func runLoop() {
	infra.FnTrace("entering")
	interval := time.Duration(runtimeCfg.TestDaemon.PollIntervalSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	infra.Info(fmt.Sprintf("Test Daemon started (poll=%v, root=%s)", interval, projectRoot))

	for range ticker.C {
		result := validatePipeline()
		storeResult(result)
		printResult(result)
		reportToGovernance(result)
	}
}

// ==============================
// VALIDATION PIPELINE
// ==============================

/*
Function: validatePipeline
Description:
  Runs the full CI/CD pipeline: changed files → affected packages → vet → test → build.

Input:
  - none

Output:
  - TestRunResult : Complete cycle result

Lines: ~40
*/
func validatePipeline() TestRunResult {
	infra.FnTrace("entering")
	start := time.Now()

	result := TestRunResult{
		Timestamp:   start,
		VetPassed:   true,
		TestPassed:  true,
		BuildPassed: true,
	}

	dc := NewDepChecker(projectRoot)

	// 1. Changed files
	files, err := dc.ChangedFiles()
	if err != nil {
		result.Error = "git diff failed: " + err.Error()
		result.Duration = time.Since(start).String()
		return result
	}
	result.Changed = files
	if len(files) == 0 {
		result.Duration = time.Since(start).String()
		return result
	}
	infra.Info(fmt.Sprintf("TestDaemon: %d files changed: %v", len(files), files))

	// 2. Affected packages
	pkgs, err := dc.AffectedPackages(files)
	if err != nil {
		result.Error = "package analysis failed: " + err.Error()
		result.Duration = time.Since(start).String()
		return result
	}
	result.Packages = pkgs
	if len(pkgs) == 0 {
		result.Duration = time.Since(start).String()
		return result
	}
	infra.Info(fmt.Sprintf("TestDaemon: %d packages affected: %v", len(pkgs), pkgs))

	// 3. Affected tests
	testPkgs, err := dc.AffectedTests(pkgs)
	if err == nil {
		result.TestsRan = testPkgs
	}

	// 4. go vet
	result.VetPassed = runVet(pkgs)
	if !result.VetPassed {
		result.Error = "go vet failed"
	}

	// 5. go test (only if vet passed)
	if result.VetPassed && len(testPkgs) > 0 {
		result.TestPassed = runTests(testPkgs)
		if !result.TestPassed {
			result.Error = "go test failed"
		}
	}

	// 6. go build on daemon entries
	restartDaemons := dc.DaemonsNeedingRestart(pkgs)
	result.DaemonsRestart = restartDaemons
	result.BuildPassed = runBuild()
	if !result.BuildPassed {
		if result.Error == "" {
			result.Error = "go build failed"
		}
	}

	result.Duration = time.Since(start).String()
	return result
}

// ==============================
// EXECUTION HELPERS
// ==============================

func runVet(pkgs []string) bool {
	infra.FnTrace(fmt.Sprintf("vetting %d packages", len(pkgs)))
	args := append([]string{"vet"}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		infra.Error("go vet FAILED:\n" + string(out))
		return false
	}
	infra.Info("go vet PASSED")
	return true
}

func runTests(pkgs []string) bool {
	infra.FnTrace(fmt.Sprintf("testing %d packages", len(pkgs)))
	args := append([]string{"test", "-count=1", "-timeout=120s"}, pkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		infra.Error("go test FAILED:\n" + string(out))
		return false
	}
	infra.Info("go test PASSED")
	return true
}

func runBuild() bool {
	infra.FnTrace("building all daemons")
	cmd := exec.Command("go", "build", "./...")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		infra.Error("go build FAILED:\n" + string(out))
		return false
	}
	infra.Info("go build PASSED")
	return true
}

// ==============================
// PERSISTENCE
// ==============================

/*
Function: storeResult
Description:
  Appends a TestRunResult to runtime/test_history.json.
  Caps history at 100 entries.

Input:
  - result TestRunResult : Cycle result

Output:
  - none

Lines: ~20
*/
func storeResult(result TestRunResult) {
	var history []TestRunResult
	data, err := os.ReadFile(historyFile)
	if err == nil {
		json.Unmarshal(data, &history)
	}

	history = append(history, result)
	if len(history) > 100 {
		history = history[len(history)-100:]
	}

	os.MkdirAll(filepath.Dir(historyFile), 0755)
	out, _ := json.MarshalIndent(history, "", "  ")
	os.WriteFile(historyFile, out, 0644)
}

func printResult(result TestRunResult) {
	passFail := func(b bool) string {
		if b {
			return "PASS"
		}
		return "FAIL"
	}
	changed := "none"
	if len(result.Changed) > 0 {
		changed = fmt.Sprintf("%d files", len(result.Changed))
	}
	fmt.Printf("\n═══ Test Daemon Result ═══\n")
	fmt.Printf("  Time:    %s\n", result.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Changed: %s\n", changed)
	fmt.Printf("  Pkgs:    %d affected\n", len(result.Packages))
	fmt.Printf("  Tests:   %d packages\n", len(result.TestsRan))
	fmt.Printf("  Vet:     %s\n", passFail(result.VetPassed))
	fmt.Printf("  Test:    %s\n", passFail(result.TestPassed))
	fmt.Printf("  Build:   %s\n", passFail(result.BuildPassed))
	fmt.Printf("  Restart: %v\n", result.DaemonsRestart)
	fmt.Printf("  Duration:%s\n", result.Duration)
	if result.Error != "" {
		fmt.Printf("  Error:   %s\n", result.Error)
	}
	fmt.Println()
}

func printHistory() {
	data, err := os.ReadFile(historyFile)
	if err != nil {
		fmt.Println("No test history found.")
		return
	}
	var history []TestRunResult
	if err := json.Unmarshal(data, &history); err != nil {
		fmt.Println("Test history corrupt:", err)
		return
	}
	fmt.Printf("Test History (%d entries):\n", len(history))
	for i, r := range history {
		status := "PASS"
		if r.Error != "" {
			status = r.Error
		}
		fmt.Printf("  %d. %s  vet=%v test=%v build=%v  %s\n",
			i+1, r.Timestamp.Format("2006-01-02 15:04"), r.VetPassed, r.TestPassed, r.BuildPassed, status)
	}
}

// ==============================
// GOVERNANCE REPORTING (Phase 24)
// ==============================

func initGovernanceUDP() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", runtimeCfg.GovernanceUDPPort))
	if err != nil {
		infra.Warn("TestDaemon: cannot resolve governance UDP: " + err.Error())
		return
	}
	govUDPConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		infra.Warn("TestDaemon: cannot dial governance UDP: " + err.Error())
		return
	}
}

func runFullTestsAndReport() {
	infra.FnTrace("entering")
	defer infra.FnTrace("done")

	fmt.Println("\n═══ Test Daemon: Running full validation suite...")
	result := validateFullSuite()
	storeResult(result)

	// Report to governance
	reportToGovernance(result)

	if result.TestPassed && result.VetPassed && result.BuildPassed {
		fmt.Println("═══ ALL CHECKS PASSED ═══")
	} else {
		fmt.Printf("═══ CHECKS COMPLETE: vet=%v test=%v build=%v ═══\n",
			result.VetPassed, result.TestPassed, result.BuildPassed)
	}
}

func validateFullSuite() TestRunResult {
	start := time.Now()
	result := TestRunResult{
		Timestamp:   start,
		VetPassed:   true,
		TestPassed:  true,
		BuildPassed: true,
	}

	// 1. go vet ./...
	infra.Info("TestDaemon: go vet ./...")
	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = projectRoot
	if out, err := vetCmd.CombinedOutput(); err != nil {
		result.VetPassed = false
		result.Error = "go vet failed"
		infra.Error("go vet FAILED:\n" + string(out))
	} else {
		infra.Info("go vet PASSED")
	}

	// 2. go test ./... (only if vet passed)
	if result.VetPassed {
		infra.Info("TestDaemon: go test ./...")
		testCmd := exec.Command("go", "test", "-count=1", "-timeout=120s", "./...")
		testCmd.Dir = projectRoot
		if out, err := testCmd.CombinedOutput(); err != nil {
			result.TestPassed = false
			result.Error = "go test failed"
			infra.Error("go test FAILED:\n" + string(out))
		} else {
			infra.Info("go test PASSED")
		}
	}

	// 3. go build ./...
	if result.TestPassed {
		infra.Info("TestDaemon: go build ./...")
		buildCmd := exec.Command("go", "build", "./...")
		buildCmd.Dir = projectRoot
		if out, err := buildCmd.CombinedOutput(); err != nil {
			result.BuildPassed = false
			result.Error = "go build failed"
			infra.Error("go build FAILED:\n" + string(out))
		} else {
			infra.Info("go build PASSED")
		}
	}

	// 4. Determine affected daemons (infra change → all, else targeted)
	dc := NewDepChecker(projectRoot)
	files, _ := dc.ChangedFiles()
	result.Changed = files
	if len(files) > 0 {
		pkgs, _ := dc.AffectedPackages(files)
		result.Packages = pkgs
		result.DaemonsRestart = dc.DaemonsNeedingRestart(pkgs)
	}

	result.Duration = time.Since(start).String()
	return result
}

func reportToGovernance(result TestRunResult) {
	if govUDPConn == nil {
		return
	}
	status := "pass"
	if !result.TestPassed || !result.VetPassed {
		status = "fail"
	}

	msg := fmt.Sprintf("testdaemon:%s:%s:vet=%t:test=%t:build=%t:daemons=%v",
		status, result.Duration,
		result.VetPassed, result.TestPassed, result.BuildPassed,
		result.DaemonsRestart)

	govUDPConn.Write([]byte(msg))
	infra.Info("TestDaemon: reported to governance: " + msg)

	// Heartbeat with results — include affected daemon list
	daemonList := "none"
	if len(result.DaemonsRestart) > 0 {
		daemonList = ""
		for i, d := range result.DaemonsRestart {
			if i > 0 {
				daemonList += ","
			}
			daemonList += d
		}
	}
	hb := &governance.DaemonInfo{
		Name:    "testdaemon",
		Version: "v1.0",
		Status:  status,
		ActiveTasks: len(result.DaemonsRestart),
		Message: fmt.Sprintf("vet=%t test=%t build=%t daemons=%s",
			result.VetPassed, result.TestPassed, result.BuildPassed, daemonList),
		LastHeartbeat: time.Now(),
	}
	govUDPConn.Write([]byte(governance.FormatHeartbeat(hb)))
}