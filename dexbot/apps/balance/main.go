package main

import (
    "dexbot/auth"
    "dexbot/balance"
    "dexbot/tokens"
)

func main() {

    client := auth.Connect()
    pk := auth.LoadPrivateKey()
    wallet := auth.GetWallet(client, pk)

    balance.Report(client, wallet, tokens.Tokens)
}

