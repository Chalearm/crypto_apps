package main

import (
        "context"
        "fmt"
        "log"
        "math/big"
        "os"
        "strings"
        "time"

        "github.com/ethereum/go-ethereum/accounts/abi"
        "github.com/ethereum/go-ethereum/accounts/abi/bind"
        "github.com/ethereum/go-ethereum/common"
        "github.com/ethereum/go-ethereum/crypto"
        "github.com/ethereum/go-ethereum/ethclient"
)

/// =======================
/// USER VARIABLES
/// =======================
var (
        USDC_ADDRESS = common.HexToAddress("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d") // BSC USDC (18 Decimals)
        WBNB_ADDRESS = common.HexToAddress("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c") // BSC WBNB (Routing Bridge)
        BTT_ADDRESS  = common.HexToAddress("0x352Cb5E19b12FC216548a2677bD0fce83BaE434B") // BSC BTT (18 Decimals)
        ROUTER_ADDR  = common.HexToAddress("0x10ED43C718714eb63d5aA57B78B54704E256024E") // Pancake Router
)

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
/// CONNECT TO BSC
/// =======================
func connect() *ethclient.Client {
        client, err := ethclient.Dial("https://bsc-dataseed.binance.org/")      
        if err != nil {
                log.Fatal(err)
        }
        return client
}

/// =======================
/// GET WALLET
/// =======================
func getWallet(client *ethclient.Client, pk string) (*bind.TransactOpts, common.Address) {
        privateKey, err := crypto.HexToECDSA(pk)
        if err != nil {
                log.Fatal("Invalid private key: ", err)
        }
        chainID := big.NewInt(56)
        auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)    
        if err != nil {
                log.Fatal(err)
        }
        return auth, auth.From
}

/// =======================
/// GET PRICE (ROUTED THROUGH WBNB)
/// =======================
func getPriceUSDCtoBTT(client *ethclient.Client) float64 {
        amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)       
        path := []common.Address{USDC_ADDRESS, WBNB_ADDRESS, BTT_ADDRESS}       

        routerABI, err := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))       
        if err != nil {
                log.Fatal(err)
        }

        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)

        var result []interface{}
        err = contract.Call(nil, &result, "getAmountsOut", amountIn, path)      
        if err != nil {
                log.Fatal("Price fetch failed (verify liquidity path): ", err)  
        }

        amounts := *abi.ConvertType(result[0], new([]*big.Int)).(*[]*big.Int)   
        usdcRaw := amounts[0]
        bttRaw := amounts[len(amounts)-1]

        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        usdcClean := new(big.Float).Quo(new(big.Float).SetInt(usdcRaw), base18) 
        bttClean := new(big.Float).Quo(new(big.Float).SetInt(bttRaw), base18)   

        price, _ := new(big.Float).Quo(bttClean, usdcClean).Float64()
        return price
}

