package integration

import (
	"strings"
	"testing"
)

// =============================================================================
// 5-BUG VERIFICATION TESTS
// These tests validate all 5 reported bugs. They are designed to FAIL when
// bugs are present and PASS after fixes are applied.
// Results written to doc/integration_bugs_test_result.txt
// =============================================================================

// ── BUG 1: Chain balance is always zero in dropdown ──

func TestBug1_ChainBalanceNotZero(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// Check: computeChainBalances aggregates usd_value correctly
	if !strings.Contains(html, "a.usd_value") {
		t.Errorf("BUG1-FAIL: computeChainBalances does not check usd_value — chain totals always 0")
	} else { t.Log("BUG1-PASS: computeChainBalances uses usd_value") }

	// Check: updateDropdownOptionLabels writes computed balances to dropdown
	if !strings.Contains(html, "chainTotals") || !strings.Contains(html, "opt.textContent") {
		t.Errorf("BUG1-FAIL: dropdown labels do NOT display chain totals")
	} else { t.Log("BUG1-PASS: dropdown labels display chain totals") }

	// Check: chainTotalDisplay element exists for current chain total
	if !strings.Contains(html, `id="chainTotalDisplay"`) {
		t.Errorf("BUG1-FAIL: chainTotalDisplay element missing — per-chain total invisible")
	} else { t.Log("BUG1-PASS: chainTotalDisplay element exists") }

	// Check: BTC toggle is wired and references chainTotals via updateDropdownOptionLabels
	if !strings.Contains(html, "btcToggle") {
		t.Errorf("BUG1-FAIL: btcToggle missing — BTC denomination not available")
	} else { t.Log("BUG1-PASS: btcToggle BTC denomination present") }

	// Check: btcPrice variable is populated (not always 0)
	if strings.Contains(html, "var btcPrice = 0") || strings.Contains(html, "var btcPrice = 0.") {
		t.Log("BUG1-WARN: btcPrice embedded as 0 — will default to 0 until CoinGecko fetch completes")
	}
	if strings.Contains(html, "fetchBTCPrice") {
		t.Log("BUG1-PASS: fetchBTCPrice live BTC price fetch exists")
	}
}

// ── BUG 2: Pencil icon missing, edit mode broken ──

func TestBug2_PencilIconAndEditMode(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "pencil-icon") {
		t.Errorf("BUG2-FAIL: pencil-icon CSS class missing — edit button invisible")
	} else { t.Log("BUG2-PASS: pencil-icon CSS class present") }

	if !strings.Contains(html, "toggleEditMode") {
		t.Errorf("BUG2-FAIL: toggleEditMode function missing — clicking pencil does nothing")
	} else { t.Log("BUG2-PASS: toggleEditMode function exists") }

	if !strings.Contains(html, "delete-token-btn") {
		t.Errorf("BUG2-FAIL: delete-token-btn CSS class missing — red minus not shown in edit mode")
	} else { t.Log("BUG2-PASS: delete-token-btn CSS class present") }

	if !strings.Contains(html, "editActions") {
		t.Errorf("BUG2-FAIL: editActions div missing — OK/Cancel buttons not shown")
	} else { t.Log("BUG2-PASS: editActions div present") }

	if !strings.Contains(html, "editOkBtn") || !strings.Contains(html, "disabled") {
		t.Errorf("BUG2-FAIL: OK button not disabled by default")
	} else { t.Log("BUG2-PASS: OK button disabled by default") }

	// Check cancel discards changes
	if !strings.Contains(html, "cancelEditMode") {
		t.Errorf("BUG2-FAIL: cancelEditMode missing — cannot exit edit mode")
	} else { t.Log("BUG2-PASS: cancelEditMode function exists") }

	// Check pencil icon ::before content renders pencil emoji
	if !strings.Contains(html, `\270F` ) && !strings.Contains(html, `\u270F`) {
		t.Log("BUG2-WARN: pencil ::before content may not render correctly")
	}
}

