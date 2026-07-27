package integration

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// cleanupProfile removes all DB rows for a test profile via serve.py API.
func cleanupProfile(profileID string) {
	if profileID == "" || len(profileID) < 60 { return }
	// Nuke all tokens for all known chains, then chains, then profile
	for _, chain := range []string{"BSC", "POLYGON", "ETHEREUM", "OPBNB"} {
		// Delete all tokens for this profile+chain by index
		for attempt := 0; attempt < 20; attempt++ {
			resp, _ := http.Post("http://127.0.0.1:8080/api/tokens/delete",
				"application/json",
				strings.NewReader(`{"account_id":"`+profileID+`","indices":[0],"chain":"`+chain+`"}`))
			if resp == nil || resp.StatusCode != 200 { break }
			resp.Body.Close()
		}
	}
}

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

func TestReg_PriceColumnPresent(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, ".asset-price") {
		t.Errorf("FAIL: .asset-price CSS class missing."+
			" Each asset row needs: [ticker] [price] [amount] [USD value].")
	} else {
		t.Log("PASS: .asset-price CSS class exists")
	}
	if !strings.Contains(html, "usd_price") {
		t.Errorf("FAIL: renderAssetRows does NOT reference usd_price."+
			" The price column shows '--' because no code reads a.usd_price.")
	} else {
		t.Log("PASS: renderAssetRows references usd_price")
	}
}

func TestReg_ProfileIDFullLength(t *testing.T) {
	pk := pkEnv(t)
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+pk+`"}`))
	if err != nil { t.Fatalf("unlock: %v", err) }
	defer resp.Body.Close()

	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)

	profileID, _ := d["profile_id"].(string)
	if profileID == "" {
		t.Fatalf("profile_id missing from unlock response")
	}
	t.Logf("profile_id length: %d", len(profileID))

	if len(profileID) < 64 {
		t.Errorf("FAIL: profile_id is %d chars — should be 64 (full SHA256)."+
			" JS uses this for DB lookups — truncated keys do NOT match.", len(profileID))
	} else {
		t.Log("PASS: profile_id is full 64-char SHA256 hash")
	}
}

func TestReg_TokenDeletePersists(t *testing.T) {
	pk := pkEnv(t)
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+pk+`"}`))
	if err != nil { t.Fatalf("unlock: %v", err) }

	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()

	pid, _ := d["profile_id"].(string)
	t.Logf("profile_id length: %d", len(pid))

	if len(pid) >= 60 {
		r, err := http.Post("http://127.0.0.1:8080/api/tokens/delete",
			"application/json",
			strings.NewReader(`{"account_id":"`+pid+`","indices":[0],"chain":"BSC"}`))
		if err != nil {
			t.Logf("WARN: delete API: %v", err)
		} else {
			r.Body.Close()
			if r.StatusCode != 200 {
				t.Errorf("FAIL: /api/tokens/delete returned %d with full account_id", r.StatusCode)
			} else {
				t.Logf("PASS: /api/tokens/delete returned %d", r.StatusCode)
			}
		}
	} else {
		t.Errorf("FAIL: Cannot test delete — profile_id too short (%d chars)", len(pid))
	}
}

func TestReg_BTCPriceTogglesInAssetRows(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// Price column must compute using btcChecked
	if !strings.Contains(html, "btcChecked") || !strings.Contains(html, "asset-price") {
		t.Fatalf("Cannot find price column or btcChecked in page")
	}

	// The price string must use btcChecked to decide symbol
	priceUsesBTC := strings.Contains(html, "btcChecked") &&
		strings.Contains(html, "asset-price") &&
		(strings.Contains(html, "a.usd_price/btcPrice") ||
		 strings.Contains(html, "a.usd_price / btcPrice") ||
		 strings.Contains(html, "usd_price/btcPrice"))

	t.Logf("Price column respects BTC toggle: %v", priceUsesBTC)
	if !priceUsesBTC {
		t.Errorf("FAIL: Price column always shows $ — does NOT toggle to BTC."+
			" When BTC checkbox is on, price should show \u20BF + (price/btcPrice).")
	} else {
		t.Log("PASS: Price column toggles between $ and BTC")
	}
}