/// =======================
/// ABIs
/// =======================
const PANCAKE_ROUTER_ABI = `[
{
  "name": "swapExactTokensForTokens",
  "type": "function",
  "inputs": [
    {"name":"amountIn","type":"uint256"},
    {"name":"amountOutMin","type":"uint256"},
    {"name":"path","type":"address[]"},
    {"name":"to","type":"address"},
    {"name":"deadline","type":"uint256"}
  ],
  "outputs":[{"name":"amounts","type":"uint256[]"}]
},
{
  "name": "getAmountsOut",
  "type": "function",
  "inputs": [
    {"name":"amountIn","type":"uint256"},
    {"name":"path","type":"address[]"}
  ],
  "outputs":[{"name":"amounts","type":"uint256[]"}],
  "stateMutability": "view"
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
                log.Fatal("Failed to fetch token balance:", err)
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
        
        // If allowance is lower than what we want to transfer, send an approval transaction
        if currentAllowance.Cmp(requiredAmount) < 0 {
                fmt.Printf("🔒 Allowance too low. Approving Router contract to manage your tokens...\n")
                
                gasPrice, err := client.SuggestGasPrice(context.Background())
                if err == nil {
                        auth.GasPrice = gasPrice
                }

                // Unlimited approval (2^256 - 1) so you only pay this gas fee once
                maxUint256 := new(big.Int).Sub(new(big.Int).Exp(big.NewInt(2), big.NewInt(256), nil), big.NewInt(1))
                
                tx, err := contract.Transact(auth, "approve", spender, maxUint256)
                if err != nil {
                        log.Fatal("Token approval transaction failed: ", err)
                }
                fmt.Printf("⏳ Approval TX broadcasted: %s. Waiting for block inclusion...\n", tx.Hash().Hex())
                
                // Allow the network block state 10 seconds to catch up
                time.Sleep(10 * time.Second)
        }
}

/// =======================
/// BUY BTT
/// =======================
func buyBTT(client *ethclient.Client, auth *bind.TransactOpts) {
        fmt.Println("🟢 Checking Wallet Account Status...")

        usdcRaw := getERC20Balance(client, USDC_ADDRESS, auth.From)
        bttRaw := getERC20Balance(client, BTT_ADDRESS, auth.From)
        bnbRaw, err := client.BalanceAt(context.Background(), auth.From, nil)   
        if err != nil {
                log.Fatal("Failed to fetch BNB balance:", err)
        }

        base18 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
        usdcClean, _ := new(big.Float).Quo(new(big.Float).SetInt(usdcRaw), base18).Float64()
        bttClean, _ := new(big.Float).Quo(new(big.Float).SetInt(bttRaw), base18).Float64()
        bnbClean, _ := new(big.Float).Quo(new(big.Float).SetInt(bnbRaw), base18).Float64()

        fmt.Println("-------------------------------------------")
        fmt.Printf("💰 USDC Balance: %.5f USDC\n", usdcClean)
        fmt.Printf("🪙 BTT Balance:  %.5f BTT\n", bttClean)
        fmt.Printf("⛽ BNB Balance:  %.5f BNB\n", bnbClean)
        fmt.Println("-------------------------------------------")

        amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil) // 0.01 USDC      
        amountOutMin := big.NewInt(0)
        path := []common.Address{USDC_ADDRESS, WBNB_ADDRESS, BTT_ADDRESS}       
        deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

        // Ensure Router is permitted to take our USDC
        enforceAllowance(client, auth, USDC_ADDRESS, ROUTER_ADDR, amountIn)

        gasPrice, err := client.SuggestGasPrice(context.Background())
        if err == nil {
                auth.GasPrice = gasPrice
        }

        routerABI, err := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))       
        if err != nil {
                log.Fatal(err)
        }
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)

        fmt.Println("⚡ Sending routed swap order for 0.01 USDC...")
        tx, err := contract.Transact(auth, "swapExactTokensForTokens",
                amountIn, amountOutMin, path, auth.From, deadline)
        if err != nil {
                log.Fatal("Swap transaction execution stalled: ", err)
        }
        fmt.Println("✅ Swap TX sent successfully:", tx.Hash().Hex())
}

/// =======================
/// SELL BTT (Liquidates ALL holdings)
/// =======================
func sellBTT(client *ethclient.Client, auth *bind.TransactOpts) {
        fmt.Println("🔴 Triggering asset realization: Selling ALL BTT → USDC")   

        amountIn := getERC20Balance(client, BTT_ADDRESS, auth.From)
        if amountIn.Cmp(big.NewInt(0)) <= 0 {
                fmt.Println("❌ Execution stopped: BTT wallet balance is empty.")
                return
        }

        amountOutMin := big.NewInt(0)
        path := []common.Address{BTT_ADDRESS, WBNB_ADDRESS, USDC_ADDRESS}       
        deadline := big.NewInt(time.Now().Add(5 * time.Minute).Unix())

        // Ensure Router is permitted to take our BTT
        enforceAllowance(client, auth, BTT_ADDRESS, ROUTER_ADDR, amountIn)

        gasPrice, err := client.SuggestGasPrice(context.Background())
        if err == nil {
                auth.GasPrice = gasPrice
        }

        routerABI, err := abi.JSON(strings.NewReader(PANCAKE_ROUTER_ABI))       
        if err != nil {
                log.Fatal(err)
        }
        contract := bind.NewBoundContract(ROUTER_ADDR, routerABI, client, client, client)
        tx, err := contract.Transact(auth, "swapExactTokensForTokens", amountIn, amountOutMin, path, auth.From, deadline)
        if err != nil {
                log.Fatal("Sell execution failed: ", err)
        }
        fmt.Println("✅ Sell TX sent successfully:", tx.Hash().Hex())
}

/// =======================
/// MONITOR PRICE
/// =======================
func monitor(client *ethclient.Client, auth *bind.TransactOpts, initial float64) {
        fmt.Printf("🎯 Tracking Started. Base Price: %.2f BTT/USDC\n", initial) 
        for {
                current := getPriceUSDCtoBTT(client)
                change := (current - initial) / initial * 100

                fmt.Printf("📊 Market Price: %.2f BTT | Change: %.4f%%\n", current, change)

                if change >= 10.0 {
                        fmt.Println("🚀 Target reached (+10% profit)! Initiating liquidation...")
                        sellBTT(client, auth)
                        break
                }
                time.Sleep(5 * time.Second)
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

        // Run balance verification and sample swap
        buyBTT(client, auth)

        // Launch monitoring array
        initial := getPriceUSDCtoBTT(client)
        monitor(client, auth, initial)
}

