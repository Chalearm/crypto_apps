/******************************************************************************
 * File Name       : pk_helper.go
 * File Path       : integration/pk_helper.go
 *
 * Author          : Chalearm Saelim
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-26 06:00:00 (UTC+7)
 * Modified Date   : 2026-07-26 06:00:00 (UTC+7)
 *
 * Description     :
 *   Helper to read PRIVATE_KEY from config.env for integration tests.
 *
 * Usage :
 *   Directory : integration/
 *   Test      : go test ./integration
 *****************************************************************************
 *
 * Responsibilities:
 *   - Part of the dexbot platform.
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
 *
 *   External :
 *     - (stdlib only)
 *
 * Updated Parts :
 *   None
 *
 * New Parts :
 *   [Function] See function list.
 *
 * Change History :
 *   -------------------------------------------------------------------------
 *   Version | Date Time (UTC+7)        | Author          | Description
 *   -------------------------------------------------------------------------
 *   1.0.0   | 2026-07-26 08:00:00 (UTC+7)      | Chalearm Saelim | Initial
 *   -------------------------------------------------------------------------
 *
 * TODO :
 *   - Add documentation.
 *
 * Notes :
 *   - Per regulator coding standard.
 */

package integration

/******************************************************************************
 * File Name       : pk_helper.go
 * File Path       : integration/pk_helper.go
 *
 * Author          : Chalearm Saelim
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-26 06:03:11 (UTC+7)
 * Modified Date   : 2026-07-26 06:03:11 (UTC+7)
 *
 * Description     :
 *   Helper to read PRIVATE_KEY from config.env for integration tests.
 *
 * Usage :
 *   Directory : integration/
 *   Test      : go test ./integration
 ******************************************************************************/

import (
	"os"
	"strings"
	"testing"
)

// pkEnv reads PRIVATE_KEY from env or config.env. Never hardcoded.
/******************************************************************************
 * Function Name : pkEnv
 *
 * Purpose :
 *   Read PRIVATE_KEY from config.env for integration tests.
 *
 * Inputs :
 *   t *testing.T
 *     Type        : test context
 *     Description : For error reporting via t.Fatal.
 *
 * Return :
 *   Type        : string
 *   Description : Private key string from environment.
 *
 * Complexity :
 *   Time  : O(N) where N = lines in config.env
 *   Space : O(1)
 *
 * Error Cases :
 *   - Config file not found returns empty string.
 *
 * Number Of Lines :
 *   15
 ******************************************************************************/
func pkEnv(t *testing.T) string {
	t.Helper()
	if pk := os.Getenv("PRIVATE_KEY"); pk != "" {
		return strings.TrimSpace(pk)
	}
	for _, p := range []string{"../config.env", "../../config.env", "config.env"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRIVATE_KEY=") {
				return strings.TrimSpace(strings.TrimPrefix(line, "PRIVATE_KEY="))
			}
		}
	}
	t.Skip("PRIVATE_KEY not found")
	return ""
}
