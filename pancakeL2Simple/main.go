/*
Filename: main.go
Author: Gemini (Updated for Chalearm Saelim)
Version: v1.4
Date: 2026-06-15

Description:
Core orchestration entry point for the opBNB Automated trading engine.
Optimized for PancakeSwap V3 / Layer 2 infrastructure.
*/

package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

/// =======================
/// USER VARIABLES (opBNB MAINNET)
/// =======================
var (
	// Official opBNB Mainnet Contracts
	USDT_ADDRESS  = common.HexToAddress("0x9e5aac1ba1a2e6aed6b32689dfcf62a509ca93f3") // Native USDT on opBNB
	WBNB_ADDRESS  = common.HexToAddress("0x4200000000000000000000000000000000000006") // Wrapped BNB on opBNB
	FDUSD_ADDRESS = common.HexToAddress("0x50c5725949a6f0af7a41251666458e80357fb29a") // FDUSD on opBNB
	
	// PancakeSwap V3 Smart Router on opBNB
	ROUTER_ADDR   = common.HexToAddress("0x13f4Db831088734324b693325161A97ef2227469") 
)

/// =======================
/// HIGH-PRECISION VISUAL FORMATTER
/// =======================
func formatWithSpacedDecimals(val float64) string {
	rawStr := fmt.Sprintf("%.12f", val)
	parts := strings.Split(rawStr, ".")

	intPart := parts[0]
	decPart := parts[1]

	var intResult []string
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			intResult = append(intResult, ",")
		}
		intResult = append(intResult, string(c))
	}
	formattedInt := strings.Join(intResult, "")

	var decResult []string
	for i, c := range decPart {
		if i > 0 && i%3 == 0 {
			decResult = append(decResult, " ")
		}
		decResult = append(decResult, string(c))
	}
	formattedDec := strings.Join(decResult, "")

	return formattedInt + "." + formattedDec
}

/// =======================
/// LOAD PRIVATE KEY
/// =======================
func loadPrivateKey() string {
	data, err := os.ReadFile("config.env")
	if err != nil {
		log.Fatal("Failed to read config.env: ", err)
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PRIVATE_KEY=") {
			return strings.TrimPrefix(line, "PRIVATE_KEY=")
		}
	}
	return ""
}

/// =======================
/// CONNECT TO opBNB (L2)
/// =======================
func connect() *ethclient.Client {
	client, err := ethclient.Dial("https://opbnb-mainnet-rpc.bnbchain.org/")
	if err != nil {
		log.Fatal("Failed connecting to opBNB network: ", err)
	}
	return client
}

/// =======================
/// GET WALLET (opBNB Chain ID 204)
/// =======================
func getWallet(client *ethclient.Client, pk string) (*bind.TransactOpts, common.Address) {
	privateKey, err := crypto.HexToECDSA(pk)
	if err != nil {
		log.Fatal("Invalid private key: ", err)
	}
	
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Error casting public key structure")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	chainID := big.NewInt(204) // opBNB Mainnet ChainID
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatal(err)
	}
	return auth, fromAddress
}

/// =======================
/// GET PRICE (MOCK FOR V3 PRICING STRUCTURE)
/// =======================
func getPriceUSDTtoFDUSD(client *ethclient.Client) float64 {
	// Simple pricing simulation layer for safety. 
	// Real V3 price fetching queries the Pancake V3 Quoter contract.
	return 1.00 
}

/// =======================
/// ABIs
/// =======================
const PANCAKE_ROUTER_ABI = `[
{
  "name": "exactInputSingle",
  "type": "function",
  "inputs": [
    {
      "components": [
        {"name": "tokenIn", "type": "address"},
        {"name": "tokenOut", "type": "address"},
        {"name": "fee", "type": "uint24"},
        {"name": "recipient", "type": "address"},
        {"name": "deadline", "type": "uint256"},
        {"name": "amountIn", "type": "uint256"},
        {"name": "amountOutMinimum", "type": "uint256"},
        {"name": "sqrtPriceLimitX96", "type": "uint160"}
      ],
      "name": "params",
      "type": "tuple"
    }
  ],
  "outputs": [{"name": "amountOut", "type": "uint256"}],
  "stateMutability": "payable"
}
]`

