/******************************************************************************
 * File Name       : pancake_auth.go
 * File Path       : auth/pancake_auth.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   Dexbot component — auto-documented per rule1.txt.
 *
 * Responsibilities:
 *   - Implement core functionality for auth package.
 *
 * Usage :
 *   Directory : auth/
 *
 *   Build :
 *     go build ./auth
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./auth
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/auth
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
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
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

func Connect() *ethclient.Client {
    client, err := ethclient.Dial("https://bsc-dataseed.binance.org/")
    if err != nil {
        log.Fatal(err)
    }
    return client
}

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
