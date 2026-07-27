/******************************************************************************
 * File Name       : main.go
 * File Path       : regulator/main.go
 *
 * Author          : Gemini 3.1 Pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-26 12:00:00 (UTC+7)
 * Modified Date   : 2026-07-26 12:00:00 (UTC+7)
 *
 * Description     :
 *   Main entry point for the Regulator audit tool suite. Coordinates execution
 *   of private key scanning, function doc-comment verification, and top-level
 *   file header compliance checks across source code repositories.
 *
 * Responsibilities:
 *   - Parse command line flags, target directories, and target private keys.
 *   - Invoke Key Scanner, Function Comment Checker, and File Header Checker.
 *   - Format audit failures and set non-zero exit code on compliance failures.
 *
 * Usage :
 *   Directory : regulator/
 *
 *   Build :
 *     go build -o regulator .
 *
 *   Run :
 *     go run . <target_directory> [target_private_key]
 * 
 *  		./regulator ../dexbot xxxx 
 *
 * Dependencies :
 *   Internal :
 *     - key_scanner.go
 *     - comment_checker.go
 *     - file_header_checker.go
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   [Function]
 *     - None
 *
 * New Parts :
 *   [Function]
 *     - main()
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 12:00:00      | Gemini 3.1 Pro  | Initial orchestrator
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add optional JSON output reporter flag.
 *
 * Notes :
 *   - Per regulator code standard rules.
 ******************************************************************************/
package main

import (
	"fmt"
	"os"
	"strings"
)

/******************************************************************************
 * Function Name : main
 *
 * Purpose :
 *   Main execution flow running key scan, function doc scan, and header scan.
 *
 * Inputs :
 *   None (Reads os.Args)
 *
 * Return :
 *   None (Exits with 0 on pass, 1 on failure)
 *
 * Complexity :
 *   Time  : O(N) where N is total file count in target tree.
 *   Space : O(K) where K is number of violations accumulated.
 *
 * Error Cases :
 *   - Exits with 1 if any audit step detects a failure.
 *
 * Number Of Lines :
 *   65
 ******************************************************************************/
func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run . <directory_path> [target_private_key]")
		os.Exit(1)
	}

	targetDir := os.Args[1]

	if len(os.Args) >= 3 {
		ReferenceKey = os.Args[2]
	} else {
		loadConfigEnv("../dexbot/config.env")
		if envKey := os.Getenv("TARGET_KEY"); envKey != "" {
			ReferenceKey = envKey
		} else if envKey := os.Getenv("PRIVATE_KEY"); envKey != "" {
			ReferenceKey = envKey
		}
	}

	hasFailures := false

	fmt.Printf("Scanning directory for private key exposures: %s...\n", targetDir)
	if ReferenceKey != "" {
		fmt.Printf("Target Reference Key: %s\n", ReferenceKey)
		fmt.Printf("Threshold: ≥ %.2f%% match\n\n", SimilarityThreshold)
	} else {
		fmt.Printf("No specific target key provided. Scanning for general key exposures...\n\n")
	}

	keyViolations, err := RunScan(targetDir)
	if err != nil {
		fmt.Printf("Error walking target directory: %v\n", err)
		os.Exit(1)
	}

	if len(keyViolations) == 0 {
		fmt.Println("✅ PASS: No unauthorized private keys found!")
	} else {
		hasFailures = true
		fmt.Printf("🚨 FAIL: Found %d private key exposure(s):\n\n", len(keyViolations))
		for _, v := range keyViolations {
			fmt.Printf("  • File: %s (Line %d) [Match: %.2f%%]\n", v.FilePath, v.LineNumber, v.Similarity)
			fmt.Printf("    Matched String: %s\n", v.MatchedKey)
			fmt.Printf("    Line Snippet  : %s\n\n", strings.TrimSpace(v.LineText))
		}
	}

	fmt.Println("\n--- Running Function Comment Header Audit ---")
	docViolations, err := CheckFunctionDocComments(targetDir)
	if err != nil {
		fmt.Printf("Error checking function doc comments: %v\n", err)
	} else if len(docViolations) > 0 {
		hasFailures = true
		fmt.Printf("🚨 FAIL: Found %d function(s) with invalid or missing doc comments:\n\n", len(docViolations))
		PrintViolationTable(docViolations)
	} else {
		fmt.Println("✅ PASS: All Go, JS, and Python functions follow the standard doc comment format!")
	}

	fmt.Println("\n--- Running File Header Audit (.go, .py, .sh, .c, .cpp, .html, .js) ---")
	headerViolations, err := CheckFileHeaders(targetDir)
	if err != nil {
		fmt.Printf("Error checking file headers: %v\n", err)
	} else if len(headerViolations) > 0 {
		hasFailures = true
		fmt.Printf("🚨 FAIL: Found %d file(s) with missing or incomplete file headers:\n\n", len(headerViolations))
		PrintHeaderViolationTable(headerViolations)
	} else {
		fmt.Println("✅ PASS: All source files contain compliant file headers!")
	}

	if hasFailures {
		os.Exit(1)
	}
}
