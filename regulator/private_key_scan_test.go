package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoExposedPrivateKeys(t *testing.T) {
	targetDir := "../"
	if customDir := os.Getenv("SCAN_DIR"); customDir != "" {
		targetDir = customDir
	}

	if ReferenceKey == "" {
		loadConfigEnv("../dexbot/config.env")
		if envKey := os.Getenv("TARGET_KEY"); envKey != "" {
			ReferenceKey = envKey
		} else if envKey := os.Getenv("PRIVATE_KEY"); envKey != "" {
			ReferenceKey = envKey
		}
	}

	violations, err := RunScan(targetDir)
	if err != nil {
		t.Fatalf("Failed to walk directory tree: %v", err)
	}

	// Filter out authorized exceptions (like config.env)
	var unauthorizedViolations []KeyMatch
	for _, v := range violations {
		fileName := strings.ToLower(filepath.Base(v.FilePath))
		if allowedFileNames[fileName] {
			continue // Skip allowed config files
		}
		unauthorizedViolations = append(unauthorizedViolations, v)
	}

	if len(unauthorizedViolations) > 0 {
		t.Errorf("🚨 FAIL: Found %d private key exposure(s) in unauthorized files:\n", len(unauthorizedViolations))
		for _, v := range unauthorizedViolations {
			t.Errorf("  • File: %s (Line %d)\n    Matched Key: %s\n    Snippet    : %s\n",
				v.FilePath, v.LineNumber, v.MatchedKey, strings.TrimSpace(v.LineText))
		}
	}
}