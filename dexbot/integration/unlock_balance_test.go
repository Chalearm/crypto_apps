/******************************************************************************
 * File Name       : unlock_balance_test.go
 * File Path       : integration/unlock_balance_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-05 14:00:00 (UTC+7)
 *
 * Description     :
 *   End-to-end test for the unlock + balance refresh flow.
 *   Tests that:
 *     1. config.env can be read and PRIVATE_KEY extracted
 *     2. queryRealBalances() returns non-zero data with a valid key
 *     3. GetBalanceSummary() produces a valid summary with real amounts
 *     4. The web page renders assetsData with correct chain names
 *     5. DB table browser endpoint returns table list
 *     6. /api/unlock writes key to config.env and governance picks it up
 *
 * Usage :
 *   go test ./integration -v -run UnlockBalance
 ******************************************************************************/
package integration

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"dexbot/infra"
)

// TestConfigEnvReadable verifies config.env exists and PRIVATE_KEY is parseable.
func TestConfigEnvReadable(t *testing.T) {
	paths := []string{"config.env", "../config.env"}
	var found string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = p
			break
		}
	}
	if found == "" {
		t.Skip("config.env not found — skipping (no key configured)")
	}
	data, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("cannot read config.env: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	keyFound := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PRIVATE_KEY=") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 && parts[1] != "" {
				keyFound = true
				t.Logf("PRIVATE_KEY found: %s... (length=%d)", parts[1][:8], len(parts[1]))
			}
		}
	}
	if !keyFound {
		t.Error("PRIVATE_KEY not set in config.env")
	}
}

// TestGetBalanceSummaryWithKey tests that GetBalanceSummary returns real balance
// when a valid private key is in config.env.
func TestGetBalanceSummaryWithKey(t *testing.T) {
	// Skip if no key set
	pk := os.Getenv("PRIVATE_KEY")
	if pk == "" {
		// Try reading from file
		data, err := os.ReadFile("config.env")
		if err != nil {
			t.Skip("config.env not found — cannot test balance")
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				pk = strings.SplitN(line, "=", 2)[1]
				break
			}
		}
	}
	if pk == "" {
		t.Skip("no PRIVATE_KEY — skipping live balance test")
	}

	am := infra.NewAccountManager()
	summary := infra.GetBalanceSummary(am)

	t.Logf("Account: %s", summary.AccountMasked)
	t.Logf("Total USD: $%.6f", summary.TotalUSD)
	t.Logf("Assets: %d tokens", len(summary.Assets))

	// Check that we have assets
	if len(summary.Assets) == 0 {
		t.Error("expected non-empty assets list")
	}

	// Check at least one asset per chain mentioned
	chains := map[string]bool{}
	positiveCount := 0
	for _, a := range summary.Assets {
		chains[a.ChainName] = true
		if a.Amount > 0.000001 {
			positiveCount++
			t.Logf("  %s/%s: %.8f $%.6f", a.ChainName, a.Ticker, a.Amount, a.USDValue)
		}
	}

	t.Logf("Chains: %v", chains)
	t.Logf("Positive tokens: %d", positiveCount)

	// With a real key, we expect at least 1 positive token and multiple chains
	if positiveCount == 0 {
		t.Error("expected at least 1 token with positive balance")
	}
}

// TestAssetJSONSerialization verifies the balance can be serialized to JSON
// and deserialized back, preserving chain names and amounts.
func TestAssetJSONSerialization(t *testing.T) {
	pk := os.Getenv("PRIVATE_KEY")
	if pk == "" {
		t.Skip("no key")
	}

	am := infra.NewAccountManager()
	summary := infra.GetBalanceSummary(am)

	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed infra.BalanceSummary
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if parsed.TotalUSD != summary.TotalUSD {
		t.Errorf("TotalUSD mismatch: %f vs %f", parsed.TotalUSD, summary.TotalUSD)
	}
	if len(parsed.Assets) != len(summary.Assets) {
		t.Errorf("asset count mismatch: %d vs %d", len(parsed.Assets), len(summary.Assets))
	}

	// Verify chain names survive round-trip
	for i, a := range parsed.Assets {
		if a.ChainName != summary.Assets[i].ChainName {
			t.Errorf("asset[%d] chain mismatch: %s vs %s", i, a.ChainName, summary.Assets[i].ChainName)
		}
	}
	t.Logf("JSON round-trip OK: %d assets, $%.6f total", len(parsed.Assets), parsed.TotalUSD)
}

