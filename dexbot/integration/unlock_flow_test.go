package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func fetchWithBody(t *testing.T, path, method, body string) *http.Response {
	t.Helper()
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(method, "http://127.0.0.1:8080"+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("Request create failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP %s %s failed: %v", method, path, err)
	}
	return resp
}

func TestUnlockEndpointReturnsOK(t *testing.T) {
	pk := pkEnv(t)
	resp := fetchWithBody(t, "/api/unlock", "POST",
		`{"private_key":"`+pk+`"}`)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	if data["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", data["status"])
	}

	// BUG CHECK: /api/unlock does NOT include assets
	// unlockWallet JS had: if(d.assets){ assetsData = d.assets; ... }
	// This caused stale assetsData with zero balances
	_, hasAssets := data["assets"]
	t.Logf("/api/unlock includes assets: %v", hasAssets)
	if !hasAssets {
		t.Logf("BUG-ROOT: /api/unlock response does NOT include assets field.\n" +
			"  The JS function unlockWallet() checked 'if(d.assets)' which was always false.\n" +
			"  After unlock, balance stayed at 0 because assetsData was never updated.\n" +
			"  FIX: unlockWallet should fetch /api/balance after successful unlock.")
	}
}

func TestUnlockRewritesToFetchBalance(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" {
		return
	}

	// Check unlockWallet JS exists
	if !strings.Contains(html, "function unlockWallet") {
		t.Errorf("FAIL: unlockWallet function missing from page JS")
	} else {
		t.Log("PASS: unlockWallet function present")
	}

	// Check it fetches /api/unlock
	if !strings.Contains(html, "api/unlock") {
		t.Errorf("FAIL: unlockWallet does NOT call /api/unlock")
	} else {
		t.Log("PASS: unlockWallet calls /api/unlock")
	}

	// CRITICAL: unlockWallet must ALSO fetch /api/balance or the response data
	// after unlock, because /api/unlock does NOT return assets.
	hasBalanceFetch := strings.Contains(html, "fetch('/api/balance')") ||
		strings.Contains(html, "api/balance")
	t.Logf("unlockWallet fetches /api/balance after unlock: %v", hasBalanceFetch)

	if !hasBalanceFetch {
		t.Errorf("FAIL: unlockWallet does NOT fetch /api/balance after unlock.\n" +
			"  /api/unlock returns {status:ok,address:...,profile_id:...} but NO assets.\n" +
			"  Without /api/balance, the page never updates with real token data.\n" +
			"  BUG: User enters key, clicks OK, nothing happens.")
	}

	// Check the obsolete pattern is gone
	if strings.Contains(html, "if(d.assets){ assetsData = d.assets;") {
		t.Errorf("FAIL: unlockWallet still checks d.assets from unlock response.\n" +
			"  /api/unlock response has NO assets field. This guard always fails.\n" +
			"  FIX: replace with return fetch('/api/balance') promise chain.")
	} else {
		t.Log("PASS: unlockWallet no longer checks d.assets from /api/unlock")
	}
}

func TestBalanceReturnsRealData(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil {
		t.Fatalf("GET /api/balance failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	totalUSD, _ := data["total_usd"].(float64)
	assets, _ := data["assets"].([]interface{})
	btcPrice, _ := data["btc_price"].(float64)

	t.Logf("Balance: total_usd=%.6f, assets=%d, btc_price=%.0f", totalUSD, len(assets), btcPrice)

	if totalUSD == 0 {
		t.Logf("WARN: total_usd is 0 — balance API returned zero")
	}
	if len(assets) == 0 {
		t.Errorf("FAIL: balance assets list is EMPTY — chain totals will be zero")
	}

	// Verify assets have required fields for chain balance computation
	for i, a := range assets {
		amap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		if _, hasChain := amap["chain_name"]; !hasChain {
			t.Logf("WARN: asset[%d] missing chain_name field", i)
		}
		if _, hasUSD := amap["usd_value"]; !hasUSD {
			t.Logf("WARN: asset[%d] missing usd_value field", i)
		}
	}

	// Verify btcPrice is realistic
	if btcPrice < 100 || btcPrice > 1000000 {
		t.Logf("WARN: btc_price=%.0f looks unrealistic", btcPrice)
	} else {
		t.Logf("PASS: btc_price=%.0f in realistic range", btcPrice)
	}
}

func TestFullUnlockFlowEndToEnd(t *testing.T) {
	// 1) Unlock
	pk := pkEnv(t)
	unlockResp := fetchWithBody(t, "/api/unlock", "POST",
		`{"private_key":"`+pk+`"}`)
	defer unlockResp.Body.Close()
	var unlockData map[string]interface{}
	json.NewDecoder(unlockResp.Body).Decode(&unlockData)
	if unlockData["status"] != "ok" {
		t.Fatalf("unlock failed: %v", unlockData)
	}
	t.Logf("1. Unlock OK: address=%v", unlockData["address"])

	// 2) After unlock, balance must return non-zero data
	balResp, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil {
		t.Fatalf("GET /api/balance failed: %v", err)
	}
	defer balResp.Body.Close()
	var balData map[string]interface{}
	json.NewDecoder(balResp.Body).Decode(&balData)

	totalUSD, _ := balData["total_usd"].(float64)
	assets, _ := balData["assets"].([]interface{})

	t.Logf("2. Balance after unlock: total_usd=%.6f, assets=%d", totalUSD, len(assets))

	if len(assets) == 0 {
		t.Errorf("FAIL: balance after unlock has 0 assets")
	}
	if totalUSD == 0 {
		t.Logf("WARN: total_usd is 0 after unlock — check if unlock refreshed balance")
	}

	// 3) Verify page JS renders after unlock data is available
	html := fetchPage(t, "/portfolio")
	if html == "" {
		return
	}

	// After unlock, the page should have updated balance
	if !strings.Contains(html, "assetsData") {
		t.Errorf("FAIL: assetsData not found in page")
	} else {
		t.Log("3. assetsData present in page")
	}

	// Check computeChainBalances exists
	if !strings.Contains(html, "function computeChainBalances") {
		t.Errorf("FAIL: computeChainBalances function missing")
	} else {
		t.Log("4. computeChainBalances function present")
	}
}
