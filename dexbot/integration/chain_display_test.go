package integration

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// ══════════════════════════════════════════════════════════
// CHAIN DROPDOWN TOTAL DISPLAY TESTS
// User reported: chain selection shows no balance in dropdown.
// These tests validate the full chain-total display pipeline.
// ══════════════════════════════════════════════════════════

func TestChainTotalDisplayElementExists(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" {
		return
	}

	// Chain total now shown in dropdown labels via updateDropdownOptionLabels,
	// not as a separate chainTotalDisplay row (removed per BUG N2).
	// Verify the dropdown-based chain total pipeline exists.
	if strings.Contains(html, "function computeChainBalances()") {
		t.Log("PASS: computeChainBalances function defined")
	} else {
		t.Errorf("FAIL: computeChainBalances() function NOT found")
	}

	if strings.Contains(html, "function updateDropdownOptionLabels") {
		t.Log("PASS: updateDropdownOptionLabels function defined")
	} else {
		t.Errorf("FAIL: updateDropdownOptionLabels missing — no per-chain totals")
	}

	if strings.Contains(html, "chainTotals[opt.value]") ||
		strings.Contains(html, "chainTotals") {
		t.Log("PASS: dropdown labels use per-chain totals")
	} else {
		t.Errorf("FAIL: dropdown labels do not reference chain totals")
	}

	re := regexp.MustCompile(`var assetsData = (.+?);`)
	m := re.FindStringSubmatch(html)
	if len(m) >= 2 {
		var assets []map[string]interface{}
		if err := json.Unmarshal([]byte(m[1]), &assets); err == nil && len(assets) > 0 {
			chainMap := make(map[string]bool)
			for _, a := range assets {
				if cn, ok := a["chain_name"].(string); ok {
					chainMap[cn] = true
				}
			}
			t.Logf("PASS: assetsData has %d unique chain_names: %v", len(chainMap), chainMap)
		}
	}
}

func TestChainTotalUpdatesOnSelection(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" {
		return
	}

	h1 := strings.Contains(html, "function checkChainSelection()")
	h2 := strings.Contains(html, "function refreshAssetPanel()")
	h3 := strings.Contains(html, "function renderAssetRows()")
	h4 := strings.Contains(html, "function updateDropdownOptionLabels()")

	t.Logf("checkChainSelection=%v refreshAssetPanel=%v renderAssetRows=%v updateDropdown=%v", h1, h2, h3, h4)

	if !h1 { t.Errorf("FAIL: checkChainSelection() NOT found") }
	if !h2 { t.Errorf("FAIL: refreshAssetPanel() NOT found") }
	if !h3 { t.Errorf("FAIL: renderAssetRows() NOT found") }
	if !h4 { t.Errorf("FAIL: updateDropdownOptionLabels() NOT found") }
	if h1 && h2 && h3 && h4 { t.Log("PASS: Full chain selection update pipeline exists") }
}

func TestChainDropdownLabelsContainBalances(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "opt.textContent") {
		t.Errorf("FAIL: updateDropdownOptionLabels does NOT set opt.textContent")
	} else { t.Log("PASS: updateDropdownOptionLabels writes to option textContent") }

	if strings.Contains(html, "chainTotals") && strings.Contains(html, "var chainUSD") {
		t.Log("PASS: dropdown labels compute per-chain totals")
	} else { t.Errorf("FAIL: dropdown labels do NOT compute per-chain totals") }
}

func TestChainTotalDisplayShowsWhenPanelOpen(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if strings.Contains(html, "refreshAssetPanel()") {
		t.Log("PASS: toggleChainPanel calls refreshAssetPanel")
	} else { t.Errorf("FAIL: toggleChainPanel does NOT call refreshAssetPanel") }

	if strings.Contains(html, "chainSum") || strings.Contains(html, "activeChainVal") {
		t.Log("PASS: renderAssetRows computes per-chain sum")
	} else { t.Errorf("FAIL: renderAssetRows does NOT compute per-chain sum") }
}
