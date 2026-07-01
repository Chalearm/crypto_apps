/******************************************************************************
 * File Name       : sample.go
 * File Path       : apps/risk_lab/sample.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-06-30 00:53:07 (UTC+7)
 * Modified Date   : 2026-06-30 00:53:07 (UTC+7)
 *
 * Description     :
 *   This file defines the synthetic market dataset used for the risk lab system. The dataset is intentionally designed to simulate: ✅ multi-asset portfolio (BTC, ETH, BNB, UNI, ADA) ✅ regime shift behavio
 *
 * Responsibilities:
 *   - Implement core functionality for apps package.
 *
 * Usage :
 *   Directory : apps/risk_lab/
 *
 *   Build :
 *     go build ./apps/risk_lab
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./apps/risk_lab
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/apps
 *
 *   External :
 *     - (stdlib only)
 *
 * Configuration :
 *   - config.env
 *
 * Updated Parts :
 *   None (initial version)
 *
 * New Parts :
 *   [Functions] All exported functions in this file
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)      | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-06-30 00:53:07 (UTC+7)   | deepseek-4.0-pro | Initial version — rule1.txt header batch
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add unit tests
 *
 * Notes :
 *   - Per rule1.txt coding standard.
 ******************************************************************************/
package main

func sampleData() map[string][]float64 {

    return map[string][]float64{

        "BTC": {
            100,102,104,106,108,110,112,114,116,118,
            120,122,124,126,128,130,132,134,136,138,
            // event
            140,135,138,142,145,148,150,147,152,155,
            158,160,162,165,170,
        },

        "ETH": {
            70,72,74,75,77,79,80,82,84,85,
            87,89,90,92,94,96,98,100,102,104,
            // event
            106,101,105,108,110,112,115,113,117,120,
            123,125,128,130,135,
        },

        "BNB": {
            50,52,51,55,53,57,60,58,61,63,
            65,62,66,68,70,69,72,74,76,78,
            // event
            80,75,82,78,85,83,88,90,87,92,
            95,93,97,100,105,
        },

        "UNI": {
            20,21,23,22,24,26,25,27,29,28,
            30,32,31,33,35,34,36,38,37,39,
            // event (volatile)
            42,35,45,38,48,40,50,42,55,45,
            60,48,65,50,70,
        },

        "ADA": {
            10,11,12,11,10,12,11,13,12,14,
            13,15,14,16,15,17,16,18,17,19,
            // event (crash cycles)
            18,12,20,11,22,10,25,9,28,8,
            30,7,32,6,35,
        },
    }
}
