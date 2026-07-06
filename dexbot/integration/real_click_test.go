package integration

import (
	"net/http"
	"strings"
	"testing"
)

func TestRealClick_UnlockHidesPKShowsStatus(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, `id="pkInput"`) {
		t.Errorf("FAIL: pkInput field missing")
	} else { t.Log("PASS: pkInput exists") }

	if !strings.Contains(html, `onclick="unlockWallet()"`) {
		t.Errorf("FAIL: OK button missing onclick=unlockWallet()")
	} else { t.Log("PASS: OK button wired to unlockWallet()") }

	if !strings.Contains(html, `id="acctStatus"`) {
		t.Errorf("FAIL: acctStatus span missing")
	} else { t.Log("PASS: acctStatus span exists") }

	// unlockWallet must hide pkInput on success
	hasHide := strings.Contains(html, "pkInput')") &&
		(strings.Contains(html, "style.display='none'") ||
		 strings.Contains(html, `style.display="none"`))
	if !hasHide {
		t.Errorf("FAIL: unlockWallet does NOT hide pkInput after success")
	} else { t.Log("PASS: unlockWallet hides pkInput on success") }

	// unlockWallet must show acctStatus
	hasShow := strings.Contains(html, "acctStatus") &&
		(strings.Contains(html, "style.display='inline'") ||
		 strings.Contains(html, `style.display="inline"`))
	if !hasShow {
		t.Errorf("FAIL: unlockWallet does NOT show acctStatus after success")
	} else { t.Log("PASS: unlockWallet shows acctStatus on success") }
}

func TestRealClick_BalanceDigitsToggleWorks(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, `id="balanceAmount"`) {
		t.Errorf("FAIL: balanceAmount span missing")
	} else { t.Log("PASS: balanceAmount span exists") }

	if !strings.Contains(html, `toggleBalancePrivacy()`) {
		t.Errorf("FAIL: toggleBalancePrivacy() onclick missing")
	} else { t.Log("PASS: toggleBalancePrivacy() wired to onclick") }

	if !strings.Contains(html, "var showAllNumbers = false") {
		t.Errorf("FAIL: showAllNumbers doesn't default to false")
	} else { t.Log("PASS: showAllNumbers defaults to false") }

	if !strings.Contains(html, "showAllNumbers = !showAllNumbers") {
		t.Errorf("FAIL: toggleBalancePrivacy doesn't toggle showAllNumbers")
	} else { t.Log("PASS: toggleBalancePrivacy flips showAllNumbers") }

	if !strings.Contains(html, "showAllNumbers ? globalSym") &&
		!strings.Contains(html, "showAllNumbers ? globalSym") {
		t.Errorf("FAIL: balanceAmount.textContent doesn't check showAllNumbers")
	} else { t.Log("PASS: balanceAmount uses showAllNumbers guard") }
}

func TestRealClick_ChainPanelExpands(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, `class="balance-interactive-header"`) {
		t.Errorf("FAIL: balance-interactive-header missing")
	} else { t.Log("PASS: balance-interactive-header exists") }

	if !strings.Contains(html, `onclick="toggleChainPanel()"`) {
		t.Errorf("FAIL: header missing onclick=toggleChainPanel()")
	} else { t.Log("PASS: header wired to toggleChainPanel()") }

	if !strings.Contains(html, "classList.contains") {
		t.Errorf("FAIL: toggleChainPanel doesn't use classList.contains")
	} else { t.Log("PASS: toggleChainPanel uses classList.contains") }

	if !strings.Contains(html, "classList.add") && !strings.Contains(html, "classList.toggle") {
		t.Errorf("FAIL: toggleChainPanel doesn't modify classList")
	} else { t.Log("PASS: toggleChainPanel modifies classList") }

	if !strings.Contains(html, ".chain-panel.open{display:block") {
		t.Errorf("FAIL: .chain-panel.open CSS rule missing")
	} else { t.Log("PASS: .chain-panel.open CSS rule exists") }
}

func TestRealClick_UnlockThenBalanceFlows(t *testing.T) {
	unlockBody := `{"private_key":"` + pkEnv(t) + `"}`
	resp, err := http.Post("http://127.0.0.1:8080/api/unlock",
		"application/json", strings.NewReader(unlockBody))
	if err != nil {
		t.Fatalf("unlock POST failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("unlock returned %d", resp.StatusCode)
	}
	t.Log("1. Unlock: HTTP 200 OK")

	balResp, err := http.Get("http://127.0.0.1:8080/api/balance")
	if err != nil {
		t.Fatalf("balance GET failed: %v", err)
	}
	balResp.Body.Close()

	if balResp.StatusCode != 200 {
		t.Errorf("FAIL: /api/balance returned %d", balResp.StatusCode)
	}
	t.Log("2. Balance API: HTTP 200 OK")

	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	checks := []string{
		`onclick="unlockWallet()"`,
		`toggleBalancePrivacy()`,
		`toggleChainPanel()`,
	}
	allOK := true
	for _, c := range checks {
		if !strings.Contains(html, c) {
			t.Errorf("FAIL: %s missing from page", c)
			allOK = false
		}
	}
	if allOK {
		t.Log("3. All onclick handlers present in page")
	}

	if !strings.Contains(html, ".chain-panel{display:none") {
		t.Errorf("FAIL: .chain-panel CSS missing")
	}
	if !strings.Contains(html, ".chain-panel.open{display:block") {
		t.Errorf("FAIL: .chain-panel.open CSS missing")
	}
	t.Log("4. Chain panel CSS rules present")
}