const ERC20_ABI = `[
{
  "name": "approve",
  "type": "function",
  "inputs": [
    {"name":"spender","type":"address"},
    {"name":"amount","type":"uint256"}
  ],
  "outputs":[{"name":"","type":"bool"}]
},
{
  "name": "balanceOf",
  "type": "function",
  "inputs": [
    {"name":"account","type":"address"}
  ],
  "outputs":[{"name":"balance","type":"uint256"}],
  "stateMutability": "view"
},
{
  "name": "allowance",
  "type": "function",
  "inputs": [
    {"name":"owner","type":"address"},
    {"name":"spender","type":"address"}
  ],
  "outputs":[{"name":"","type":"uint256"}],
  "stateMutability": "view"
}
]`

/// =======================
/// ERC20 HELPER FUNCTIONS
/// =======================
func getERC20Balance(client *ethclient.Client, token common.Address, owner common.Address) *big.Int {
	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20_ABI))
	contract := bind.NewBoundContract(token, erc20ABI, client, client, client)

	var result []interface{}
	err := contract.Call(nil, &result, "balanceOf", owner)
	if err != nil {
		log.Printf("Warning: Failed to fetch balance for token %s: %v", token.Hex(), err)
		return big.NewInt(0)
	}

	return result[0].(*big.Int)
}

func enforceAllowance(client *ethclient.Client, auth *bind.TransactOpts, token common.Address, spender common.Address, requiredAmount *big.Int) {
	erc20ABI, _ := abi.JSON(strings.NewReader(ERC20_ABI))
	contract := bind.NewBoundContract(token, erc20ABI, client, client, client)

	var result []interface{}
	err := contract.Call(nil, &result, "allowance", auth.From, spender)
	if err != nil {
		log.Fatal("Failed to check allowance:", err)
	}
	
	currentAllowance := result[0].(*big.Int)
	
	if currentAllowance.Cmp(requiredAmount) < 0 {
		fmt.Printf("🔒 Allowance too low. Approving Router contract...\n")
		
		gasPrice, err := client.SuggestGasPrice(context.Background())
		if err == nil {
			auth.GasPrice = gasPrice
		}

		maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
		
		tx, err := contract.Transact(auth, "approve", spender, maxUint256)
		if err != nil {
			log.Fatal("Token approval transaction failed: ", err)
		}
		fmt.Printf("⏳ Approval TX broadcasted: %s. Waiting for confirmation...\n", tx.Hash().Hex())
		time.Sleep(5 * time.Second)
	}
}

/// =======================
/// GAS REPORT GENERATOR
/// =======================
func reportGas(client *ethclient.Client, tx *types.Transaction, gasPrice *big.Int) {
	receipt, err := bind.WaitMined(context.Background(), client, tx)
	if err != nil {
		return
	}

	gasUsed := receipt.GasUsed
	totalWei := new(big.Int).Mul(big.NewInt(int64(gasUsed)), gasPrice)

	bnb := new(big.Float).Quo(new(big.Float).SetInt(totalWei), big.NewFloat(1e18))
	bnbF, _ := bnb.Float64()
	usd := bnbF * 600

	prettyBNBCost := formatWithSpacedDecimals(bnbF)
	prettyUSDCost := formatWithSpacedDecimals(usd)

	fmt.Println("Gas Units Consumed:", gasUsed)
	fmt.Printf("Gas Cost Metrics: %s BNB (~$%s USD)\n", prettyBNBCost, prettyUSDCost)
}

