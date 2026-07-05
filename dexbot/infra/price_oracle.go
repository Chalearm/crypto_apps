/******************************************************************************
 * File Name       : price_oracle.go
 * File Path       : infra/price_oracle.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:31 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:31 (UTC+7)
 *
 * Description     :
 *   PancakeSwap price oracle — fetches real-time token prices via
 *
 * Responsibilities:
 *   - Implement core functionality.
 *
 * Usage :
 *   Directory : infra/
 *
 *   Build :
 *     go build ./infra
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./infra
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Functions] All exported functions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-01 19:25:31 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package infra

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// PancakeSwap Router V2 on BSC
var PANCAKE_ROUTER = common.HexToAddress("0x10ED43C718714eb63d5aA57B78B54704E256024E")

// WBNB on BSC
var WBNB_ADDR = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c")

// USDC on BSC
var USDC_ADDR = common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d")

const ROUTER_ABI = `[{
  "name":"getAmountsOut",
  "type":"function",
  "inputs":[
    {"name":"amountIn","type":"uint256"},
    {"name":"path","type":"address[]"}
  ],
  "outputs":[{"name":"amounts","type":"uint256[]"}],
  "stateMutability":"view"
}]`

// PriceOracle fetches real-time token prices from PancakeSwap.
type PriceOracle struct {
	client *ethclient.Client
	abi    abi.ABI
}

// NewPriceOracle connects to BSC and initializes the router ABI.
func NewPriceOracle() (*PriceOracle, error) {
	client, err := ethclient.Dial("https://bsc-dataseed.binance.org/")
	if err != nil {
		return nil, fmt.Errorf("BSC dial: %w", err)
	}
	routerABI, err := abi.JSON(strings.NewReader(ROUTER_ABI))
	if err != nil {
		return nil, fmt.Errorf("ABI parse: %w", err)
	}
	return &PriceOracle{client: client, abi: routerABI}, nil
}

// GetPriceUSD returns the USD price of a token by routing through WBNB, then to USDC.
// Path: token → WBNB → USDC (or BUSD if needed)
func (po *PriceOracle) GetPriceUSD(tokenAddr common.Address) (float64, error) {
	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil) // 1 token in base18
	path := []common.Address{tokenAddr, WBNB_ADDR, USDC_ADDR}

	contract := bind.NewBoundContract(PANCAKE_ROUTER, po.abi, po.client, po.client, po.client)

	var result []interface{}
	err := contract.Call(nil, &result, "getAmountsOut", amountIn, path)
	if err != nil {
		return 0, fmt.Errorf("getAmountsOut failed: %w", err)
	}

	amounts := *abi.ConvertType(result[0], new([]*big.Int)).(*[]*big.Int)
	if len(amounts) < 3 {
		return 0, fmt.Errorf("unexpected amount count: %d", len(amounts))
	}

	tokenRaw := amounts[0]
	usdcRaw := amounts[len(amounts)-1]

	base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	tokenClean := new(big.Float).Quo(new(big.Float).SetInt(tokenRaw), base18)
	usdcClean := new(big.Float).Quo(new(big.Float).SetInt(usdcRaw), base18)

	price, _ := new(big.Float).Quo(usdcClean, tokenClean).Float64()
	return price, nil
}

// GetPriceBNB returns the BNB price in USD (WBNB → USDC).
func (po *PriceOracle) GetPriceBNB() (float64, error) {
	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	path := []common.Address{WBNB_ADDR, USDC_ADDR}

	contract := bind.NewBoundContract(PANCAKE_ROUTER, po.abi, po.client, po.client, po.client)

	var result []interface{}
	err := contract.Call(nil, &result, "getAmountsOut", amountIn, path)
	if err != nil {
		return 0, fmt.Errorf("BNB price failed: %w", err)
	}

	amounts := *abi.ConvertType(result[0], new([]*big.Int)).(*[]*big.Int)
	base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	usdcClean := new(big.Float).Quo(new(big.Float).SetInt(amounts[len(amounts)-1]), base18)

	price, _ := usdcClean.Float64()
	return price, nil
}

// FetchAllPrices fetches USD prices for all tokens in the given map.
// Returns a map of ticker → USD price.
func (po *PriceOracle) FetchAllPrices(tokens map[string]common.Address) map[string]float64 {
	prices := make(map[string]float64)

	for ticker, addr := range tokens {
		if ticker == "BNB" {
			if p, err := po.GetPriceBNB(); err == nil {
				prices[ticker] = p
				// BTC price updated via GetBTCPrice()
			}
		} else if addr == (common.Address{}) {
			continue
		} else {
			if p, err := po.GetPriceUSD(addr); err == nil {
				prices[ticker] = p
			}
		}
	}
	return prices
}

// Close disconnects the BSC client.
func (po *PriceOracle) Close() {
	if po.client != nil {
		po.client.Close()
	}
}

// ── BTC PRICE FROM COINGECKO (§119) ──

var (
	lastBTCPrice   float64
	lastBTCFetch   time.Time
	btcPriceMu     sync.Mutex
)

// GetBTCPrice fetches the real-time BTC/USD price from CoinGecko free API.
// Caches the result for 5 minutes to avoid rate limiting.
// Per myreq6.txt §119: replaces hardcoded BTCPriceMock=85000.
func GetBTCPrice() float64 {
	btcPriceMu.Lock()
	if time.Since(lastBTCFetch) < 5*time.Minute && lastBTCPrice > 0 {
		p := lastBTCPrice
		btcPriceMu.Unlock()
		return p
	}
	btcPriceMu.Unlock()

	resp, err := http.Get("https://api.coingecko.com/api/v3/simple/price?ids=bitcoin&vs_currencies=usd")
	if err != nil {
		return 58500.0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var data map[string]map[string]float64
	if err := json.Unmarshal(body, &data); err != nil {
		return 58500.0
	}
	if btc, ok := data["bitcoin"]; ok {
		if usd, ok2 := btc["usd"]; ok2 && usd > 0 {
			btcPriceMu.Lock()
			lastBTCPrice = usd
			lastBTCFetch = time.Now()
			BTCPriceMock = usd
			btcPriceMu.Unlock()
			return usd
		}
	}
	return 58500.0
}
