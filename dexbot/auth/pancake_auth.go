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