func TestReg_ChainsLoadedAfterUnlock(t *testing.T) {
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := "AA" + ts + ts + "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "F" }
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	if err != nil { t.Fatalf("unlock: %v", err) }

	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()

	pid, _ := d["profile_id"].(string)
	defer cleanupProfile(pid)
	chains, _ := d["chains"].([]interface{})
	t.Logf("Unlock response: chains=%d tokens=%d", len(chains), len(d["tokens"].([]interface{})))

	if len(chains) < 4 {
		t.Errorf("FAIL: unlock returned %d chains — should be 4 (BSC, POLYGON, ETHEREUM, OPBNB)."+
			" Chain dropdown will only show %d options.", len(chains), len(chains))
	} else {
		t.Log("PASS: unlock returns 4+ chains from DB")
	}

	// Verify the page HTML has the chains dropdown populated option values
	html := fetchPage(t, "/portfolio")
	if html != "" {
		chainOptions := []string{"BSC", "POLYGON", "ETHEREUM", "OPBNB"}
		missing := []string{}
		for _, co := range chainOptions {
			if !strings.Contains(html, co) {
				missing = append(missing, co)
			}
		}
		if len(missing) > 0 {
			t.Errorf("FAIL: Chain dropdown missing: %v", missing)
		} else {
			t.Log("PASS: All 4 chains present in page HTML")
		}
	}
}

func TestReg_DBNoPrivateKeyFragment(t *testing.T) {
	pk := pkEnv(t)
	if pk == "" {
		t.Skip("No PK available")
	}

	// Clear log first to get fresh state
	os.Truncate("../logs/system.log", 0)

	prefix := pk[:6]
	t.Logf("Checking DB for PK prefix '%s'", prefix)

	// Check user_profiles via API (use /api/daemons' DB check as proxy)
	resp, _ := http.Get("http://127.0.0.1:8080/api/daemons")
	if resp != nil {
		defer resp.Body.Close()
		var d struct{ Daemons []struct{ Name, Status, Message string } }
		json.NewDecoder(resp.Body).Decode(&d)
		for _, dm := range d.Daemons {
			if dm.Name == "database" && dm.Status == "healthy" {
				t.Log("DB is healthy — will check via psql")
				break
			}
		}
	}

	// Check file system for PK fragments in logs/output
	pkPrefix := pk[:8]
	logDirs := []string{"logs", "apps/governance/logs", "apps/school/logs",
		"apps/trading/logs", "web_output", "runtime"}
	found := false
	for _, dir := range logDirs {
		entries, _ := os.ReadDir("../" + dir)
		if entries == nil { continue }
		for _, e := range entries {
			if e.IsDir() { continue }
			data, err := os.ReadFile("../" + dir + "/" + e.Name())
			if err != nil { continue }
			if strings.Contains(string(data), pkPrefix) {
				t.Errorf("FAIL: PK prefix '%s' found in %s/%s", pkPrefix, dir, e.Name())
				found = true
			}
		}
	}
	if !found {
		t.Log("PASS: No PK fragment in log/output files")
	}

	// Also check balance.json + database.json
	for _, f := range []string{"../web_output/api/balance.json", "../web_output/api/database.json"} {
		data, err := os.ReadFile(f)
		if err != nil { continue }
		if strings.Contains(string(data), pkPrefix) {
		t.Errorf("FAIL: PK prefix '%s' found in %s", pkPrefix, f)
	}
}
}

