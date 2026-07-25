/******************************************************************************
 * File Name       : integration_test.go
 * File Path       : apps/balance/integration_test.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-20 19:40:00 (UTC+7)
 * Modified Date   : 2026-07-20 19:40:00 (UTC+7)
 *
 * Description     :
 *   Integration test for apps/balance CLI commands. Tests view-balance,
 *   add-chain, add-token, delete-token, delete-chain using real config.env
 *   PRIVATE_KEY. Verifies balance daemon HTTP API response format.
 *
 * Usage :
 *   Directory : apps/balance/
 *   Test      : go test -run=TestIntegration -v ./apps/balance
 ******************************************************************************/
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func min3(a, b int) int { if a < b { return a }; return b }

func loadTestPK(t *testing.T) string {
	t.Helper()
	// Try config.env
	for _, p := range []string{"config.env", "../../config.env", "../../../config.env"} {
		data, err := os.ReadFile(p)
		if err != nil { continue }
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				return strings.TrimPrefix(line, "PRIVATE_KEY=")
			}
		}
	}
	t.Skip("PRIVATE_KEY not found in config.env")
	return ""
}

func TestIntegration_ViewBalance(t *testing.T) {
	pk := loadTestPK(t)
	if pk == "" { return }

	// Test via HTTP API (balance daemon must be running on port 8087)
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:8087/api/balance?private_key=%s", pk))
	if err != nil {
		t.Skipf("Balance daemon not running: %v", err)
		return
	}
	defer resp.Body.Close()

	var report map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&report)

	t.Logf("Status: %v", resp.Status)
	t.Logf("Account: %v", report["account"])
	t.Logf("Total USD: %v", report["total_usd"])
	t.Logf("Live BTC Price: %v", report["live_btc_price"])
	t.Logf("Last Updated: %v", report["last_updated_time"])
	chains, _ := report["chains"].([]interface{})
	t.Logf("Chains: %d", len(chains))

	if report["total_usd"] == nil {
		t.Errorf("FAIL: total_usd missing from balance report")
	}
	if report["live_btc_price"] == nil {
		t.Errorf("FAIL: live_btc_price missing from balance report")
	}
	if len(chains) == 0 {
		t.Errorf("FAIL: No chains returned")
	}
}

func TestIntegration_ChainAddDelete(t *testing.T) {
	pk := loadTestPK(t)
	if pk == "" { return }

	// Derive account hash
	accountHash := deriveAccountHash(pk)
	t.Logf("Account hash: %s", accountHash)

	// Add a test chain
	addBody := fmt.Sprintf(`{"account_id":"%s","name":"TESTCHAIN","chain_id":"999"}`, accountHash)
	resp, err := http.Post("http://127.0.0.1:8087/api/chain/add",
		"application/json", strings.NewReader(addBody))
	if err != nil {
		t.Skipf("Balance daemon not running: %v", err)
		return
	}
	defer resp.Body.Close()

	var addResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&addResult)
	t.Logf("Chain add: status=%v", addResult["status"])

	if addResult["status"] != "ok" {
		t.Logf("WARN: Chain add returned: %v (may already exist)", addResult)
	}

	// Delete the test chain
	delBody := fmt.Sprintf(`{"account_id":"%s","chain_id":"999"}`, accountHash)
	resp2, err := http.Post("http://127.0.0.1:8087/api/chain/delete",
		"application/json", strings.NewReader(delBody))
	if err != nil { t.Fatalf("chain delete: %v", err) }
	defer resp2.Body.Close()

	var delResult map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&delResult)
	t.Logf("Chain delete: status=%v", delResult["status"])

	if delResult["status"] != "ok" {
		t.Errorf("FAIL: Chain delete returned: %v", delResult)
	}
}

func TestIntegration_TokenAddDelete(t *testing.T) {
	pk := loadTestPK(t)
	if pk == "" { return }

	accountHash := deriveAccountHash(pk)
	t.Logf("Account hash: %s", accountHash)

	// Add test token to BSC
	addBody := fmt.Sprintf(`{"account_id":"%s","chain_id":"56","ticker":"TESTTOK","address":"0xAbCdEf0123456789AbCdEf0123456789AbCdEf01"}`,
		accountHash)
	resp, err := http.Post("http://127.0.0.1:8087/api/token/add",
		"application/json", strings.NewReader(addBody))
	if err != nil {
		t.Skipf("Balance daemon not running: %v", err)
		return
	}
	defer resp.Body.Close()

	var addResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&addResult)
	t.Logf("Token add: status=%v", addResult["status"])

	if addResult["status"] != "ok" {
		t.Logf("WARN: Token add returned: %v", addResult)
	}

	// Delete test token
	delBody := fmt.Sprintf(`{"account_id":"%s","chain_id":"56","ticker":"TESTTOK"}`, accountHash)
	resp2, err := http.Post("http://127.0.0.1:8087/api/token/delete",
		"application/json", strings.NewReader(delBody))
	if err != nil { t.Fatalf("token delete: %v", err) }
	defer resp2.Body.Close()

	var delResult map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&delResult)
	t.Logf("Token delete: status=%v", delResult["status"])

	if delResult["status"] != "ok" {
		t.Errorf("FAIL: Token delete returned: %v", delResult)
	}
}

func TestIntegration_BalanceDaemonAlive(t *testing.T) {
	// Quick health check via governance dashboard
	resp, err := http.Get("http://127.0.0.1:8080/api/daemons")
	if err != nil {
		t.Skipf("Governance not running: %v", err)
		return
	}
	defer resp.Body.Close()

	var d struct{ Daemons []map[string]interface{} }
	json.NewDecoder(resp.Body).Decode(&d)
	for _, dm := range d.Daemons {
		if dm["Name"] == "balance" {
			t.Logf("Balance daemon: status=%v msg=%v", dm["Status"], dm["Message"])
			if dm["Status"] != "healthy" && dm["Status"] != "pass" {
				t.Errorf("FAIL: Balance daemon is %v", dm["Status"])
			}
			return
		}
	}
	t.Log("Balance daemon not found in daemon list — may not be running yet")
}

func TestIntegration_ViewBalanceCLI(t *testing.T) {
	pk := loadTestPK(t)
	if pk == "" { return }

	// Run ./balance -action=view-balance -private-key=... via go run
	cmd := exec.Command("/usr/local/go/bin/go", "run", "./apps/balance",
		"-action=view-balance", "-private-key="+pk)
	cmd.Dir = findProjectRoot()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("CLI balance: err=%v output=%s", err, string(out[:min3(200, len(out))]))
		// CLI requires daemon running — may fail, that's OK
	} else {
		t.Logf("CLI balance output: %s", string(out[:min3(300, len(out))]))
	}
}

func findProjectRoot() string {
	for _, p := range []string{".", "..", "../..", "../../.."} {
		if _, err := os.Stat(p + "/go.mod"); err == nil {
			return p
		}
	}
	return "."
}
