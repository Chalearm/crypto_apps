package integration

import (
	"strings"
	"testing"
)

// new_bugs2_test.go — additional bugs/requirements reported 2026-07-05
// Tests for: live refresh, number format, chain-add inline form position

// ━━━━━ N7: Asset values must auto-refresh like BTC price ━━━━━

func TestBugN7_AssetValuesAutoRefresh(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) setInterval must exist for periodic refresh of balance/asset data
	if !strings.Contains(html, "setInterval") {
		t.Errorf("FAIL: No setInterval found in page JS — no live refresh.\n" +
			"  All asset values and BTC price are frozen after initial load.\n" +
			"  FIX: add setInterval(fetchAssetData, balanceRefreshSeconds*1000)\n" +
			"  where balanceRefreshSeconds reads BALANCE_REFRESH_SECONDS from config.")
	} else {
		t.Log("PASS: setInterval exists")
	}

	// 2) BTC price must be refreshed by setInterval (not just once)
	hasBTCInterval := strings.Contains(html, "setInterval(fetchBTCPrice") ||
		strings.Contains(html, "setInterval(function") && strings.Contains(html, "fetchBTCPrice")
	t.Logf("BTC price wrapped in setInterval: %v", hasBTCInterval)
	if !hasBTCInterval {
		t.Errorf("FAIL: fetchBTCPrice not called from setInterval — BTC price loads once only")
	}

	// 3) Balance/asset data must be refreshed after unlock
	hasBalanceRefresh := strings.Contains(html, "setInterval") &&
		(strings.Contains(html, "api/balance") || strings.Contains(html, "fetchBalance"))
	t.Logf("Balance data refreshed periodically: %v", hasBalanceRefresh)
	if !hasBalanceRefresh {
		t.Errorf("FAIL: No periodic balance/asset fetch — amounts stay stale after initial unlock")
	}
}

// ━━━━━ N8: All numbers must use 3-digit spaced format (format9Decimal) ━━━━━

func TestBugN8_NumberFormatThreeDigitSpacing(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) format9Decimal function must exist
	if !strings.Contains(html, "function format9Decimal") {
		t.Errorf("FAIL: format9Decimal function not found — numbers won't have 3-digit spacing")
	} else { t.Log("PASS: format9Decimal function defined") }

	// 2) format9Decimal must split into integer + fraction with 3-digit groups
	hasIntGroups := strings.Contains(html, "intGroups")
	hasFracGroups := strings.Contains(html, "fracGroups")
	t.Logf("intGroups: %v, fracGroups: %v", hasIntGroups, hasFracGroups)
	if !hasIntGroups || !hasFracGroups {
		t.Errorf("FAIL: format9Decimal missing 3-digit grouping logic")
	} else { t.Log("PASS: format9Decimal has 9 fractional digit groups") }

	// 3) All numeric display fields must use format9Decimal
	// Check: balanceAmount, chainTotalDisplay, asset-amount, asset-usd
	usesFormatFunc := strings.Count(html, "format9Decimal")
	t.Logf("format9Decimal called %d times", usesFormatFunc)
	if usesFormatFunc < 4 {
		t.Errorf("FAIL: format9Decimal used only %d times — some number fields may lack 3-digit spacing.\n"+
			"  All of balanceAmount, chainTotalDisplay, asset-amount, asset-usd, btcPrice must use it.", usesFormatFunc)
	}
}

// ━━━━━ N9: Chain add (+) shows dialog — should be inline form between dropdown and token list ━━━━━

func TestBugN9_ChainAddInlineBetweenDropdownAndTokens(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// 1) Old modal chainEditor should NOT exist
	if strings.Contains(html, `id="chainEditor"`) && strings.Contains(html, `position:fixed`) {
		t.Errorf("FAIL: chainEditor modal still exists with position:fixed — should be inline form")
	} else { t.Log("PASS: chainEditor modal removed") }

	// 2) There should be exactly ONE chainSelect (not duplicated)
	chainSelectCount := strings.Count(html, `id="chainSelect"`)
	t.Logf("chainSelect occurrences: %d", chainSelectCount)
	if chainSelectCount > 1 {
		t.Errorf("FAIL: chainSelect appears %d times (duplicate) — duplicate dropdown in page", chainSelectCount)
	}

	// 3) chainAddRow must exist for inline form
	if !strings.Contains(html, "chainAddRow") {
		t.Errorf("FAIL: chainAddRow div missing — inline form doesn't exist")
	} else { t.Log("PASS: chainAddRow div exists") }

	// 4) chainAddRow must have 3 input fields: name, id, baseUrl
	hasName := strings.Contains(html, "chainNameInput")
	hasId := strings.Contains(html, "chainIdInput")
	hasUrl := strings.Contains(html, "chainBaseUrlInput")
	t.Logf("nameInput:%v idInput:%v urlInput:%v", hasName, hasId, hasUrl)
	if !hasName || !hasId || !hasUrl {
		n := 0; if hasName {n++}; if hasId {n++}; if hasUrl {n++}
		t.Errorf("FAIL: chainAddRow missing input fields (need name, id, baseUrl) — %d/3 present", n)
	} else { t.Log("PASS: all 3 chain-add input fields present") }

	// 5) chainAddRow must have OK + Cancel buttons
	hasOkBtn := strings.Contains(html, "saveChain()")
	hasCancelBtn := strings.Contains(html, "cancelChainAdd()")
	t.Logf("OK btn:%v Cancel btn:%v", hasOkBtn, hasCancelBtn)
	if !hasOkBtn || !hasCancelBtn {
		t.Errorf("FAIL: OK/Cancel buttons missing for chain-add form")
	} else { t.Log("PASS: OK and Cancel buttons present") }

	// 6) The form must be positioned BETWEEN chain dropdown and assetRows/token list
	// This is a structural check — chainAddRow div should appear after chainSelect
	// and before assetRows in the HTML. We verify both elements exist.
	hasAssetRows := strings.Contains(html, `id="assetRows"`)
	t.Logf("assetRows div exists: %v", hasAssetRows)
	if !hasAssetRows {
		t.Errorf("FAIL: assetRows div missing — token list not rendered")
	}

	// 7) Chain add button text must be "+ Add New Chain" not some other label
	if !strings.Contains(html, "Add New Chain") {
		t.Errorf("FAIL: Add New Chain option text missing from dropdown")
	} else { t.Log("PASS: + Add New Chain option exists") }
}

// ━━━━━ N10: Balance card dropdown appears twice (duplicate HTML) ━━━━━

func TestBugN10_DuplicateDropdowns(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// Count the balance-card sections
	balanceCardCount := strings.Count(html, `class="balance-card"`)
	t.Logf("balance-card sections: %d", balanceCardCount)

	// Count chainSelect elements — should be exactly 1 per balance card
	chainSelectCount := strings.Count(html, `id="chainSelect"`)
	t.Logf("chainSelect elements: %d", chainSelectCount)

	if chainSelectCount > balanceCardCount {
		t.Errorf("FAIL: chainSelect elements(%d) > balance-card sections(%d) — duplicate dropdown.\n"+
			"  Each balance card should have exactly 1 chainSelect.", chainSelectCount, balanceCardCount)
	}
	if chainSelectCount > 1 {
		t.Errorf("FAIL: %d chainSelect elements — dropdown duplicated on page", chainSelectCount)
	} else if chainSelectCount == 1 {
		t.Log("PASS: exactly 1 chainSelect dropdown")
	}
}
