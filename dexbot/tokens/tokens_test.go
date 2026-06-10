/*
Filename: tokens/tokens_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-10

Description:
Tests for token registry.

AI Prompt Idea:
"Create Go unit tests for validating a token map with blockchain addresses."

How to test:
go test ./tokens -v
*/

package tokens

import "testing"

func TestUSDCExists(t *testing.T) {
    if _, ok := Tokens["USDC"]; !ok {
        t.Error("USDC missing")
    }
}

func TestInvalidToken(t *testing.T) {
    if _, ok := Tokens["FAKE"]; ok {
        t.Error("fake token exists")
    }
}

func TestTokenCount(t *testing.T) {
    if len(Tokens) < 5 {
        t.Error("not enough tokens")
    }
}

func TestAddressNotZero(t *testing.T) {
    if Tokens["USDC"].Hex() == "0x0000000000000000000000000000000000000000" {
        t.Error("invalid address")
    }
}

func TestSymbolsNonEmpty(t *testing.T) {
    for k := range Tokens {
        if k == "" {
            t.Error("empty symbol")
        }
    }
}

