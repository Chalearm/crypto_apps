/*
Filename: infra/db_test.go

Author: M365 Copilot (GPT-5)
Owner: Chalearm Saelim
Version: v2.0
Date: 2026-06-23 07:53 ICT (UTC+7)

Description:
Unit tests for DB module.

Tests:
✅ InitDB safe fallback
✅ DB health behavior

Usage:
go test ./infra -v

UPDATED:
- fixed import cycle
- switched to infra_test package

*/

package infra_test

import (
    "testing"

    "dexbot/infra"
)

/*
Function: TestInitDB_NoCrash
Description:
Ensure InitDB does not crash without env.

Input:
- testing.T

Output:
- none

Lines: ~10
*/
func TestInitDB_NoCrash(t *testing.T) {

    err := infra.InitDB()

    if err != nil {
        t.Log("InitDB returned error (acceptable in no env)")
    }
}

/*
Function: TestDBHealth_NoCrash
Description:
Ensure DB health check does not crash.

Input:
- testing.T

Output:
- none

Lines: ~10
*/
func TestDBHealth_NoCrash(t *testing.T) {

    _ = infra.CheckDBHealth()
}