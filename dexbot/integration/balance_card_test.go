package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// balance_card_test.go — validates full balance card behavior
// 1) enter private key → click OK → balance data loads
// 2) toggleBalancePrivacy → ****** ↔ real digits
// 3) chainPanel expands/shows data after unlock

func TestBalanceCard_UnlockLoadsRealBalance(t *testing.T) {
	// Step 0: verify /api/balance returns default data (no key)
	bal0, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil { t.Fatalf("balance GET: %v", err) }
	defer bal0.Body.Close()
	var d0 map[string]interface{}
	json.NewDecoder(bal0.Body).Decode(&d0)
	masked0, _ := d0["account_masked"].(string)
	t.Logf("Before unlock: masked=%s usd=%.6f", masked0, d0["total_usd"])

	// Step 1: unlock with real private key
	pk := pkEnv(t)
	unlockResp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+pk+`"}`))
	if err != nil { t.Fatalf("unlock POST: %v", err) }
	defer unlockResp.Body.Close()

	var ud map[string]interface{}
	json.NewDecoder(unlockResp.Body).Decode(&ud)
	if ud["status"] != "ok" {
		t.Fatalf("unlock failed: %v", ud)
	}
	t.Logf("Unlock OK: address=%v profile_exists=%v", ud["address"], ud["profile_exists"])

	// Step 2: wait for governance to regenerate balance.json
	time.Sleep(2 * time.Second)

	// Step 3: verify /api/balance now returns real data
	bal1, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil { t.Fatalf("balance GET after unlock: %v", err) }
	defer bal1.Body.Close()
	var d1 map[string]interface{}
	json.NewDecoder(bal1.Body).Decode(&d1)
	masked1, _ := d1["account_masked"].(string)
	totalUSD1, _ := d1["total_usd"].(float64)
	assets1, _ := d1["assets"].([]interface{})

	t.Logf("After unlock: masked=%s usd=%.6f assets=%d", masked1, totalUSD1, len(assets1))

	// CRITICAL: balance must show real masked account, not "no-account" or empty
	if masked1 == "" || masked1 == "no-account" {
		t.Errorf("FAIL: account_masked is '%s' after unlock — should be masked PK like 'aabbcc******'.\n"+
			"  BALANCE CARD BUG: user enters PK, clicks OK, but balance never shows real data.\n"+
			"  Root cause: /api/balance returns _default_balance (no-account) even after /api/unlock.\n"+
			"  FIX: serve.py must re-read PRIVATE_KEY from env/os.environ after unlock.", masked1)
	}

	// balance must show >0 assets
	if len(assets1) == 0 {
		t.Errorf("FAIL: balance returns 0 assets after unlock")
	}
}

func TestBalanceCard_TogglePrivacyWorks(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) toggleBalancePrivacy must be a real function with full body
	if !strings.Contains(html, "function toggleBalancePrivacy") {
		t.Errorf("FAIL: toggleBalancePrivacy function missing from page")
		return
	}
	t.Log("PASS: toggleBalancePrivacy function exists")

	// 2) It must toggle showAllNumbers
	if !strings.Contains(html, "showAllNumbers = !showAllNumbers") {
		t.Errorf("FAIL: toggleBalancePrivacy doesn't toggle showAllNumbers — can't flip privacy")
	}

	// 3) It must call renderAssetRows to update DOM
	if !strings.Contains(html, "renderAssetRows()") {
		t.Errorf("FAIL: toggleBalancePrivacy doesn't call renderAssetRows — no DOM update on toggle")
	}
	t.Log("PASS: toggleBalancePrivacy calls renderAssetRows")

	// 4) balanceAmount element must exist and show initial value
	if !strings.Contains(html, `id="balanceAmount"`) {
		t.Errorf("FAIL: balanceAmount element missing — visible balance line absent")
	} else {
		t.Log("PASS: balanceAmount span exists")
	}

	// 5) showAllNumbers guard must apply to BOTH amount and USD columns
	// Check renderAssetRows uses showAllNumbers for amount
	amountProtected := strings.Contains(html, "showAllNumbers ? format9Decimal(a.amount")
	t.Logf("amount column has showAllNumbers guard: %v", amountProtected)
	if !amountProtected {
		t.Errorf("FAIL: asset amount column NOT protected by showAllNumbers —\n"+
			"  privacy toggle only hides USD, amounts still visible")
	}

	// 6) balanceAmount uses showAllNumbers
	balProtected := strings.Contains(html, "showAllNumbers ? globalSym")
	t.Logf("balanceAmount uses showAllNumbers: %v", balProtected)
	if !balProtected {
		t.Errorf("FAIL: balanceAmount doesn't use showAllNumbers —\n"+
			"  main balance always shows real digits regardless of privacy toggle")
	}
}

