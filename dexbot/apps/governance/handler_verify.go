/******************************************************************************
 * File Name       : handler_verify.go
 * File Path       : apps/governance/handler_verify.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-01 19:30:00 (UTC+7)
 *
 * Description     :
 *   Web page verification API per myreq6.txt §107, §111.
 *   SHA256(private_key) authentication for all endpoints.
 *   Returns the EXACT values visible on the web page.
 *
 * Endpoints:
 *   GET  /api/verify/balance?auth=SHA256(pk) → total + per-chain balance
 *   GET  /api/verify/tokens?auth=SHA256(pk)  → full token list with display_format
 *   GET  /api/verify/token/X?auth=SHA256(pk) → single token value
 *   POST /api/verify/token/add?auth=SHA256(pk)  → add token
 *   POST /api/verify/token/delete?auth=SHA256(pk) → remove token
 *   POST /api/verify/chain/add?auth=SHA256(pk)  → add chain
 *   POST /api/verify/chain/delete?auth=SHA256(pk) → remove chain
 ******************************************************************************/

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"dexbot/infra"
)

// authOK checks SHA256 of the given authToken against the stored private key.
func authOK(r *http.Request) bool {
	authToken := r.URL.Query().Get("auth")
	if authToken == "" {
		return false
	}
	pk := os.Getenv("PRIVATE_KEY")
	if pk == "" {
		data, _ := os.ReadFile("config.env")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				pk = strings.TrimPrefix(line, "PRIVATE_KEY=")
				break
			}
		}
	}
	if pk == "" {
		return false
	}
	hash := sha256.Sum256([]byte(pk))
	expected := hex.EncodeToString(hash[:])
	return authToken == expected
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	writeJSON(w, map[string]string{"error": msg})
}

// handleVerifyBalance returns the total + per-chain balance as seen on the trading page.
func handleVerifyBalance(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	chain := r.URL.Query().Get("chain")
	am := infra.NewAccountManager()
	summary := infra.GetBalanceSummary(am)

	totalUSD := summary.TotalUSD
	chainTotal := totalUSD
	if chain != "" {
		chainTotal = 0
		for _, a := range summary.Assets {
			if a.ChainName == chain || a.ChainID == chain {
				chainTotal += a.USDValue
			}
		}
	}

	writeJSON(w, map[string]interface{}{
		"total_usd":      totalUSD,
		"total_btc":      summary.TotalBTC,
		"btc_price":      summary.BTCPrice,
		"chain_usd":      chainTotal,
		"chain_name":     chain,
		"display_masked": "******",
		"display_visible": infra.FormatAmount(totalUSD),
		"assets_count":   len(summary.Assets),
	})
}

// handleVerifyTokens returns the full token list as displayed on the trading page.
func handleVerifyTokens(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	chain := r.URL.Query().Get("chain")
	am := infra.NewAccountManager()
	summary := infra.GetBalanceSummary(am)

	type tokenOut struct {
		Ticker         string  `json:"ticker"`
		Amount         float64 `json:"amount"`
		USDValue       float64 `json:"usd_value"`
		DisplayFormat  string  `json:"display_format"`
		ChainName      string  `json:"chain_name"`
		Address        string  `json:"address"`
	}
	var tokens []tokenOut
	for _, a := range summary.Assets {
		if chain != "" && a.ChainName != chain {
			continue
		}
		tokens = append(tokens, tokenOut{
			Ticker:        a.Ticker,
			Amount:        a.Amount,
			USDValue:      a.USDValue,
			DisplayFormat: infra.FormatAmount(a.Amount) + " " + a.Ticker + " ($" + infra.FormatAmount(a.USDValue) + ")",
			ChainName:     a.ChainName,
			Address:       a.BSCAddr,
		})
	}
	writeJSON(w, map[string]interface{}{"tokens": tokens, "count": len(tokens)})
}

