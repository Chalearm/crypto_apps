package main

import (
	"os"
	"testing"
)

func TestFileHeaders(t *testing.T) {
	targetDir := "../"
	if customDir := os.Getenv("SCAN_DIR"); customDir != "" {
		targetDir = customDir
	}

	violations, err := CheckFileHeaders(targetDir)
	if err != nil {
		t.Fatalf("Failed to execute file header scan: %v", err)
	}

	if len(violations) > 0 {
		t.Errorf("🚨 FAIL: Found %d file(s) with invalid/missing file headers:\n", len(violations))
		PrintHeaderViolationTable(violations)
	}
}
