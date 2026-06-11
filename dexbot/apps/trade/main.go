package main

import (
    "dexbot/auth"
    "dexbot/swap"
    "dexbot/tokens"
)

func main() {
    // Connect to BNB Smart Chain RPC node
    client := auth.Connect()
    pk := auth.LoadPrivateKey()
    wallet := auth.GetWallet(client, pk)

    // Execute the automated strategy routing matrix
    // (This function already sets up and tries the 5000 BTT amounts internally!)
    swap.SwapBTTtoSHIB(client, wallet, tokens.Tokens)
}