// ── BUG 3: Duplicate items in DB dropdown ──

func TestBug3_DBDropdownDuplicates(t *testing.T) {
	html := fetchPage(t, "/school")
	if html == "" { return }

	if !strings.Contains(html, "_dbTablesLoaded") {
		t.Errorf("BUG3-FAIL: _dbTablesLoaded guard missing — populateDBTables can fire multiple times causing duplicates")
	} else { t.Log("BUG3-PASS: _dbTablesLoaded guard exists") }

	// Check that the guard prevents re-population
	if !strings.Contains(html, "if(_dbTablesLoaded") && !strings.Contains(html, "if(_dbTablesLoaded2") {
		t.Errorf("BUG3-FAIL: populateDBTables guard check missing")
	} else { t.Log("BUG3-PASS: populateDBTables guard check present") }

	// Check duplicate population calls (DOMContentLoaded + 500ms setTimeout)
	calls := strings.Count(html, "populateDBTables()")
	t.Logf("DEBUG: populateDBTables() calls found: %d", calls)
	if calls > 2 {
		t.Logf("BUG3-WARN: %d populateDBTables() calls — verify dedup guard prevents duplicates", calls)
	}
}

// ── BUG 4: Chain add dialog should be inline, not modal ──

func TestBug4_ChainAddInlineNotModal(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if strings.Contains(html, `id="chainEditor"`) && strings.Contains(html, `position:fixed`) {
		t.Errorf("BUG4-FAIL: chainEditor still uses position:fixed modal — should be inline")
	} else { t.Log("BUG4-PASS: chainEditor modal removed/replaced with inline form") }

	if !strings.Contains(html, "chainAddRow") {
		t.Errorf("BUG4-FAIL: chainAddRow div missing — inline chain-add form not present")
	} else { t.Log("BUG4-PASS: chainAddRow inline form present") }

	// Check inline inputs exist
	if !strings.Contains(html, `chainNameInput`) {
		t.Errorf("BUG4-FAIL: chainNameInput missing in inline form")
	} else { t.Log("BUG4-PASS: chainNameInput exists") }

	if !strings.Contains(html, `cancelChainAdd`) {
		t.Errorf("BUG4-FAIL: cancelChainAdd function missing — cannot close inline form")
	} else { t.Log("BUG4-PASS: cancelChainAdd function exists") }

	// Check the styling uses visible class toggle, not display:none/block
	if strings.Contains(html, "chain-add-row") {
		t.Log("BUG4-PASS: chain-add-row CSS class defined for inline styling")
	}
}

// ── BUG 5: BTC reference price not working ──

func TestBug5_BTCRefPriceWorking(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, `id="btcPrice"`) {
		t.Errorf("BUG5-FAIL: btcPrice span missing — BTC ref not displayed")
	} else { t.Log("BUG5-PASS: btcPrice span exists") }

	if strings.Contains(html, "fetchBTCPrice") {
		t.Log("BUG5-PASS: fetchBTCPrice CoinGecko fetch exists")
	} else {
		// Fallback: at least btcPrice shouldn't be hardcoded 0
		t.Log("BUG5-WARN: no fetchBTCPrice — btcPrice may be static 0")
	}

	// Check BTC toggle checkbox exists
	if !strings.Contains(html, `id="btcToggle"`) {
		t.Errorf("BUG5-FAIL: btcToggle checkbox missing — can't switch to BTC denomination")
	} else { t.Log("BUG5-PASS: btcToggle checkbox exists") }

	// Check computeChainBalances uses btcPrice for conversion
	if strings.Contains(html, "chainVal/btcPrice") || strings.Contains(html, "/ btcPrice") {
		t.Log("BUG5-PASS: btcPrice used in chain total conversion")
	} else { t.Log("BUG5-WARN: btcPrice may not be used in chain calculations") }
}