func TestBalanceCard_ChainPanelExpandsAfterUnlock(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) toggleChainPanel must exist and open chain-panel
	if !strings.Contains(html, "function toggleChainPanel") {
		t.Errorf("FAIL: toggleChainPanel function missing — clicking header does nothing")
		return
	}
	t.Log("PASS: toggleChainPanel function exists")

	// 2) Opening must trigger refreshAssetPanel to load token data
	if !strings.Contains(html, "refreshAssetPanel()") {
		t.Errorf("FAIL: toggleChainPanel doesn't call refreshAssetPanel — no token refresh")
	} else {
		t.Log("PASS: toggleChainPanel calls refreshAssetPanel on open")
	}

	// 3) chain-panel CSS must toggle via classList (not style.display)
	if !strings.Contains(html, "classList.contains") || !strings.Contains(html, "classList.toggle") {
		t.Errorf("FAIL: toggleChainPanel doesn't use classList — CSS .chain-panel.open may not work")
	}

	// 4) chainSelect dropdown must exist with options
	if !strings.Contains(html, `id="chainSelect"`) {
		t.Errorf("FAIL: chainSelect dropdown missing — no chain selection")
	} else {
		t.Log("PASS: chainSelect dropdown exists")
	}

	// 5) assetRows div must exist for token display
	if !strings.Contains(html, `id="assetRows"`) {
		t.Errorf("FAIL: assetRows div missing — token list won't render")
	} else {
		t.Log("PASS: assetRows div exists")
	}

	// 6) updateDropdownOptionLabels must show per-chain totals
	if !strings.Contains(html, "chainTotals[opt.value]") &&
		!strings.Contains(html, "var chainUSD") {
		t.Errorf("FAIL: dropdown labels don't compute per-chain totals —\n"+
			"  chain dropdown shows only names, no balances")
	}
	t.Log("PASS: dropdown labels compute per-chain USD")
}

func TestBalanceCard_RenderAssetRowsShowsZeroBalanceTokens(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) renderAssetRows must exist
	if !strings.Contains(html, "function renderAssetRows") {
		t.Errorf("FAIL: renderAssetRows function missing")
		return
	}
	t.Log("PASS: renderAssetRows function exists")

	// 2) Must filter by selectedChain
	if !strings.Contains(html, "chain_name !== selectedChain") &&
		!strings.Contains(html, "chain_name!==selectedChain") {
		t.Errorf("FAIL: renderAssetRows doesn't filter by selected chain")
	}

	// 3) "Show all tokens" checkbox must exist to show zero-balance tokens
	if !strings.Contains(html, `showAllTokens`) {
		t.Errorf("FAIL: showAllTokens checkbox missing — zero-balance tokens invisible")
	}

	// 4) USD column must use showAllNumbers guard
	if !strings.Contains(html, "showAllNumbers ? sym") {
		t.Errorf("FAIL: USD column not protected by showAllNumbers")
	}

	// 5) The showAllNumbers initial value must be false (privacy ON)
	if !strings.Contains(html, "var showAllNumbers = false") {
		t.Errorf("FAIL: showAllNumbers doesn't default to false — privacy off at page load")
	} else {
		t.Log("PASS: showAllNumbers defaults to false (privacy ON)")
	}
}

func TestBalanceCard_APIReturnsAssetsArray(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil { t.Fatalf("balance GET: %v", err) }
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	assets, ok := data["assets"].([]interface{})
	if !ok || assets == nil {
		t.Errorf("FAIL: /api/balance assets is nil or wrong type.\n"+
			"  JS code checks: if(bd && bd.assets){...}\n"+
			"  If assets is nil/null, the check fails and assetsData never updates.\n"+
			"  This is the ROOT CAUSE: balance.json has 'assets': null\n"+
			"  FIX: governance refreshDashboard must populate Assets with token defaults.")
		return
	}
	if len(assets) == 0 {
		t.Errorf("FAIL: /api/balance returns 0 assets — assetsData stays empty")
	}
	t.Logf("PASS: /api/balance returns %d assets", len(assets))

	// Each asset must have chain_name for chain filtering
	for i, a := range assets {
		amap, ok := a.(map[string]interface{})
		if !ok { continue }
		if cn, hasCN := amap["chain_name"]; !hasCN || cn == nil || cn == "" {
			t.Logf("WARN: asset[%d] missing chain_name — chain filtering may not work", i)
		}
	}

	// Verify balance.json on disk also has real assets (not null)
	// This is what governance writes every 10 seconds
	if data, err := os.ReadFile("web_output/api/balance.json"); err == nil {
		var diskData map[string]interface{}
		json.Unmarshal(data, &diskData)
		diskAssets := diskData["assets"]
		if diskAssets == nil {
			t.Errorf("FAIL: web_output/api/balance.json has assets: null.\n"+
				"  Governance refreshDashboard wrote null assets because GetBalanceSummary\n"+
				"  returned empty summary (BSC RPC failed). serve.py reads this file and\n"+
				"  falls through to _default_balance(). All tokens have zero amounts.\n"+
				"  FIX: governance must populate Assets with token registry defaults\n"+
				"  when real balance fetch fails.")
		} else {
			t.Log("PASS: balance.json has non-null assets on disk")
		}
	}
}
