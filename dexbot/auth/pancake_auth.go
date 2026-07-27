/******************************************************************************
 * File Name       : pancake_auth.go
 * File Path       : auth/pancake_auth.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.1
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:35 (UTC+7)
 * Modified Date   : 2026-07-02 19:15:00 (UTC+7)
 *
 * Description     :
 *    Dexbot component — auto-documented per rule1.txt.
 *
 * Responsibilities:
 *    - Implement core functionality for auth package with multi-chain features.
 *
 * Usage :
 *    Directory : auth/
 *
 *    Build :
 *       go build ./auth
 *
 *    Run :
 *       go run .  (from dexbot root)
 *
 *    Test :
 *       go test ./auth
 *
 * Dependencies :
 *    Internal :
 *       - dexbot/auth
 *
 *    External :
 *       - (stdlib only)
 *
 * Configuration :
 *    - config.env
 *
 * Updated Parts  :
 *    - Added ConnectToChain and GetWalletForChain to cleanly allow multi-chain support
 *
 * New Parts      :
 *    [Functions] ConnectToChain, GetWalletForChain
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)      | Author          | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-01 19:25:35    | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *    1.0.1   | 2026-07-02 19:15:00    | Gemini          | Added multi-chain RPC extensions
 *    -------------------------------------------------------------------------
 *
 * TODO :
 *    - Add unit tests
 *
 * Notes :
 *    - Per rule1.txt coding standard.
 ******************************************************************************/
package auth

import (
    "context"
    "log"
    "math/big"
    "os"
    "strings"

    "github.com/ethereum/go-ethereum/accounts/abi/bind"
    "github.com/ethereum/go-ethereum/crypto"
    "github.com/ethereum/go-ethereum/ethclient"
)
/******************************************************************************
 * Function Name : LoadPrivateKey
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/


func LoadPrivateKey() string {
    data, err := os.ReadFile("config.env")
    if err != nil {
        log.Fatal(err)
    }

    for _, line := range strings.Split(string(data), "\n") {
        if strings.HasPrefix(line, "PRIVATE_KEY=") {
            return strings.TrimPrefix(line, "PRIVATE_KEY=")
        }
    }
    return ""
}

// Connect retains backward compatibility with all apps (Defaults to BSC)
/******************************************************************************
 * Function Name : Connect
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func Connect() *ethclient.Client {
    client, err := ethclient.Dial("https://bsc-dataseed.binance.org/")
    if err != nil {
        log.Fatal(err)
    }
    return client
}

// GetWallet retains backward compatibility with all apps (Defaults to Chain ID 56)
/******************************************************************************
 * Function Name : GetWallet
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func GetWallet(client *ethclient.Client, pk string) *bind.TransactOpts {
    privateKey, _ := crypto.HexToECDSA(pk)
    chainID := big.NewInt(56)

    auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
    if err != nil {
        log.Fatal(err)
    }

    nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
    gasPrice, _ := client.SuggestGasPrice(context.Background())

    auth.Nonce = big.NewInt(int64(nonce))
    auth.Value = big.NewInt(0)
    auth.GasPrice = gasPrice

    return auth
}

// ============================================================================
// NEW MULTI-CHAIN COMPATIBLE EXTENSIONS (Will not break existing legacy apps)
// ============================================================================

// ConnectToChain initializes an ethclient interface dynamically to any RPC link
/******************************************************************************
 * Function Name : ConnectToChain
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func ConnectToChain(rpcURL string) *ethclient.Client {
    client, err := ethclient.Dial(rpcURL)
    if err != nil {
        log.Fatal("Failed connecting to RPC endpoint: ", err)
    }
    return client
}

// GetWalletForChain safely targets the customized target ChainID context 
/******************************************************************************
 * Function Name : GetWalletForChain
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func GetWalletForChain(client *ethclient.Client, pk string, targetChainID int64) *bind.TransactOpts {
    privateKey, _ := crypto.HexToECDSA(pk)
    chainID := big.NewInt(targetChainID)

    auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
    if err != nil {
        log.Fatal("Failed assigning targeted transactor context: ", err)
    }

    nonce, _ := client.PendingNonceAt(context.Background(), auth.From)
    gasPrice, _ := client.SuggestGasPrice(context.Background())

    auth.Nonce = big.NewInt(int64(nonce))
    auth.Value = big.NewInt(0)
    auth.GasPrice = gasPrice

    return auth
}