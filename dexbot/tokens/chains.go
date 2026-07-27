/******************************************************************************
 * File Name       : chains.go
 * File Path       : tokens/chains.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Version         : 1.0.0
 * Created Date    : 2026-07-06 08:00:00 (UTC+7)
 *
 * Description     :
 *   Chain registry — defines the 4 supported blockchain networks
 *
 * Usage :
 *   Directory : tokens/
 *   Package   : dexbot/tokens with
 *   metadata. Used by serve.py to populate user_chains DB table on
 *   first unlock for a new profile.
 *
 * Change History :
 *   v1.0.0 | 2026-07-06 08:00 | deepseek-4.0-pro | Initial version
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
 * TODO :
 *   - Add documentation.
 *
 * Notes :
 *   - Per regulator coding standard.
 *
 * Reviewer        : Chalearm Saelim
 * Status          : Development
 * Modified Date   : 2026-07-26 08:00:00 (UTC+7)
 */

package tokens

// ChainMeta holds metadata for a supported blockchain network.
type ChainMeta struct {
	Name    string // human-readable name
	ChainID string // numeric chain ID string
	BaseURL string // default RPC endpoint
}

// AllChains returns the 4 default supported chains.
// Used by serve.py to populate user_chains on first profile creation.
/******************************************************************************
 * Function Name : AllChains
 *
 * Purpose :
 *   Performs its designated operation.
 *
 * Inputs :
 *   None (see function signature)
 *
 * Return :
 *   Type        : varies
 *   Description : Result of computation.
 *
 * Complexity :
 *   Time  : O(1)
 *   Space : O(1)
 *
 * Error Cases :
 *   - None
 *
 * Number Of Lines :
 *   10
 ******************************************************************************/

func AllChains() []ChainMeta {
	return []ChainMeta{
		{Name: "BSC", ChainID: "56", BaseURL: "https://bsc-dataseed.binance.org"},
		{Name: "POLYGON", ChainID: "137", BaseURL: "https://polygon-rpc.com"},
		{Name: "ETHEREUM", ChainID: "1", BaseURL: "https://eth.llamarpc.com"},
		{Name: "OPBNB", ChainID: "204", BaseURL: "https://opbnb-mainnet-rpc.bnbchain.org"},
	}
}
