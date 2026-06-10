/*
Filename: swap/swap_test.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-10

Description:
Tests for swap module configuration.

AI Prompt Idea:
"Create tests to validate router address and token decimals in a DEX swap engine."

How to test:
go test ./swap -v
*/

package swap

import "testing"

func TestRouterNotEmpty(t *testing.T) {
    if ROUTER == "" {
        t.Error("router empty")
    }
}

func TestDecimalsExist(t *testing.T) {
    if Decimals["USDC"] == 0 {
        t.Error("missing decimals")
    }
}

func TestDecimalsPositive(t *testing.T) {
    for _, d := range Decimals {
        if d <= 0 {
            t.Error("invalid decimals")
        }
    }
}

func TestCommonTokens(t *testing.T) {
    if _, ok := Decimals["BTT"]; !ok {
        t.Error("missing BTT")
    }
}

func TestRouterLength(t *testing.T) {
    if len(ROUTER) < 10 {
        t.Error("router too short")
    }
}
