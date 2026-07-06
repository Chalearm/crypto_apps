package integration

import (
	"strings"
	"testing"
)

// new_bugs_test.go — 6 new bugs reported 2026-07-05 after unlock fix.
// Tests designed to FAIL when bugs present, PASS after fixes.

func TestBugN1_BTCPriceAutoRefresh(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function fetchBTCPrice") {
		t.Errorf("FAIL: fetchBTCPrice function not found")
	} else { t.Log("PASS: fetchBTCPrice function exists") }

	if !strings.Contains(html, "setInterval") {
		t.Errorf("FAIL: No setInterval for BTC price auto-refresh")
	} else { t.Log("PASS: setInterval found") }
}

func TestBugN2_ChainTotalRowRedundant(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }
	n := strings.Count(html, "chainTotalDisplay")
	t.Logf("chainTotalDisplay occurrences: %d", n)
}

func TestBugN3_AddTokenFieldsMissing(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "toggleEditMode") {
		t.Errorf("FAIL: toggleEditMode missing")
	} else { t.Log("PASS: toggleEditMode exists") }

	hasAdd := strings.Contains(html, "addTokenFields") || strings.Contains(html, "tokTicker")
	hasAddr := strings.Contains(html, "tokAddr")
	t.Logf("addTokenFields: %v, tokAddr: %v", hasAdd, hasAddr)
	if !hasAdd || !hasAddr {
		t.Errorf("FAIL: Add-token input fields missing in edit mode")
	}

	hasSave := strings.Contains(html, "saveToken(") || strings.Contains(html, "addTokenSubmit")
	hasCancel := strings.Contains(html, "hideAddTokenFields") || strings.Contains(html, "cancelAddToken")
	t.Logf("save: %v, cancel: %v", hasSave, hasCancel)
	if !hasSave { t.Errorf("FAIL: No submit button for add token") }
	if !hasCancel { t.Errorf("FAIL: No cancel button for add token") }
}

func TestBugN4_RedMinusButtonBroken(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "markTokenDeleted") {
		t.Errorf("FAIL: markTokenDeleted function missing")
	} else { t.Log("PASS: markTokenDeleted exists") }

	if !strings.Contains(html, "delete-token-btn") {
		t.Errorf("FAIL: delete-token-btn CSS missing")
	} else { t.Log("PASS: delete-token-btn CSS exists") }

	hasInlineStyle := strings.Contains(html, "opacity:1")
	t.Logf("Inline opacity:1 present: %v", hasInlineStyle)
	if !hasInlineStyle {
		t.Errorf("FAIL: delete-token-btn lacks inline opacity:1 in edit mode")
	}
}

// NOTE: Bug N5 (user_tokens DB empty) removed per user request — see TODO5.txt

func TestBugN6_PrivacyToggleLeaksAmounts(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// Asset amount should NOT use format9Decimal without showAllNumbers check
	hasUnprotectedAmount := strings.Contains(html, "format9Decimal(a.amount||0)")
	hasProtectedAmount := strings.Contains(html, "showAllNumbers ? format9Decimal(a.amount")
	t.Logf("amount has showAllNumbers guard: %v", hasProtectedAmount)
	t.Logf("amount leaks real numbers: %v", hasUnprotectedAmount)
	if hasUnprotectedAmount && !hasProtectedAmount {
		t.Errorf("FAIL: Amount column leaks real numbers when privacy ON")
	} else { t.Log("PASS: Amount column respects privacy toggle") }

	if !strings.Contains(html, "balanceAmount") {
		t.Errorf("FAIL: balanceAmount element missing")
	}
}
