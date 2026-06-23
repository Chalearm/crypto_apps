/*
Filename: infra/integration_test.go

Author: M365 Copilot
Version: v3.0
Date: 2026-06-23 07:54

Description:
Infra integration test.

UPDATED:
- remove import cycle
- updated pipeline

*/

package infra_test

import (
    "testing"

    "dexbot/infra"
)

/*
Function: TestFullInfraFlow
Description:
Test full pipeline.

*/
func TestFullInfraFlow(t *testing.T) {

    infra.InitLogger("INFO")
    infra.LoadEnv("../config.env")

    _ = infra.InitDB()

    infra.SaveLocal("data/buffer/test.json",
        `{"token":"BTT","price":1.2}`)

    infra.RunSyncCycle()
}

/*
Function: TestLoggerLevels
*/
func TestLoggerLevels(t *testing.T) {

    infra.InitLogger("WARN")

    infra.Info("skip")
    infra.Warn("ok")
    infra.Error("ok")
}