/// =======================
/// BUY FDUSD WITH USDT (V3 Execution)
/// =======================
func buyTokens(client *ethclient.Client, auth *bind.TransactOpts, amountUSDT float64) {
	fmt.Println("🟢 Checking Wallet Account Status...")

	usdtRaw := getERC20Balance(client, USDT_ADDRESS, auth.From)
	fdusdRawBefore := getERC20Balance(client, FDUSD_ADDRESS, auth.From) 
	bnbRaw, err := client.BalanceAt(context.Background(), auth.From, nil)   
	if err != nil {
		log.Fatal("Failed to fetch BNB balance:", err)
	}

	base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	usdtClean, _ := new(big.Float).Quo(new(big.Float).SetInt(usdtRaw), base18).Float64()
	fdusdCleanBefore, _ := new(big.Float).Quo(new(big.Float).SetInt(fdusdRawBefore), base18).Float64()
	bnbClean, _ := new(big.Float).Quo(new(big.Float).SetInt(bnbRaw), base18).Float64()

	fmt.Println("-------------------------------------------")
	fmt.Printf("💰 USDT Balance:  %s USDT\n", formatWithSpacedDecimals(usdtClean))
	fmt.Printf("🪙 FDUSD Balance: %s FDUSD\n", formatWithSpacedDecimals(fdusdCleanBefore))
	fmt.Printf("⛽ BNB Balance:   %s BNB\n", formatWithSpacedDecimals(bnbClean))
	fmt.Println("-------------------------------------------")

	bigAmt := new(big.Float).Mul(big.NewFloat(amountUSDT), base18)
	amountIn := new(big.Int)
	bigAmt.Int(amountIn)

	if usdtRaw.Cmp(amountIn) < 0 {
		log.Println("❌ Insufficient USDT balance to execute swap order test.")
		return
	}

	enforceAllowance(client, auth, USDT_ADDRESS, ROUTER_ADDR, amountIn)

	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err == nil {
		auth.GasPrice = gasPrice
	}

	routerABI, err := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))       
	if err != nil {
		log.Fatal(err)
	}
	contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)

	// Packed parameters matching PancakeSwap V3 Struct rules
	params := struct {
		TokenIn           common.Address
		TokenOut          common.Address
		Fee               *big.Int
		Recipient         common.Address
		Deadline          *big.Int
		AmountIn          *big.Int
		AmountOutMinimum  *big.Int
		SqrtPriceLimitX96 *big.Int
	}{
		TokenIn:           USDT_ADDRESS,
		TokenOut:          FDUSD_ADDRESS,
		Fee:               big.NewInt(2500), // 0.25% fee tier typical for stables on V3
		Recipient:         auth.From,
		Deadline:          big.NewInt(time.Now().Add(5 * time.Minute).Unix()),
		AmountIn:          amountIn,
		AmountOutMinimum:  big.NewInt(0),
		SqrtPriceLimitX96: big.NewInt(0),
	}

	fmt.Printf("⚡ Sending L2 exactInputSingle swap order for %s USDT...\n", formatWithSpacedDecimals(amountUSDT))
	tx, err := contract.Transact(auth, "exactInputSingle", params)
	if err != nil {
		log.Fatal("Swap transaction execution failed: ", err)
	}
	fmt.Println("✅ Swap TX sent successfully:", tx.Hash().Hex())

	reportGas(client, tx, gasPrice)
}

/// =======================
/// MONITOR PRICE LOOP
/// =======================
func monitor(client *ethclient.Client, initial float64) {
	fmt.Printf("🎯 Tracking Started. Base Price: %.2f FDUSD/USDT\n", initial) 
	for i := 0; i < 3; i++ { 
		current := getPriceUSDTtoFDUSD(client)
		change := (current - initial) / initial * 100

		fmt.Printf("📊 Market Price: %.2f FDUSD | Change: %.4f%%\n", current, change)
		time.Sleep(2 * time.Second)
	}
}

/// =======================
/// ENTRYPOINT
/// =======================
func main() {
	pk := loadPrivateKey()
	if pk == "" {
		log.Fatal("Private key target empty in config.env")
	}

	client := connect()
	auth, _ := getWallet(client, pk)

	// Executing test swap trying to spend 0.00005 USDT
	buyTokens(client, auth, 0.00005)

	initial := getPriceUSDTtoFDUSD(client)
	monitor(client, initial)
}