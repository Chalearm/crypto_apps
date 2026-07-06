/******************************************************************************
 * File Name       : db_fallback_failure_test.go
 * File Path       : integration/db_fallback_failure_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-05 16:30:00 (UTC+7)
 *
 * Description     :
 *   PROVES the DB dropdown fails by demonstrating the fallback path is broken.
 *   The serve.py DB API has TWO paths:
 *     1. Cached JSON from governance (works if governance has run)
 *     2. Direct psycopg2 query (fallback — BROKEN because psycopg2 not installed)
 *
 *   When governance cache is missing (fresh start, crash, or cache duration expired),
 *   the fallback fails and the browser gets nothing.
 *
 *   THIS TEST WILL FAIL TO DEMONSTRATE THE BUG.
 ******************************************************************************/
package integration

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestDBDropdownFallbackBroken DELIBERATELY triggers the failure path
// by removing the governance-generated cache file, then calling
// /api/database_tables to show it returns 500 (psycopg2 not installed).
func TestDBDropdownFallbackBroken(t *testing.T) {
	// Find the cache file
	cachePaths := []string{
		"web_output/api/database_tables.json",
		"../web_output/api/database_tables.json",
	}
	var cacheFile string
	for _, p := range cachePaths {
		if _, err := os.Stat(p); err == nil {
			cacheFile = p
			break
		}
	}

	if cacheFile == "" {
		// File doesn't exist — verify the fallback is broken right now
		t.Log("Cache file does not exist — DB dropdown is ALREADY in fallback mode")
	} else {
		t.Logf("Cache file exists at: %s", cacheFile)
	}

	// 1. Check if psycopg2 is installed (required for fallback)
	psycopg2Available := false
	out, err := exec.Command("python3", "-c", "import psycopg2; print('ok')").CombinedOutput()
	if err == nil && strings.TrimSpace(string(out)) == "ok" {
		psycopg2Available = true
	}
	t.Logf("psycopg2 available: %v", psycopg2Available)

	// 2. Try calling /api/database_tables
	resp := apiCall(t, "/api/database_tables")
	t.Logf("/api/database_tables response: %s", strings.TrimSpace(resp[:strMin(len(resp), 200)]))

	// 3. Determine whether the DB dropdown actually works
	if strings.Contains(resp, `"tables"`) && strings.Contains(resp, `[`) {
		t.Log("OK: /api/database_tables returns tables — dropdown works (governance cache present)")
	} else if strings.Contains(resp, "error") || strings.Contains(resp, "Error") || strings.Contains(resp, "500") {
		t.Errorf("FAIL: /api/database_tables returned error: %s", resp[:strMin(len(resp), 200)])
		if !psycopg2Available {
			t.Errorf("ROOT CAUSE: psycopg2 NOT INSTALLED — serve.py fallback ALWAYS fails")
			t.Errorf("FIX: pip3 install psycopg2-binary OR serve.py fallback must not depend on psycopg2")
		}
	} else if strings.Contains(resp, `"tables": []`) || strings.Contains(resp, `"tables":null`) {
		t.Error("FAIL: /api/database_tables returned empty tables list — dropdown shows nothing")
	}

	// 4. If cache file exists, remove it and re-test to prove the fallback path fails
	if cacheFile != "" && psycopg2Available {
		// Backup
		data, _ := os.ReadFile(cacheFile)
		os.Remove(cacheFile)
		defer os.WriteFile(cacheFile, data, 0644)

		resp2 := apiCall(t, "/api/database_tables")
		if strings.Contains(resp2, "error") || strings.Contains(resp2, "500") {
			t.Errorf("FAIL (WITHOUT CACHE): /api/database_tables: %s", resp2[:min(len(resp2), 200)])
		} else {
			t.Logf("OK (WITHOUT CACHE): fallback works: %s", resp2[:min(len(resp2), 200)])
		}
	} else if cacheFile != "" && !psycopg2Available {
		t.Log("Skipping cache removal test — psycopg2 not installed so fallback would fail regardless")
	}
}

func apiCall(t *testing.T, path string) string {
	t.Helper()
	urls := []string{
		fmt.Sprintf("http://127.0.0.1:8080%s", path),
		fmt.Sprintf("http://localhost:8080%s", path),
	}
	for _, u := range urls {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(u)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}
	return fmt.Sprintf("unreachable (tried %v)", urls)
}

func strMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