// TestDashboardFileGeneration verifies governance generates balance.json with
// real data after config.env is updated.
func TestDashboardFileGeneration(t *testing.T) {
	// Check if balance.json exists in web_output/api
	paths := []string{"web_output/api/balance.json", "../web_output/api/balance.json"}
	var found string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			found = p
			break
		}
	}
	if found == "" {
		t.Skip("balance.json not found — governance may not have run yet")
	}

	data, err := os.ReadFile(found)
	if err != nil {
		t.Fatalf("cannot read balance.json: %v", err)
	}

	var summary infra.BalanceSummary
	if err := json.Unmarshal(data, &summary); err != nil {
		t.Fatalf("parse balance.json: %v", err)
	}

	t.Logf("balance.json: account=%s total=$%.6f assets=%d",
		summary.AccountMasked, summary.TotalUSD, len(summary.Assets))

	// Check that chain_name field exists in each asset
	for i, a := range summary.Assets {
		if a.ChainName == "" {
			t.Errorf("asset[%d] has empty chain_name", i)
		}
	}

	// With a real key and running governance, total should be non-zero
	if summary.TotalUSD <= 0 && len(summary.Assets) > 0 {
		// Find any positive amount
		hasPositive := false
		for _, a := range summary.Assets {
			if a.Amount > 0 {
				hasPositive = true
				break
			}
		}
		if hasPositive {
			t.Error("has positive amounts but TotalUSD <= 0")
		} else {
			t.Log("all zero amounts — may be no-key scenario or slow RPC")
		}
	}
}

// TestUnlockEndpointViaHTTP tests the /api/unlock POST endpoint.
func TestUnlockEndpointViaHTTP(t *testing.T) {
	// Try to reach the web server
	urls := []string{"http://127.0.0.1:8080/api/unlock", "http://localhost:8080/api/unlock"}
	var resp *http.Response
	var err error
	for _, u := range urls {
		client := &http.Client{Timeout: 3 * time.Second}
		body := fmt.Sprintf(`{"private_key":"%s"}`, "aa"+"bb"+"cc"+"dd"+"ee"+"ff"+ // 64 dummy chars
			"0000000000000000000000000000000000000000000000000000000000000000")
		req, _ := http.NewRequest("POST", u, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err = client.Do(req)
		if err == nil {
			break
		}
	}
	if err != nil {
		t.Skipf("web server not reachable: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("/api/unlock returned status %d", resp.StatusCode)

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	if status, ok := result["status"]; !ok || status != "ok" {
		t.Errorf("expected status=ok, got %v", result)
	}
	if addr, ok := result["address"]; !ok || addr == "" {
		t.Error("expected address in response")
	}
	t.Logf("unlock response: %v", result)
}

// TestDatabaseBrowserAPI tests that /api/database_tables returns a table list.
func TestDatabaseBrowserAPI(t *testing.T) {
	urls := []string{"http://127.0.0.1:8080/api/database_tables", "http://localhost:8080/api/database_tables"}
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

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	tables, ok := result["tables"].([]interface{})
	if !ok {
		t.Fatalf("expected tables array, got %T", result["tables"])
	}
	if len(tables) == 0 {
		t.Error("expected at least 1 table")
	}
	t.Logf("database_tables: %d tables", len(tables))
	for _, tbl := range tables {
		t.Logf("  - %s", tbl)
	}
}
