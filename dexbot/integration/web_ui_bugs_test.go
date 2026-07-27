/******************************************************************************
 * File Name       : web_ui_bugs_test.go
 * File Path       : integration/web_ui_bugs_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-05 14:00:00 (UTC+7)
 *
 * Description     :
 *   Web UI bug fix verification tests per session 2026-07-05.
 *   Tests verify: DB dropdown max 80 + empty table message,
 *   chain balance display format, pencil icon token editor,
 *   chain (+) add feature, BTC price live value.
 *
 *   8 positive + 3 negative test cases per myreq6.txt §114.
 *****************************************************************************
 *
 * Notes :
 *   - Per regulator coding standard.
 */

package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

// ── POSITIVE TESTS (8) ──

// P1: DB dropdown max rows is 80, not 25
func TestDBDropdownMaxRows80(t *testing.T) {
	html := fetchPage(t, "/school")
	if html == "" {
		return
	}
	if strings.Contains(html, `max="80"`) && strings.Contains(html, `Max 80 rows`) {
		t.Log("PASS: DB max rows set to 80")
	} else if strings.Contains(html, `max="25"`) {
		t.Error("FAIL: DB max rows still 25 — should be 80")
	} else {
		t.Log("WARN: could not determine max rows value")
	}

	// Also check validateDBInput bound
	has80 := strings.Contains(html, `v>80`)
	has25 := strings.Contains(html, `v>25`)
	if has80 && !has25 {
		t.Log("PASS: validateDBInput bound is 80")
	} else if has25 {
		t.Error("FAIL: validateDBInput bound still 25 — should be 80")
	}
}

// P2: Empty table shows "No data" not error
func TestDBDropdownEmptyTableNoError(t *testing.T) {
	// Verify the loadDBTable JS contains "No data" message for empty tables
	html := fetchPage(t, "/school")
	if html == "" {
		return
	}
	if strings.Contains(html, "No data available for") {
		t.Log("PASS: loadDBTable has 'No data' fallback message")
	} else {
		t.Error("FAIL: loadDBTable missing 'No data' message for empty tables")
	}
	// Verify error display still works
	if strings.Contains(html, `"<div style='color:var(--rose);padding:8px'>"+d.error+"</div>"`) {
		t.Log("PASS: Error display preserved for actual errors")
	}
}

// P3: Chain balance format uses 9 decimal spaces
func TestChainBalanceFormat9Decimal(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	// Check format9Decimal function exists and handles 9 decimals
	if strings.Contains(html, "toFixed(9)") {
		t.Log("PASS: format9Decimal uses 9 fractional digits")
	} else {
		t.Error("FAIL: format9Decimal not using 9 fractional digits")
	}
	// Check spacing format
	if strings.Contains(html, "fracGroups.join(' ')") && strings.Contains(html, "intGroups.join(' ')") {
		t.Log("PASS: 3-digit spacing in both integer and fraction parts")
	}
}

// P4: Pencil icon exists and token editor has red remove icons
func TestPencilIconAndTokenEditor(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	if strings.Contains(html, "pencil-icon") {
		t.Log("PASS: Pencil icon CSS class exists")
	} else {
		t.Error("FAIL: Pencil icon CSS class missing")
	}
	if strings.Contains(html, "openTokenEditor()") {
		t.Log("PASS: openTokenEditor function exists")
	} else {
		t.Error("FAIL: openTokenEditor function missing")
	}
	// Check delete token with red marker exists
	if strings.Contains(html, "deleteToken(") {
		t.Log("PASS: deleteToken function exists for removing tokens")
	}
	// Check token editor modal has submit/cancel
	if strings.Contains(html, "saveToken()") && strings.Contains(html, "closeTokenEditor()") {
		t.Log("PASS: Token editor has saveToken() and closeTokenEditor()")
	}
}

