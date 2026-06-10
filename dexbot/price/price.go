package price

import (
    "fmt"
)

func FormatPrice(amount float64, base string) string {

    switch base {
    case "USD":
        return fmt.Sprintf("$%.6f", amount)

    case "BNB":
        bnb := amount / 600.0
        return fmt.Sprintf("%.8f BNB", bnb)

    case "BTC":
        btc := amount / 70000.0
        return fmt.Sprintf("%.8f BTC", btc)
    }

    return fmt.Sprintf("%.6f", amount)
}
