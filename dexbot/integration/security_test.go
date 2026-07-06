package integration

import (
	"strings"
	"testing"
)

func TestSecurity_NoBalanceLeakWithoutPrivateKey(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	flbIdx := strings.Index(html, "function fetchLiveBalance")
	if flbIdx < 0 {
		t.Fatal("fetchLiveBalance function not found")
	}
	flbBody := html[flbIdx : flbIdx+400]

	hasGuard := strings.Contains(flbBody, "return") &&
		(strings.Contains(flbBody, "_accountId") ||
		 strings.Contains(flbBody, "unlocked") ||
		 strings.Contains(flbBody, "_wallet"))

	overwritesAlways := strings.Contains(flbBody, "assetsData = bd.assets")

	t.Logf("has unlock guard: %v", hasGuard)
	t.Logf("overwrites assetsData always: %v", overwritesAlways)

	if overwritesAlways && !hasGuard {
		t.Errorf("FAIL: fetchLiveBalance overwrites assetsData with real API data\n"+
			"  even when wallet is NOT unlocked. SetInterval runs every 5 seconds.\n"+
			"  /api/balance returns cached real balances ($1.03 with 21 tokens).\n"+
			"  When user clicks privacy toggle, real values leak.\n"+
			"  FIX: add if(!window._accountId) return; at top of fetchLiveBalance")
	} else {
		t.Log("PASS: fetchLiveBalance has unlock guard")
	}

	if strings.Contains(html, "setInterval(fetchLiveBalance, 5000)") && !hasGuard {
		t.Errorf("FAIL: setInterval(fetchLiveBalance,5000) starts at page load.\n"+
			"  FIX: add guard in fetchLiveBalance, OR start setInterval only\n"+
			"  inside unlockWallet() after successful unlock.")
	}
}

func TestSecurity_BTCPriceDOMUpdateWorking(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function fetchBTCPrice") {
		t.Errorf("FAIL: fetchBTCPrice function missing")
		return
	}
	t.Log("PASS: fetchBTCPrice function exists")

	if !strings.Contains(html, "setInterval(fetchBTCPrice") {
		t.Errorf("FAIL: setInterval(fetchBTCPrice) missing — no auto-refresh")
	} else {
		t.Log("PASS: setInterval(fetchBTCPrice) exists")
	}

	fbpIdx := strings.Index(html, "function fetchBTCPrice")
	fbpBody := html[fbpIdx : fbpIdx+400]

	updatesDOM := strings.Contains(fbpBody, "getElementById('btcPrice')") ||
		strings.Contains(fbpBody, `getElementById("btcPrice")`)
	t.Logf("fetchBTCPrice updates btcPrice DOM span: %v", updatesDOM)
	if !updatesDOM {
		t.Errorf("FAIL: fetchBTCPrice does NOT update the btcPrice DOM span")
	} else {
		t.Log("PASS: fetchBTCPrice updates btcPrice span via getElementById")
	}

	// Check btcPrice span exists in HTML with correct structure
	hasBtcPriceSpan := strings.Contains(html, `id="btcPrice"`) ||
		strings.Contains(html, `id='btcPrice'`)
	t.Logf("btcPrice span exists: %v", hasBtcPriceSpan)
	if !hasBtcPriceSpan {
		t.Errorf("FAIL: No btcPrice span in HTML — BTC price never displayed")
	}

	// Verify CoinGecko fetch is reasonable
	usesCoinGecko := strings.Contains(html, "coingecko.com")
	t.Logf("Coingecko API used: %v", usesCoinGecko)
	if !usesCoinGecko {
		t.Errorf("FAIL: BTC price fetch does not use CoinGecko")
	}
}

func TestSecurity_BalanceAPIReturnsDataWithoutKey(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	hasUnconditionalPoll := strings.Contains(html, "setInterval(fetchLiveBalance") &&
		strings.Contains(html, "5000")

	t.Logf("unconditional balance poll: %v", hasUnconditionalPoll)
	if hasUnconditionalPoll {
		t.Log("NOTE: /api/balance returns real data without private key.\n"+
			"  setInterval(fetchLiveBalance,5000) fires before unlock.\n"+
			"  If fetchLiveBalance lacks unlock guard, assetsData leaks.")
	}
}
