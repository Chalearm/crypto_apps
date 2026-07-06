package integration

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// regression_test.go — comprehensive regression suite

var portfolioCriticalHandlers = []string{
	"toggleBalancePrivacy", "unlockWallet", "toggleChainPanel",
	"toggleEditMode", "openTokenEditor", "showAddTokenFields",
	"addTokenSubmit", "cancelAddToken", "cancelEditMode",
	"saveTokenEdits", "markTokenDeleted", "saveChain",
	"cancelChainAdd", "renderAssetRows", "refreshAssetPanel",
	"checkChainSelection", "togglePortDetail", "fetchLiveBalance",
	"fetchBTCPrice", "computeChainBalances", "updateDropdownOptionLabels",
}

func TestReg_AllOnclickHandlersHaveFunctions(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	failures := 0
	for _, fn := range portfolioCriticalHandlers {
		fnDef2 := "function " + fn + "("
		if !strings.Contains(html, fnDef2) {
			t.Errorf("FAIL: '%s()' onclick handler has NO function definition on portfolio page", fn)
			failures++
		}
	}
	t.Logf("%d critical handlers checked, %d missing", len(portfolioCriticalHandlers), failures)
}
func TestReg_NoStaleCodePatterns(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	failures := 0

	// CRITICAL: No duplicate JS function definitions (syntax error kills ALL JS)
	funcNames := []string{"format9Decimal", "saveTokenEdits", "computeChainBalances",
		"updateDropdownOptionLabels", "renderAssetRows", "toggleEditMode",
		"unlockWallet", "toggleBalancePrivacy", "toggleChainPanel",
		"fetchBTCPrice", "fetchLiveBalance"}
	for _, fn := range funcNames {
		count := strings.Count(html, "function "+fn+"(")
		if count > 1 {
			t.Errorf("FAIL: function '%s' defined %d times — DUPLICATE JS SYNTAX ERROR! This breaks ALL JavaScript.", fn, count)
			failures++
		} else if count == 0 {
			t.Errorf("FAIL: function '%s' NOT defined", fn)
			failures++
		}
	}
	if failures == 0 {
		t.Logf("PASS: No duplicate JS function definitions (%d checked)", len(funcNames))
	}

	if strings.Contains(html, "if(d.assets){ assetsData = d.assets;") {
		t.Errorf("FAIL: Stale pattern 'if(d.assets){ assetsData = d.assets;'")
	}
	if strings.Contains(html, "if (a.deleted) continue") &&
		!strings.Contains(html, "if(deletedTokens[i]) continue") {
		t.Errorf("FAIL: Stale pattern 'if (a.deleted) continue'")
	}
	if strings.Contains(html, `id="chainEditor"`) && strings.Contains(html, `position:fixed`) {
		t.Errorf("FAIL: chainEditor modal still present")
	}
	flbIdx := strings.Index(html, "function fetchLiveBalance")
	if flbIdx >= 0 {
		flbBody := html[flbIdx : flbIdx+150]
		if !strings.Contains(flbBody, "_accountId") &&
		   strings.Contains(flbBody, "fetch('/api/balance')") {
			t.Errorf("FAIL: fetchLiveBalance missing unlock guard")
		}
	}
	if !strings.Contains(html, "setInterval(fetchBTCPrice") {
		t.Errorf("FAIL: BTC auto-refresh missing")
	}
	if !strings.Contains(html, "setInterval(fetchLiveBalance") {
		t.Errorf("FAIL: Balance auto-refresh missing")
	}
	if strings.Contains(html, "function addTokenSubmit") {
		fnIdx := strings.Index(html, "function addTokenSubmit")
		fnBody := html[fnIdx : fnIdx+400]
		if !strings.Contains(fnBody, "0x[a-fA-F0-9]") && !strings.Contains(fnBody, "/^0x") {
			t.Errorf("FAIL: addTokenSubmit missing hex validation")
		}
	}
}

func TestReg_NoHardcodedPrivateKeys(t *testing.T) {
	dirs := []string{"../integration", "../webui", "../apps", "../school", "../trading",
		"../governance", "../infra", "../config", "../testdaemon", "../"}

	// Read the real key from config.env (never hardcoded)
	fullKey := pkEnv(t)
	if fullKey == "" {
		t.Skip("PRIVATE_KEY not found — cannot scan")
	}

	found := []string{}
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil { continue }
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".go") &&
			   !strings.HasSuffix(e.Name(), ".py") { continue }
			if e.Name() == "pk_helper.go" { continue }

			data, err := os.ReadFile(dir + "/" + e.Name())
			if err != nil { continue }
			if strings.Contains(string(data), fullKey) {
				found = append(found, dir+"/"+e.Name())
			}
		}
	}

	if len(found) > 0 {
		t.Errorf("FAIL: %d file(s) contain hardcoded private key: %v\n  Only config.env should have the key.", len(found), found)
	} else {
		t.Logf("PASS: No hardcoded private keys in %d directories", len(dirs))
	}
}

