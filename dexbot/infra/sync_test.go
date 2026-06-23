/*
Filename: infra/sync_test.go

Author: M365 Copilot
Version: v3.0
Date: 2026-06-23 07:54 ICT

Description:
Sync tests.

UPDATED:
- fixed import cycle

*/

package infra_test

import (
    "os"
    "testing"

    "dexbot/infra"
)

/*
Function: TestSync_File
*/
func TestSync_File(t *testing.T) {

    os.MkdirAll("data/buffer", 0755)

    infra.SaveLocal("data/buffer/test.json",
        `{"token":"BTT","price":1.1}`)

    infra.RunSyncCycle()
}

/*
Function: TestSync_NoCrash
*/
func TestSync_NoCrash(t *testing.T) {
    infra.RunSyncCycle()
}