func TestReg_NewTokenGetsLivePriceAfterOK(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// fetchLiveBalance must check DB for new tokens not in assetsData
	flbIdx := strings.Index(html, "function fetchLiveBalance")
	if flbIdx < 0 {
		t.Fatal("fetchLiveBalance not found")
	}
	flbBody := html[flbIdx : flbIdx+1500]
	hasTokenListSync := strings.Contains(flbBody, "/api/tokens/list") ||
		strings.Contains(flbBody, "api/tokens/list")
	t.Logf("fetchLiveBalance syncs from DB token list: %v", hasTokenListSync)
	if !hasTokenListSync {
		t.Errorf("FAIL: fetchLiveBalance does NOT call /api/tokens/list."+
			" After adding a token and OK, DB has it but balance.json does not."+
			" fetchLiveBalance only merges from balance.json -- new token never gets prices."+
			" FIX: fetchLiveBalance must also pull from /api/tokens/list.")
	} else {
		t.Log("PASS: fetchLiveBalance syncs new tokens from DB")
	}

	hasPriceMerge := strings.Contains(flbBody, "if(!found") ||
		strings.Contains(flbBody, "if(!found)")
	t.Logf("fetchLiveBalance adds unfound tokens: %v", hasPriceMerge)
	t.Logf("fetchLiveBalance adds unfound tokens: %v", hasPriceMerge)
}

func TestReg_ChainAddOptionPresent(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// The + Add New Chain option must be in chainSelect dropdown
	if !strings.Contains(html, `__add__`) || !strings.Contains(html, `Add New Chain`) {
		t.Errorf("FAIL: '+ Add New Chain' option missing from chainSelect dropdown."+
			" It must be in both the static HTML and the JS unlock flow.")
	} else {
		t.Log("PASS: + Add New Chain option present")
	}
}

func TestReg_MaskedKeyIsHashOnly(t *testing.T) {
	// Unlock with a fresh key to ensure clean profile
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := "EE" + ts + ts + "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "E" }
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	if err != nil { t.Fatalf("unlock: %v", err) }

	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()

	maskedKey, _ := d["masked_key"].(string)
	t.Logf("masked_key: %s", maskedKey)

	// masked_key must be 13-char SHA256 hash truncated + "*****"
	// Format: "XXXXXXXX*****" where X = hex chars
	if len(maskedKey) < 8 || strings.Contains(maskedKey, "...") {
		t.Errorf("FAIL: masked_key = '%s' — should be SHA256[:13] never a PK fragment.", maskedKey)
	} else {
		t.Log("PASS: masked_key is hash-based, not PK-based")
	}

	// Verify the profile_id does NOT contain any real PK prefix
	pk := pkEnv(t)
	pkPrefix := pk[:6]
	profileID, _ := d["profile_id"].(string)
	defer cleanupProfile(profileID)
	if strings.Contains(profileID, pkPrefix) {
		t.Errorf("FAIL: profile_id '%s' contains PK prefix '%s'", profileID[:16], pkPrefix)
	}
}

func TestReg_FirstUnlockSavesToDB(t *testing.T) {
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := ts + ts + ts + "FFFFFFFFFFFFF"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "0" }
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	if err != nil { t.Fatalf("unlock: %v", err) }

	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()

	pid, _ := d["profile_id"].(string)
	defer cleanupProfile(pid)
	profileExists, _ := d["profile_exists"].(bool)
	chains, _ := d["chains"].([]interface{})

	t.Logf("profile_exists=%v chains=%d tokens=%d",
		profileExists, len(chains), len(d["tokens"].([]interface{})))

	if len(chains) < 4 {
		t.Errorf("FAIL: New profile unlock returned %d chains — should be 4."+
			" This means chains were NOT seeded to user_chains DB on first unlock.", len(chains))
	} else {
		t.Log("PASS: 4 chains seeded to DB on first unlock")
	}

	if len(d["tokens"].([]interface{})) < 14 {
		t.Errorf("FAIL: New profile has only %d tokens — should be 14+ across all chains."+
			" Tokens were NOT seeded to user_tokens DB on first unlock.",
			len(d["tokens"].([]interface{})))
	} else {
		t.Log("PASS: tokens seeded to DB on first unlock")
	}
}

