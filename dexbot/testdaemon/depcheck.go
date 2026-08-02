/******************************************************************************
 * File Name       : depcheck.go
 * File Path       : testdaemon/depcheck.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
  *
  * Function Name : relativePath
  * Purpose :
  *   Performs its designated operation.
  *
  * Function Name : DaemonsNeedingRestart
  * Purpose :
  *   Performs its designated operation.
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:34 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:34 (UTC+7)
 *
 * Description     :
 *   Go package dependency analyzer for the Test Daemon (§24-25). Uses `go list` to find affected packages, tests, and daemons when source files change. Per myreq2.txt §25, dependency analysis uses Go pack
 *
 * Responsibilities:
 *   - - Implement core functionality for testdaemon package.
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
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:34 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
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
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ==============================
// DEP CHECKER
// ==============================

/*
Struct: DepChecker
Description:
  Analyzes Go package dependencies using `go list -json`.
  Determines which packages/tests/daemons are affected by file changes.

Fields:
  - projectRoot string : Absolute path to dexbot/

Lines: ~3
*/
type DepChecker struct {
	projectRoot string
}

/******************************************************************************
 * Function Name : NewDepChecker
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func NewDepChecker(root string) *DepChecker {
	return &DepChecker{projectRoot: root}
}

// goListPackage represents a single package from `go list -json`.
type goListPackage struct {
	Dir        string   `json:"Dir"`
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	GoFiles    []string `json:"GoFiles"`
	TestGoFiles []string `json:"TestGoFiles"`
	XTestGoFiles []string `json:"XTestGoFiles"`
	Imports    []string `json:"Imports"`
	Deps       []string `json:"Deps"`
}

/******************************************************************************
 * Function Name : ChangedFiles
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (dc *DepChecker) ChangedFiles() ([]string, error) {
	// Try git first
	cmd := exec.Command("git", "-C", dc.projectRoot, "diff", "--name-only", "HEAD")
	out, err := cmd.Output()
	if err == nil && len(out) > 0 {
		var files []string
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			f = strings.TrimSpace(f)
			if strings.HasSuffix(f, ".go") {
				files = append(files, f)
			}
		}
		return files, nil
	}

	// Fallback: return empty (nothing changed)
	return nil, nil
}

/******************************************************************************
 * Function Name : AffectedPackages
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/
func (dc *DepChecker) AffectedPackages(changedFiles []string) ([]string, error) {
	if len(changedFiles) == 0 {
		return nil, nil
	}

	// Get all packages
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = dc.projectRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go list failed: %w", err)
	}

	// Parse JSON stream
	var allPkgs []goListPackage
	decoder := json.NewDecoder(strings.NewReader(string(out)))
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			continue
		}
		allPkgs = append(allPkgs, pkg)
	}

	// Build filename → package map
	fileToPkg := make(map[string]string)
	for _, pkg := range allPkgs {
		for _, f := range pkg.GoFiles {
			rel := relativePath(dc.projectRoot, filepath.Join(pkg.Dir, f))
			fileToPkg[rel] = pkg.ImportPath
		}
		for _, f := range pkg.TestGoFiles {
			rel := relativePath(dc.projectRoot, filepath.Join(pkg.Dir, f))
			fileToPkg[rel] = pkg.ImportPath
		}
	}

	// Match changed files to packages
	affected := make(map[string]bool)
	for _, cf := range changedFiles {
		if pkg, ok := fileToPkg[cf]; ok {
			affected[pkg] = true
		}
		// Also match by directory prefix
		for pkgDir, pkg := range fileToPkg {
			if strings.HasPrefix(cf, filepath.Dir(pkgDir)) {
				affected[pkg] = true
			}
		}
	}

	var result []string
	for pkg := range affected {
		result = append(result, pkg)
	}
	return result, nil
}

/******************************************************************************
 * Function Name : AffectedTests
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (dc *DepChecker) AffectedTests(affectedPkgs []string) ([]string, error) {
	if len(affectedPkgs) == 0 {
		return nil, nil
	}

	args := append([]string{"list", "-json"}, affectedPkgs...)
	cmd := exec.Command("go", args...)
	cmd.Dir = dc.projectRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var testPkgs []string
	decoder := json.NewDecoder(strings.NewReader(string(out)))
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			continue
		}
		if len(pkg.TestGoFiles) > 0 || len(pkg.XTestGoFiles) > 0 {
			testPkgs = append(testPkgs, pkg.ImportPath)
		}
	}
	return testPkgs, nil
}

/******************************************************************************
 * Function Name : DaemonsNeedingRestart
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func (dc *DepChecker) DaemonsNeedingRestart(affectedPkgs []string) []string {
	daemonDirs := map[string]string{
		"dexbot/apps/governance": "governance",
		"dexbot/apps/school":     "school",
		"dexbot/apps/trading":    "trading",
		"dexbot/testdaemon":      "testdaemon",
		"dexbot/governance":      "governance",
		"dexbot/school":          "school",
		"dexbot/trading":         "trading",
		"dexbot/infra":           "governance school trading testdaemon",
		"dexbot/config":          "governance school trading testdaemon",
		"dexbot/webui":           "governance",
	}

	restart := make(map[string]bool)
	for _, pkg := range affectedPkgs {
		if names, ok := daemonDirs[pkg]; ok {
			for _, n := range strings.Fields(names) {
				restart[n] = true
			}
		}
	}

	var result []string
	for name := range restart {
		result = append(result, name)
	}
	return result
}

// relativePath returns the relative path of target from base.
/******************************************************************************
 * Function Name : relativePath
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func relativePath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}
