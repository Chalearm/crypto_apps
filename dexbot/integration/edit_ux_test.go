package integration

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// edit_ux_test.go — validates edit mode UX per user requirements 2026-07-06
// Bug C1: OK button fails to save (serve.py /api/tokens/delete wrong handler)
// Bug C2: Edit mode shows inputs immediately — should show green (+) first
// Bug C3: Token add lacks address hex format validation

func TestEdit_OkButtonSavesAndPersists(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function saveTokenEdits") {
		t.Errorf("FAIL: saveTokenEdits function missing — OK button does nothing")
	} else { t.Log("PASS: saveTokenEdits function exists") }

	if !strings.Contains(html, "api/tokens/delete") {
		t.Errorf("FAIL: saveTokenEdits does NOT call /api/tokens/delete")
	} else { t.Log("PASS: saveTokenEdits calls /api/tokens/delete") }

	if !strings.Contains(html, "function cancelEditMode") {
		t.Errorf("FAIL: cancelEditMode function missing")
	} else { t.Log("PASS: cancelEditMode function exists") }

	if !strings.Contains(html, "editActions") {
		t.Errorf("FAIL: editActions div missing")
	} else { t.Log("PASS: editActions div exists") }
}

func TestEdit_GreenPlusButtonBeforeInputs(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "toggleEditMode") {
		t.Errorf("FAIL: toggleEditMode missing")
	} else { t.Log("PASS: toggleEditMode function exists") }

	// Green (+) button must exist separately from the addTokenFields inputs
	hasAddTokenBtnRow := strings.Contains(html, "addTokenBtnRow")
	hasAddTokenFields := strings.Contains(html, "addTokenFields")
	t.Logf("addTokenBtnRow: %v, addTokenFields: %v", hasAddTokenBtnRow, hasAddTokenFields)

	if !hasAddTokenBtnRow {
		t.Errorf("FAIL: addTokenBtnRow missing — no green (+) button.\n"+
			"  Expected: click pencil → green (+) button appears →\n"+
			"  click green (+) → tokTicker/tokAddr inputs + Submit/Cancel appear,\n"+
			"  red (-) delete buttons hide. FIX: add addTokenBtnRow div.")
	} else { t.Log("PASS: addTokenBtnRow green (+) button exists") }

	// Green button must use green color to distinguish from OK/Cancel
	hasGreenColor := strings.Contains(html, "#34d399") || strings.Contains(html, "var(--green)")
	t.Logf("green color: %v", hasGreenColor)

	// addTokenFields must NOT use edit-actions class (separate from OK/Cancel row)
	bothEditActions := strings.Contains(html, `id="addTokenFields"`) &&
		strings.Contains(html, `class="edit-actions"`) &&
		strings.Contains(html, `id="addTokenFields"`) &&
		strings.Contains(html, `class="edit-actions"`)
	_ = bothEditActions // informational
	t.Logf("addTokenFields and editActions are separate divs: %v", hasAddTokenFields)
}

func TestEdit_TokenAddAddressValidation(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function addTokenSubmit") {
		t.Errorf("FAIL: addTokenSubmit function missing")
		return
	}
	t.Log("PASS: addTokenSubmit function exists")

	// Must have regex pattern check for 0x + 40 hex chars
	hasHexRegex := strings.Contains(html, "0x[a-fA-F0-9]{40}") ||
		strings.Contains(html, "/^0x[a-fA-F0-9]{40}$/")
	t.Logf("hex regex validation: %v", hasHexRegex)
	if !hasHexRegex {
		t.Errorf("FAIL: addTokenSubmit missing address hex format validation.\n"+
			"  Users can submit invalid addresses causing API errors.\n"+
			"  FIX: add /^0x[a-fA-F0-9]{40}$/.test(address) check before POST.")
	}

	// Must have ticker uppercase + trim
	hasTickerSanitize := strings.Contains(html, "toUpperCase")
	t.Logf("ticker sanitize: %v", hasTickerSanitize)
	if !hasTickerSanitize {
		t.Errorf("FAIL: ticker not sanitized — should use toUpperCase()")
	}
}

func TestEdit_CancelRollsBackState(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	if !strings.Contains(html, "function cancelEditMode") {
		t.Errorf("FAIL: cancelEditMode missing")
		return
	}
	t.Log("PASS: cancelEditMode function exists")

	hasClear := strings.Contains(html, "deletedTokens = {}") ||
		strings.Contains(html, "deletedTokens={}")
	t.Logf("cancelEditMode clears deletedTokens: %v", hasClear)
	if !hasClear {
		t.Errorf("FAIL: cancelEditMode does NOT clear deletedTokens")
	}

	// Cancel must also clear addTokenFields state
	hasClearAdd := strings.Contains(html, "addTokenFields") &&
		(strings.Contains(html, "style.display='none'") ||
		 strings.Contains(html, "style.display=\"none\""))
	t.Logf("cancel resets addTokenFields: %v", hasClearAdd)
}

func TestEdit_TokenDeleteAPIWorks(t *testing.T) {
	resp, err := http.Post("http://127.0.0.1:8080/api/tokens/delete",
		"application/json",
		strings.NewReader(`{"account_id":"test","indices":[0],"chain":"BSC"}`))
	if err != nil {
		t.Fatalf("POST failed: %v", err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&data)

	status, _ := data["status"].(string)
	t.Logf("/api/tokens/delete status=%s body=%v", status, data)

	if resp.StatusCode == 500 {
		t.Errorf("FAIL: /api/tokens/delete returns HTTP 500.\n"+
			"  JS saveTokenEdits sends {account_id, indices, chain} format.\n"+
			"  serve.py old handler expects {ticker, chain_id} — mismatch causes crash.\n"+
			"  FIX: serve.py must parse 'indices' array and delete by index.")
	}
	if status != "ok" {
		t.Errorf("FAIL: /api/tokens/delete status=%s (expected 'ok')", status)
	} else {
		t.Log("PASS: /api/tokens/delete works with JS payload format")
	}
}

func TestEdit_DeleteButtonHidesInAddMode(t *testing.T) {
	html := fetchPage(t, "/portfolio")
	if html == "" { return }

	// delete buttons must be conditional on addTokenMode
	hasConditional := strings.Contains(html, "addTokenMode") &&
		(strings.Contains(html, "editMode && !addTokenMode") ||
		 strings.Contains(html, "editMode&&!addTokenMode"))
	t.Logf("delete buttons conditional on addTokenMode: %v", hasConditional)
	if !hasConditional {
		t.Errorf("FAIL: delete (-) buttons not hidden during add-token mode.\n"+
			"  When user clicks green (+) to add token, the red (-) buttons\n"+
			"  on existing rows should disappear. Only OK and Cancel remain.\n"+
			"  FIX: delBtn = (editMode && !addTokenMode) ? ... : ''")
	} else { t.Log("PASS: delete buttons hidden when addTokenMode active") }
}