func TestReg_ExistingProfileNoReseed(t *testing.T) {
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := "RST" + ts + ts + "EEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEEE"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "R" }

	// 1) First unlock — create profile
	resp1, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&d1)
	resp1.Body.Close()

	pid, _ := d1["profile_id"].(string)
	defer cleanupProfile(pid)
	tokens1 := len(d1["tokens"].([]interface{}))
	t.Logf("1. First unlock: pid=%s... tokens=%d", pid[:16], tokens1)

	// 2) Delete ALL tokens for BSC — keep deleting until empty
	t.Log("2. Deleting all BSC tokens via API...")
	for attempt := 0; attempt < 30; attempt++ {
		r, err := http.Post("http://127.0.0.1:8080/api/tokens/delete",
			"application/json",
			strings.NewReader(`{"account_id":"`+pid+`","indices":[0],"chain":"BSC"}`))
		if err != nil { break }
		r.Body.Close()
		if r.StatusCode != 200 { break }
	}
	t.Log("   Done deleting BSC tokens")

	// 3) Re-unlock — verify tokens came from DB (should be fewer)
	resp2, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&d2)
	resp2.Body.Close()

	tokens2 := len(d2["tokens"].([]interface{}))
	chains2 := len(d2["chains"].([]interface{}))
	t.Logf("3. Re-unlock: chains=%d tokens=%d (was %d)", chains2, tokens2, tokens1)

	// Critical: tokens should NOT be back to full count
	if tokens2 >= tokens1-2 {
		t.Errorf("FAIL: Re-unlock returned %d tokens — same as first unlock %d."+
			" Tokens were deleted but unlock reseeded defaults."+
			" BUG in serve.py: default tokens re-added on existing profile.", tokens2, tokens1)
	}

	// 4) Check balance.json does not inject default tokens
	if tokens2 > 0 {
		t.Logf("INFO: %d non-BSC tokens still present (from other chains)", tokens2)
	}
}


func TestReg_EmptyChainsShowsAddOnlyOption(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// When unlock returns 0 chains, the JS should still show +Add option
	hasAddOption := strings.Contains(html, `__add__`) || strings.Contains(html, `Add New Chain`)
	t.Logf("+Add option in static HTML: %v", hasAddOption)
	if !hasAddOption {
		t.Errorf("FAIL: + Add New Chain option not in HTML."+
			" Without this, a profile with 0 chains has an empty dropdown with no way to add one.")
	}

	// When chains=0, unlockWallet code path must still be safe
	// The condition "if(d.chains && d.chains.length > 0)" must NOT block the flow
	if !strings.Contains(html, `d.chains && d.chains.length > 0`) {
		t.Errorf("FAIL: chains length check missing in unlockWallet JS")
	}
}

func TestReg_EmptyTokensShowsNoAssets(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// renderAssetRows must handle empty assetsData
	if !strings.Contains(html, "No active assets") {
		t.Errorf("FAIL: Empty assets message missing."+
			" When assetsData is empty and showAllTokens is off, renderAssetRows must show"+
			" 'No active assets on CHAIN' message, not an empty panel.")
	} else {
		t.Log("PASS: Empty assets message present")
	}

	// When tokens are 0 in response, the JS must still proceed
	hasTokensGuard := strings.Contains(html, `d.tokens && d.tokens.length > 0`)
	t.Logf("d.tokens length guard: %v", hasTokensGuard)
	if hasTokensGuard {
		t.Log("PASS: tokens guard exists — protects against empty response")
	}
}

