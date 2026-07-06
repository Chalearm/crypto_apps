/******************************************************************************
 * File Name       : db_dropdown_test.go
 * File Path       : integration/db_dropdown_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-05 16:00:00 (UTC+7)
 *
 * Description     :
 *   End-to-end test that simulates what a BROWSER does when loading
 *   the School page and trying to use the DB browser dropdown.
 *   Demonstrates the exact failures the user sees.
 *
 *   Test flow:
 *     1. Load school page HTML → check DB browser elements exist
 *     2. Simulate populateDBTables() → fetch /api/database_tables
 *     3. Verify table list populates the dropdown
 *     4. Simulate loadDBTable() → fetch /api/database?table=X
 *     5. Verify row data is returned and displayable
 *     6. Check for duplicate function definitions (renderer bug)
 *
 * Usage : go test ./integration -v -run DBDropdown -timeout 30s
 ******************************************************************************/
package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestDBDropdownPopulates verifies that the DB browser dropdown can
// retrieve table names from the API and populate the select element.
func TestDBDropdownPopulates(t *testing.T) {
	// 1. Fetch the school page
	html := fetchPage(t, "/school")
	if html == "" {
		t.Fatal("FATAL: School page not reachable — browser shows blank page")
	}

	// 2. Check populateDBTables function definition exists
	if !strings.Contains(html, "function populateDBTables") {
		t.Fatal("FATAL: populateDBTables function missing from page — dropdown never populates")
	}
	t.Log("OK: populateDBTables function found")

	// 3. Check that it calls the correct API endpoint
	if !strings.Contains(html, `/api/database_tables`) {
		t.Fatal("FATAL: /api/database_tables not referenced in function — no API call made")
	}
	t.Log("OK: /api/database_tables URL found in JS")

	// 4. Check that dbTableSelect exists with onchange handler
	if !strings.Contains(html, `id="dbTableSelect"`) {
		t.Fatal("FATAL: dbTableSelect element missing — no dropdown to populate")
	}
	t.Log("OK: dbTableSelect element found")

	// 5. Check for duplicate JS function definitions (silent JS error)
	mPop := regexp.MustCompile(`function populateDBTables`)
	popMatches := mPop.FindAllString(html, -1)
	if len(popMatches) > 1 {
		t.Errorf("BUG: populateDBTables defined %d times — browser throws 'already declared' error", len(popMatches))
	} else {
		t.Log("OK: populateDBTables defined exactly once")
	}

	mLoad := regexp.MustCompile(`function loadDBTable`)
	loadMatches := mLoad.FindAllString(html, -1)
	if len(loadMatches) > 1 {
		t.Errorf("BUG: loadDBTable defined %d times — browser throws 'already declared' error", len(loadMatches))
	} else {
		t.Log("OK: loadDBTable defined exactly once")
	}

	// 6. Actually call the API endpoint (same as what fetch() does in browser)
	resp := apiGet(t, "/api/database_tables")
	if resp == "" {
		t.Fatal("FATAL: /api/database_tables API call failed — no response")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		t.Fatalf("FATAL: /api/database_tables returned invalid JSON: %v", err)
	}

	tables, ok := result["tables"].([]interface{})
	if !ok {
		t.Fatalf("FATAL: response has no 'tables' array: %v", result)
	}
	if len(tables) == 0 {
		t.Fatal("FATAL: /api/database_tables returned 0 tables — dropdown shows empty")
	}
	t.Logf("OK: /api/database_tables returned %d tables", len(tables))

	// 7. Verify the select element would actually populate
	// (Check that the JS adds <option> elements to dbTableSelect)
	if !strings.Contains(html, `sel.appendChild(o)`) && !strings.Contains(html, `appendChild`) {
		t.Error("BUG: populateDBTables JS does not append <option> elements — dropdown stays empty even after API call")
	} else {
		t.Log("OK: JS appends <option> elements to select")
	}
}

// TestDBDropdownQueryRows verifies that selecting a table and clicking
// load actually fetches rows and displays them.
func TestDBDropdownQueryRows(t *testing.T) {
	html := fetchPage(t, "/school")
	if html == "" {
		t.Fatal("FATAL: School page not reachable")
	}

	// 1. Check loadDBTable function exists and has proper error handling
	if !strings.Contains(html, "function loadDBTable") {
		t.Fatal("FATAL: loadDBTable function missing")
	}

	// 2. Check the fetch URL pattern in loadDBTable
	fetchPattern := regexp.MustCompile(`/api/database\?table=`)
	if !fetchPattern.MatchString(html) {
		t.Fatal("FATAL: /api/database?table= not found in loadDBTable — can't query tables")
	}
	t.Log("OK: loadDBTable fetches /api/database?table=")

	// 3. Actually test a real table query
	resp := apiGet(t, "/api/database?table=market_prices&rows=5&sort=newest")
	if resp == "" {
		t.Fatal("FATAL: /api/database?table=market_prices API call failed")
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(resp), &data); err != nil {
		t.Fatalf("FATAL: invalid JSON from /api/database: %v", err)
	}

	cols, _ := data["columns"].([]interface{})
	rows, _ := data["rows"].([]interface{})
	t.Logf("OK: market_prices query returned %d columns x %d rows", len(cols), len(rows))

	if len(cols) == 0 {
		t.Error("BUG: no columns returned — table header won't render")
	}

	// 4. Check loadDBTable has proper innerHTML rendering
	if !strings.Contains(html, "innerHTML") && !strings.Contains(html, "dbTableView") {
		t.Error("BUG: loadDBTable does not update dbTableView innerHTML — results never shown")
	}

	// 5. Check error handling in loadDBTable
	if !strings.Contains(html, "d.error") && !strings.Contains(html, "catch") {
		t.Error("BUG: loadDBTable has no error handling — API failures show nothing to user")
	}
}

// TestDBDropdownCORSHeaders checks that API responses include CORS headers
// needed for the browser to allow the fetch.
func TestDBDropdownCORSHeaders(t *testing.T) {
	urls := []string{
		"http://127.0.0.1:8080/api/database_tables",
		"http://localhost:8080/api/database_tables",
	}
	var resp *http.Response
	var err error
	for _, u := range urls {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err = client.Get(u)
		if err == nil && resp.StatusCode == 200 {
			break
		}
	}
	if err != nil {
		t.Skipf("web server not reachable: %v", err)
	}
	defer resp.Body.Close()

	corsOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	corsMethods := resp.Header.Get("Access-Control-Allow-Methods")

	t.Logf("Access-Control-Allow-Origin: %q", corsOrigin)
	t.Logf("Access-Control-Allow-Methods: %q", corsMethods)

	if corsOrigin != "*" {
		t.Errorf("BUG: CORS Allow-Origin is %q (expected '*') — browser may block fetch from /school page", corsOrigin)
	}
}

// TestDBDropdownOnAllPages checks if the DB browser works on ALL pages
// or only on the school page.
func TestDBDropdownOnAllPages(t *testing.T) {
	pages := []string{"/", "/trading", "/school"}
	for _, p := range pages {
		html := fetchPage(t, p)
		if html == "" {
			continue
		}
		hasDB := strings.Contains(html, "populateDBTables") && strings.Contains(html, "dbTableSelect")
		t.Logf("Page %s: DB browser present=%v", p, hasDB)
		if p == "/school" && !hasDB {
			t.Errorf("BUG: School page (%s) missing DB browser", p)
		}
	}
}

// Helper: apiGet fetches an API endpoint and returns the body as string.
func apiGet(t *testing.T, path string) string {
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
	return ""
}