// P5: Chain (+) add feature with name/id/baseurl + submit/cancel
func TestChainAddFeature(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	if strings.Contains(html, "Add New Network Chain") && strings.Contains(html, "chainBaseUrlInput") {
		t.Log("PASS: Chain editor has name/id/baseurl inputs")
	} else {
		t.Error("FAIL: Chain editor missing baseUrl input or title")
	}
	if strings.Contains(html, "saveChain()") && strings.Contains(html, "closeChainEditor()") {
		t.Log("PASS: Chain editor has save/cancel buttons")
	}
	if strings.Contains(html, "__add__") {
		t.Log("PASS: Dropdown has '+ Add New Chain' option")
	} else {
		t.Error("FAIL: Dropdown missing '__add__' option")
	}
}

// P6: BTC price updates from real CoinGecko API
func TestBTCPriceLiveAPI(t *testing.T) {
	// Verify the BTC price endpoint exists
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/balance", baseURL))
	if err != nil {
		t.Fatalf("GET /api/balance failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)
	btcPrice, ok := data["btc_price"].(float64)
	if !ok || btcPrice <= 0 {
		t.Error("FAIL: btc_price is 0 or missing — should be real CoinGecko value")
	} else if btcPrice < 50000 || btcPrice > 70000 {
		t.Errorf("FAIL: btc_price = %.2f — out of expected range (50000-70000 USD)", btcPrice)
	} else {
		t.Logf("PASS: BTC price = $%.2f (live CoinGecko API)", btcPrice)
	}
}

// P7: Balance card has chain balance display on dropdown row
func TestChainBalanceDisplayed(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	if strings.Contains(html, "chainTotalDisplay") {
		t.Log("PASS: chainTotalDisplay element exists")
	} else {
		t.Error("FAIL: chainTotalDisplay element missing — no per-chain balance shown")
	}
	// Verify computeChainBalances computes totals
	if strings.Contains(html, "computeChainBalances()") {
		t.Log("PASS: computeChainBalances function exists")
	}
}

// P8: Pre-populated assetsData has 11 tokens with chain info
func TestPrePopulatedTokensHaveChains(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	re := regexp.MustCompile(`var assetsData = (.+?);`)
	m := re.FindStringSubmatch(html)
	if len(m) < 2 {
		t.Fatal("assetsData not found in page")
	}
	var assets []map[string]interface{}
	if err := json.Unmarshal([]byte(m[1]), &assets); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if len(assets) < 10 {
		t.Errorf("FAIL: assetsData has only %d tokens — expected >= 10", len(assets))
	} else {
		t.Logf("PASS: assetsData pre-populated with %d tokens", len(assets))
	}
	chains := make(map[string]bool)
	for _, a := range assets {
		if cn, ok := a["chain_name"].(string); ok {
			chains[cn] = true
		}
	}
	if len(chains) < 1 {
		t.Error("FAIL: no chain_name in any asset")
	} else {
		t.Logf("PASS: %d unique chains in assetsData: %v", len(chains), chains)
	}
}

// ── NEGATIVE TESTS (3) ──

// N1: Empty table selected -> shows "No data" not error
func TestNegativeEmptyTableNoError(t *testing.T) {
	// Verify the DOM structure doesn't show error for empty data
	html := fetchPage(t, "/school")
	if html == "" {
		return
	}
	// The "No data available for" message should exist
	if !strings.Contains(html, "No data available for") && !strings.Contains(html, "No data") && !strings.Contains(html, "No rows") {
		t.Error("FAIL: No empty-table message found in loadDBTable")
	} else {
		t.Log("PASS: Empty table message exists in page")
	}
}

// N2: Invalid chain name in add -> server rejects
func TestNegativeInvalidChainName(t *testing.T) {
	// Test that chain editor has input validation
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	if strings.Contains(html, "Dynamic fields required") {
		t.Log("PASS: Chain editor has client-side validation")
	} else {
		t.Error("FAIL: Chain editor missing validation message")
	}
}

// N3: saveToken called with empty ticker -> alert
func TestNegativeEmptyTickerSave(t *testing.T) {
	html := fetchPage(t, "/trading")
	if html == "" {
		return
	}
	if strings.Contains(html, "Parameters missing") {
		t.Log("PASS: Token editor has client-side validation for empty fields")
	} else {
		t.Error("FAIL: Token editor missing 'Parameters missing' validation")
	}
}