func TestReg_DaemonHealth(t *testing.T) {
	resp, err := http.Get("http://127.0.0.1:8080/api/daemons")
	if err != nil {
		t.Fatalf("Cannot reach daemons: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Daemons []struct {
			Name    string `json:"Name"`
			Status  string `json:"Status"`
			Message string `json:"Message"`
		} `json:"daemons"`
	}
	json.NewDecoder(resp.Body).Decode(&data)

	failures := 0
	for _, d := range data.Daemons {
		if strings.HasPrefix(d.Name, "integration_test") { continue }
		if d.Status == "starting" {
			t.Logf("  %-15s %-10s %s (transient — acceptable)", d.Name, d.Status, d.Message)
			continue
		}
		if d.Status != "healthy" && d.Status != "pass" {
			t.Errorf("FAIL: %s is %s — %s", d.Name, d.Status, d.Message)
			failures++
		}
		t.Logf("  %-15s %-10s %s", d.Name, d.Status, d.Message)
	}
	if failures == 0 {
		t.Log("PASS: All daemons healthy")
	}
}

func TestReg_FullUnlockBalanceToggleFlow(t *testing.T) {
	// 1) Unlock
	unlockBody := `{"private_key":"` + pkEnv(t) + `"}`
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json", strings.NewReader(unlockBody))
	if err != nil { t.Fatalf("unlock POST: %v", err) }
	defer resp.Body.Close()

	var unlockData map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&unlockData)
	if unlockData["status"] != "ok" {
		t.Fatalf("unlock failed: %v", unlockData)
	}
	t.Log("1. Unlock: OK")

	// 2) Balance after unlock
	time.Sleep(1 * time.Second)
	balResp, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil { t.Fatalf("balance GET: %v", err) }
	defer balResp.Body.Close()

	var balData map[string]interface{}
	json.NewDecoder(balResp.Body).Decode(&balData)

	totalUSD, _ := balData["total_usd"].(float64)
	assets, _ := balData["assets"].([]interface{})
	btcPrice, _ := balData["btc_price"].(float64)
	accountMasked, _ := balData["account_masked"].(string)

	t.Logf("2. Balance: usd=%.6f assets=%d btc=%.0f masked=%s",
		totalUSD, len(assets), btcPrice, accountMasked)

	if accountMasked == "" || accountMasked == "no-account" {
		t.Errorf("FAIL: account_masked is '%s' — unlock did not register", accountMasked)
	}

	// 3) Verify page has toggleBalancePrivacy
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function toggleBalancePrivacy") {
		t.Errorf("FAIL: toggleBalancePrivacy function missing")
	}
	if !strings.Contains(html, "showAllNumbers = !showAllNumbers") {
		t.Errorf("FAIL: toggleBalancePrivacy body incomplete")
	}
	if !strings.Contains(html, "toggleBalancePrivacy()") {
		t.Errorf("FAIL: toggleBalancePrivacy onclick missing from balance display")
	}
	t.Log("3. toggleBalancePrivacy: present and wired")
}

func TestReg_ChainPanelToggleWorks(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function toggleChainPanel") {
		t.Errorf("FAIL: toggleChainPanel missing")
	}
	if !strings.Contains(html, "classList.contains('open')") {
		t.Errorf("FAIL: toggleChainPanel does NOT use classList")
	}
	if !strings.Contains(html, `onclick="toggleChainPanel()"`) {
		t.Errorf("FAIL: balance-interactive-header missing onclick=toggleChainPanel()")
	}
	if !strings.Contains(html, ".chain-panel.open{display:block") {
		t.Errorf("FAIL: .chain-panel.open CSS missing")
	}
	t.Log("PASS: Chain panel toggle wiring correct")
}

func TestReg_NoDuplicateIDs(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	criticalIDs := []string{
		"chainSelect", "chainPanel", "assetRows", "pkInput",
		"balanceAmount", "btcPrice", "btcToggle",
		"editActions", "editOkBtn", "addTokenBtnRow", "addTokenFields",
		"tokTicker", "tokAddr", "chainNameInput", "chainIdInput",
		"chainAddRow",
	}

	failures := 0
	for _, id := range criticalIDs {
		count := strings.Count(html, `id="`+id+`"`)
		if count > 1 {
			t.Errorf("FAIL: id='%s' appears %d times (DUPLICATE!) — clicks target wrong element", id, count)
			failures++
		} else if count == 0 {
			t.Logf("WARN: id='%s' not found (may be optional)", id)
		}
	}
	if failures == 0 {
		t.Log("PASS: No duplicate IDs")
	}
}

func TestReg_TradingDaemonResponds(t *testing.T) {
	// Probe trading directly
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8083")
	if err != nil { t.Skipf("resolve: %v", err); return }
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil { t.Skipf("dial: %v", err); return }
	defer conn.Close()

	conn.Write([]byte("governance:probe:health_check"))
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 2048)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		t.Errorf("FAIL: Trading daemon not responding to UDP health probe: %v\n"+
			"  The dashboard shows 'unhealthy' because health probes fail.\n"+
			"  This is the root cause of 'trading unhealthy' status.", err)
		return
	}
	resp := string(buf[:n])
	t.Logf("Trading UDP response: %s", resp)
	if !strings.Contains(resp, "pong") {
		t.Errorf("FAIL: Trading response '%s' does not contain 'pong'", resp)
	} else {
		t.Log("PASS: Trading daemon responds to health probe")
	}
}