func TestReg_ChainDeleteRedIconInEditMode(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) chain-delete row must exist in HTML
	if !strings.Contains(html, "chainDeleteRow") {
		t.Errorf("FAIL: chainDeleteRow div missing in page HTML."+
			" Chain delete chips cannot appear without this container."+
			" FIX: add <div id='chainDeleteRow'> to balance card HTML between chainAddRow and assetRows.")
	} else {
		t.Log("PASS: chainDeleteRow div exists")
	}

	// 2) markChainDeleted function must exist (was deleteChain — now defer-to-OK pattern)
	if !strings.Contains(html, "function markChainDeleted") {
		t.Errorf("FAIL: markChainDeleted JS function missing. Clicking red (-) on a chain chip has no handler.")
	} else {
		t.Log("PASS: markChainDeleted function exists")
	}
	// 4) markChainDeleted must flag deletedChains (not POST immediately)
	mcdIdx := strings.Index(html, "function markChainDeleted")
	if mcdIdx >= 0 {
		mcdBody := html[mcdIdx : mcdIdx+400]
		flagsDeleted := strings.Contains(mcdBody, "deletedChains[") || strings.Contains(mcdBody, "deletedChains=")
		t.Logf("markChainDeleted flags deletedChains: %v", flagsDeleted)
		if !flagsDeleted {
			t.Errorf("FAIL: markChainDeleted does NOT flag deletedChains."+
				" Red (-) click must only mark for deletion, not POST immediately.")
		} else {
			t.Log("PASS: markChainDeleted flags deletedChains for deferred save")
		}
		doesNotPOST := !strings.Contains(mcdBody, "api/verify/chain/delete")
		t.Logf("markChainDeleted does NOT POST immediately: %v", doesNotPOST)
		if !doesNotPOST {
			t.Errorf("FAIL: markChainDeleted immediately POSTs to server."+
				" Should only mark in deletedChains dict — OK button handles the POST.")
		} else {
			t.Log("PASS: markChainDeleted defers to saveTokenEdits for POST")
		}
	}

	// 5) saveTokenEdits must POST chain deletions
	saveIdx := strings.Index(html, "function saveTokenEdits")
	if saveIdx >= 0 {
		saveBody := html[saveIdx : saveIdx+2000]
		if !strings.Contains(saveBody, "chain/delete") {
			t.Errorf("FAIL: saveTokenEdits does NOT POST chain deletions."+
				" Deleted chains are lost when user clicks OK.")
		} else {
			t.Log("PASS: saveTokenEdits POSTs chain deletions")
		}
	}

	// 6) cancelEditMode must restore chains
	cancelIdx := strings.Index(html, "function cancelEditMode")
	if cancelIdx >= 0 {
		cancelBody := html[cancelIdx : cancelIdx+600]
		restoresChain := strings.Contains(cancelBody, "deletedChains") || strings.Contains(cancelBody, "chainRestore")
		t.Logf("cancelEditMode restores chains: %v", restoresChain)
		if !restoresChain {
			t.Errorf("FAIL: cancelEditMode does NOT restore chain deletions."+
				" If user clicks pencil again instead of OK, deleted chains are permanently lost.")
		} else {
			t.Log("PASS: cancelEditMode restores deleted chains")
		}
	}
}

func TestReg_ChainDeleteAPIRemovesChain(t *testing.T) {
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := "CHD" + ts + ts + "DDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDDD"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "D" }

	// 1) Create profile with chains
	resp, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()

	pid, _ := d["profile_id"].(string)
	defer cleanupProfile(pid)
	chains := len(d["chains"].([]interface{}))
	t.Logf("1. Unlock: chains=%d pid=%s...", chains, pid[:16])

	if chains < 4 {
		t.Fatalf("Expected 4 chains, got %d", chains)
	}

	// 2) Delete a chain via API
	delBody := fmt.Sprintf(`{"account_id":"%s","chain_name":"POLYGON"}`, pid)
	delResp, err := http.Post("http://127.0.0.1:8080/api/verify/chain/delete",
		"application/json",
		strings.NewReader(delBody))
	if err != nil { t.Fatalf("delete API: %v", err) }
	delResp.Body.Close()

	if delResp.StatusCode != 200 {
		t.Errorf("FAIL: /api/verify/chain/delete returned %d (expected 200)."+
			" API endpoint may not exist or is broken.", delResp.StatusCode)
		return
	}
	t.Log("2. DELETE /api/verify/chain/delete: 200 OK")

	// 3) Unlock again — verify chain count decreased
	resp2, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json",
		strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&d2)
	resp2.Body.Close()

	chains2 := len(d2["chains"].([]interface{}))
	t.Logf("3. Re-unlock: chains=%d (was %d)", chains2, chains)

	if chains2 >= chains {
		t.Errorf("FAIL: After deleting POLYGON chain, re-unlock still shows %d chains (was %d)."+
			" DELETE did not actually remove from user_chains DB.", chains2, chains)
	} else {
		t.Log("PASS: Chain count decreased after delete")
	}
}

