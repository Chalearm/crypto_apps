/*
Filename: price_test.go

Test:
✅ DB fallback
✅ non-zero price
*/

package main

import "testing"

/*
Test: fallback works
*/
func TestPriceFallback(t *testing.T) {

    price := GetLatestPrice("UNKNOWN")

    if price <= 0 {
        t.Error("price should fallback > 0")
    }
}

/*
Test: simulate consistency
*/
func TestSimulatePrice(t *testing.T) {

    p := simulatePrice()

    if p == 0 {
        t.Error("simulated price invalid")
    }
}