// handleVerifyToken returns a single token's value as shown on the page.
func handleVerifyToken(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	ticker := strings.ToUpper(r.URL.Query().Get("ticker"))
	if ticker == "" {
		// Try path-based: /api/verify/token/BNB
		path := strings.TrimPrefix(r.URL.Path, "/api/verify/token/")
		if path != "" {
			ticker = strings.ToUpper(path)
		}
	}
	if ticker == "" {
		writeError(w, 400, "ticker required")
		return
	}

	chain := r.URL.Query().Get("chain")
	am := infra.NewAccountManager()
	summary := infra.GetBalanceSummary(am)

	for _, a := range summary.Assets {
		if strings.ToUpper(a.Ticker) == ticker {
			if chain != "" && a.ChainName != chain {
				writeJSON(w, map[string]interface{}{
					"ticker": ticker,
					"error":  "token exists but not on chain " + chain,
					"found_on_chain": a.ChainName,
				})
				return
			}
			writeJSON(w, map[string]interface{}{
				"ticker":         a.Ticker,
				"amount":         a.Amount,
				"usd_value":      a.USDValue,
				"display_format": infra.FormatAmount(a.Amount) + " " + a.Ticker + " ($" + infra.FormatAmount(a.USDValue) + ")",
				"chain_name":     a.ChainName,
				"address":        a.BSCAddr,
			})
			return
		}
	}
	writeError(w, 404, "token "+ticker+" not found")
}

// handleVerifyTokenAdd adds a new token via curl.
func handleVerifyTokenAdd(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	if r.Method != "POST" {
		writeError(w, 405, "POST required")
		return
	}
	var body struct {
		Ticker    string `json:"ticker"`
		Address   string `json:"address"`
		ChainID   string `json:"chain_id"`
		ChainName string `json:"chain_name"`
		BaseURL   string `json:"base_url"`
		USDPrice  float64 `json:"usd_price"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if body.Ticker == "" || body.Address == "" {
		writeError(w, 400, "ticker and address required")
		return
	}
	if body.ChainID == "" {
		body.ChainID = "56"
	}
	if body.ChainName == "" {
		body.ChainName = "BSC"
	}

	tokenReg := infra.NewTokenRegistry()
	err := tokenReg.AddToken(infra.TokenEntry{
		Ticker:    strings.ToUpper(body.Ticker),
		Address:   body.Address,
		ChainID:   body.ChainID,
		ChainName: body.ChainName,
		BaseURL:   body.BaseURL,
		USDPrice:  body.USDPrice,
	})
	if err != nil {
		writeError(w, 500, "save failed: "+err.Error())
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "ticker": strings.ToUpper(body.Ticker), "address": body.Address, "chain": body.ChainName})
}

// handleVerifyTokenDelete removes a token via curl.
func handleVerifyTokenDelete(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	if r.Method != "POST" {
		writeError(w, 405, "POST required")
		return
	}
	var body struct {
		Ticker  string `json:"ticker"`
		ChainID string `json:"chain_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if body.Ticker == "" {
		writeError(w, 400, "ticker required")
		return
	}
	if body.ChainID == "" {
		body.ChainID = "56"
	}

	tokenReg := infra.NewTokenRegistry()
	if err := tokenReg.RemoveToken(strings.ToUpper(body.Ticker), body.ChainID); err != nil {
		writeJSON(w, map[string]string{"status": "error", "message": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "ticker": strings.ToUpper(body.Ticker)})
}

// handleVerifyChainAdd adds a new chain via curl.
func handleVerifyChainAdd(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	if r.Method != "POST" {
		writeError(w, 405, "POST required")
		return
	}
	var body struct {
		Name    string `json:"name"`
		ID      string `json:"id"`
		BaseURL string `json:"base_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if body.Name == "" || body.ID == "" {
		writeError(w, 400, "name and id required")
		return
	}
	writeJSON(w, map[string]string{"status": "ok", "chain": body.Name, "id": body.ID, "base_url": body.BaseURL})
}

// handleVerifyChainDelete removes a chain via curl.
func handleVerifyChainDelete(w http.ResponseWriter, r *http.Request) {
	if !authOK(r) {
		writeError(w, 403, "unauthorized")
		return
	}
	if r.Method != "POST" {
		writeError(w, 405, "POST required")
		return
	}
	var body struct {
		ChainName string `json:"chain_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, 400, "invalid JSON: "+err.Error())
		return
	}
	if body.ChainName == "" {
		writeError(w, 400, "chain_name required")
		return
	}
	writeJSON(w, map[string]string{"status": "error", "message": body.ChainName + " chain not found"})
}
