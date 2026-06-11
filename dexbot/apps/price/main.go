package main

import (
    "fmt"
    "dexbot/price"
)

func main() {

    shibUSD := 0.000025

    usd := price.FormatPrice(shibUSD, "USD")
    bnb := price.FormatPrice(shibUSD, "BNB")

    fmt.Println("SHIBA PRICE:")
    fmt.Printf("%s (%s)\n", usd, bnb)
}