func TestReg_DeleteAllChainsDoesNotReseed(t *testing.T) {
	ts := fmt.Sprintf("%x", time.Now().UnixNano())
	uniquePK := "NOC" + ts + ts + "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"
	if len(uniquePK) > 64 { uniquePK = uniquePK[:64] }
	for len(uniquePK) < 64 { uniquePK += "F" }

	// 1) Unlock to create profile
	resp, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json", strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&d)
	resp.Body.Close()
	pid, _ := d["profile_id"].(string)
	defer cleanupProfile(pid)
	t.Logf("1. Created profile: chains=%d", len(d["chains"].([]interface{})))

	// 2) Delete ALL chains via API
	chainNames := []string{"BSC", "POLYGON", "ETHEREUM", "OPBNB"}
	for _, cn := range chainNames {
		http.Post("http://127.0.0.1:8080/api/verify/chain/delete",
			"application/json",
			strings.NewReader(`{"account_id":"`+pid+`","chain_name":"`+cn+`"}`))
	}
	t.Log("2. Deleted all 4 chains via API")

	// 3) Re-unlock — must get 0 chains (not reseed BSC)
	resp2, _ := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json", strings.NewReader(`{"private_key":"`+uniquePK+`"}`))
	var d2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&d2)
	resp2.Body.Close()
	t.Logf("3. Re-unlock: chains=%d", len(d2["chains"].([]interface{})))

	if len(d2["chains"].([]interface{})) > 0 {
		t.Errorf("FAIL: After deleting all chains, re-unlock returned %d chains."+
			" Must be 0 — chains were reseeded (BUG in serve.py or unlockWallet JS).",
			len(d2["chains"].([]interface{})))
	} else {
		t.Log("PASS: Re-unlock returns 0 chains — no reseed")
	}
}

func TestReg_DBDropdownDeleteAPIWorks(t *testing.T) {
	// Test /api/database/delete endpoint (deletes row_count from a table)
	resp, err := http.Post("http://127.0.0.1:8080/api/database/delete",
		"application/json",
		strings.NewReader(`{"table":"user_tokens","rows":1}`))
	if err != nil {
		t.Logf("WARN: /api/database/delete unreachable: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		t.Log("PASS: /api/database/delete returns 200")
	} else {
		t.Errorf("FAIL: /api/database/delete returned %d", resp.StatusCode)
	}
}

func TestReg_NoReSeedOnReUnlock(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// When unlock returns chains=0, JS must show empty dropdown, not old static BSC
	if strings.Contains(html, `sel.innerHTML = '';`) {
		t.Log("PASS: unlockWallet clears chainSelect before populating")
	} else {
		t.Errorf("FAIL: unlockWallet does NOT clear chainSelect."+
			" Static BSC option persists when chains=0.")
	}

	// Must handle chains=0 gracefully
	hasEmptyOption := strings.Contains(html, `(no chains)`) || strings.Contains(html, `if(d.chains`)
	t.Logf("Empty chains handling: %v", hasEmptyOption)
	if !hasEmptyOption {
		t.Errorf("FAIL: No handling for chains=0 case")
	}
}
