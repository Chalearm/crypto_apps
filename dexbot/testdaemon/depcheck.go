/******************************************************************************
 * File Name       : depcheck.go
 * File Path       : testdaemon/depcheck.go
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
 *   Go package dependency analyzer for the Test Daemon (§24-25). Uses `go list` to find affected packages, tests, and daemons when source files change. Per myreq2.txt §25, dependency analysis uses Go pack
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

/*
Function: NewDepChecker
Description:
  Creates a new DepChecker for the given project root.

Input:
  - root string : Project root path

Output:
  - *DepChecker : Initialized checker

Lines: ~6
*/
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

/*
Function: ChangedFiles
Description:
  Returns Go source files changed since the last test run.
  Uses `git diff --name-only HEAD` when available, otherwise scans
  all .go files with recent modification times.

Input:
  - none

Output:
  - []string : Changed .go file paths relative to project root
  - error    : Non-nil if git command fails

Lines: ~20
*/
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

/*
Function: AffectedPackages
Description:
  Determines which Go packages are affected by changes to the given files.
  Uses `go list -json ./...` and matches file paths to package Dirs.

Input:
  - changedFiles []string : Changed .go files (from ChangedFiles)

Output:
  - []string : Affected package import paths (e.g., "dexbot/infra")
  - error    : Non-nil if go list fails

Lines: ~30
*/
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

/*
Function: AffectedTests
Description:
  Returns the list of packages that have test files among the affected set.

Input:
  - affectedPkgs []string : Packages returned by AffectedPackages

Output:
  - []string : Packages with test files (may be same as input, filtered)

Lines: ~15
*/
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

/*
Function: DaemonsNeedingRestart
Description:
  Returns which daemon app directories are affected and need restart.
  daemon directories: apps/governance, apps/school, apps/trading.

Input:
  - affectedPkgs []string : Packages from AffectedPackages

Output:
  - []string : Daemon names needing restart

Lines: ~15
*/
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
func relativePath(base, target string) string {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return target
	}
	return filepath.ToSlash(rel)
}
