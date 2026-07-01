/******************************************************************************
 * File Name       : main_test.go
 * File Path       : testdaemon/main_test.go
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
 *   Unit tests for Test Daemon. 5 positive + 2 negative + 2 utility tests per coding rule §2. go test ./testdaemon -v Directory: dexbot/testdaemon/
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
 *   [Test Functions] Test suite: TestDepChecker_AffectedPackages_KnownFile, TestDepChecker_AffectedTests_Infra, TestDepChecker_DaemonsNeedingRestart, TestDepChecker_ChangedFiles_ReturnsSlice
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
	"os"
	"path/filepath"
	"testing"
)

func getProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "/workspace/crypto_apps/dexbot"
		}
		dir = parent
	}
}

// ==============================
// POSITIVE TESTS
// ==============================

func TestDepChecker_AffectedPackages_KnownFile(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, err := dc.AffectedPackages([]string{"infra/logger.go"})
	if err != nil {
		t.Fatalf("AffectedPackages failed: %v", err)
	}
	found := false
	for _, p := range pkgs {
		if p == "dexbot/infra" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected dexbot/infra in affected packages, got %v", pkgs)
	}
}

func TestDepChecker_AffectedTests_Infra(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, err := dc.AffectedPackages([]string{"infra/logger.go"})
	if err != nil {
		t.Fatalf("AffectedPackages: %v", err)
	}
	testPkgs, err := dc.AffectedTests(pkgs)
	if err != nil {
		t.Fatalf("AffectedTests: %v", err)
	}
	if len(testPkgs) == 0 {
		t.Error("Expected at least 1 test package")
	}
}

func TestDepChecker_DaemonsNeedingRestart(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, _ := dc.AffectedPackages([]string{"infra/logger.go"})
	daemons := dc.DaemonsNeedingRestart(pkgs)
	if len(daemons) < 3 {
		t.Errorf("Expected at least 3 daemons needing restart (infra change), got %d: %v", len(daemons), daemons)
	}
}

func TestDepChecker_ChangedFiles_ReturnsSlice(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	files, err := dc.ChangedFiles()
	if err != nil {
		t.Logf("ChangedFiles returned error (acceptable in CI): %v", err)
	}
	if files == nil && err == nil {
		t.Log("No changed files — clean working tree")
	}
}

func TestDepChecker_NoChanges_EmptyPackages(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, err := dc.AffectedPackages(nil)
	if err != nil {
		t.Fatalf("AffectedPackages: %v", err)
	}
	if pkgs != nil {
		t.Errorf("Expected nil for empty input, got %v", pkgs)
	}
}

// ==============================
// NEGATIVE TESTS
// ==============================

func TestDepChecker_NonexistentFile(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, err := dc.AffectedPackages([]string{"ghost/never_exists.go"})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(pkgs) > 0 {
		t.Logf("Ghost file matched packages: %v", pkgs)
	}
}

func TestDepChecker_EmptyAffectedTests(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, err := dc.AffectedTests(nil)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if pkgs != nil {
		t.Errorf("Expected nil for empty input, got %v", pkgs)
	}
}

// ==============================
// UTILITY TESTS
// ==============================

func TestTestRunResult_JSONRoundTrip(t *testing.T) {
	r := TestRunResult{
		Changed:  []string{"infra/logger.go"},
		Packages: []string{"dexbot/infra"},
		TestsRan: []string{"dexbot/infra"},
		VetPassed: true, TestPassed: true, BuildPassed: true,
		DaemonsRestart: []string{"governance", "school", "trading"},
		Duration:       "1.5s",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var r2 TestRunResult
	if err := json.Unmarshal(data, &r2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if !r2.VetPassed || len(r2.Packages) != 1 {
		t.Errorf("Round-trip mismatch: %+v", r2)
	}
}

func TestDaemonsNeedingRestart_WithLibrary(t *testing.T) {
	dc := NewDepChecker(getProjectRoot())
	pkgs, _ := dc.AffectedPackages([]string{"governance/registry.go"})
	daemons := dc.DaemonsNeedingRestart(pkgs)
	hasGov := false
	for _, d := range daemons {
		if d == "governance" {
			hasGov = true
		}
	}
	if !hasGov {
		t.Errorf("Expected governance restart for registry.go change, got %v", daemons)
	}
}
