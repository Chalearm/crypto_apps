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
 *   Chain registry — defines the 4 supported blockchain networks with
 *   metadata. Used by serve.py to populate user_chains DB table on
 *   first unlock for a new profile.
 *
 * Change History :
 *   v1.0.0 | 2026-07-06 08:00 | deepseek-4.0-pro | Initial version
 ******************************************************************************/

package tokens

// ChainMeta holds metadata for a supported blockchain network.
type ChainMeta struct {
	Name    string // human-readable name
	ChainID string // numeric chain ID string
	BaseURL string // default RPC endpoint
}

// AllChains returns the 4 default supported chains.
// Used by serve.py to populate user_chains on first profile creation.
func AllChains() []ChainMeta {
	return []ChainMeta{
		{Name: "BSC", ChainID: "56", BaseURL: "https://bsc-dataseed.binance.org"},
		{Name: "POLYGON", ChainID: "137", BaseURL: "https://polygon-rpc.com"},
		{Name: "ETHEREUM", ChainID: "1", BaseURL: "https://eth.llamarpc.com"},
		{Name: "OPBNB", ChainID: "204", BaseURL: "https://opbnb-mainnet-rpc.bnbchain.org"},
	}
}
