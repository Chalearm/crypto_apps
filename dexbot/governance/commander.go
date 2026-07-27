/******************************************************************************
 * File Name       : commander.go
 * File Path       : governance/commander.go
 *
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 *
 * Function Name   : Dispatch
 * Purpose         : Performs its designated operation.
 * Inputs          : None (see function signature)
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:45 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:45 (UTC+7)
 *
 * Description     :
 *    Governance command dispatcher. Provides CLI + API command interface per myreq2.txt ?4.
 *
 * Responsibilities:
 *    - Implement core functionality for governance package.
 *
 * Usage :
 *    Directory : governance/
 *
 *    Build :
 *      go build ./governance
 *
 *    Run :
 *      go run .  (from dexbot root)
 *
 *    Test :
 *      go test ./governance
 *
 * Dependencies :
 *    Internal :
 *      - dexbot/governance
 *
 *    External :
 *      - (stdlib only)
 *
 * Configuration :
 *    - config.env
 *
 * Updated Parts :
 *    None (initial version)
 *
 * New Parts :
 *    [Functions] All exported functions in this file
 *
 * Change History :
 *    -------------------------------------------------------------------------
 *    Version | Date Time (UTC+7)       | Author           | Description
 *    -------------------------------------------------------------------------
 *    1.0.0   | 2026-07-01 19:25:45 (UTC+7)   | deepseek-4.0-pro | Header validation — rule1.txt compliant
 *    -------------------------------------------------------------------------
 *
 * TODO :
 *    - Add unit tests
 *
 * Notes :
 *    - Per rule1.txt coding standard.
 ******************************************************************************/
package governance

import (
	"errors"
	"fmt"
)

// ==============================
// ACTION CONSTANTS
// ==============================

const (
	ActionStatus          = "status"
	ActionReloadConfig    = "reload-config"
	ActionReloadLog       = "reload-log"
	ActionRestart         = "restart"
	ActionStop            = "stop"
	ActionStart           = "start"
	ActionSuspend         = "suspend"
	ActionResume          = "resume"
	ActionPromote         = "promote"
	ActionDemote          = "demote"
	ActionShutdown        = "shutdown"
	ActionFetchMarket     = "fetchMarket"
	ActionFetchDB         = "fetchDB"
	ActionHelp            = "help"
	ActionHelpConfig      = "help-configuration"
	ActionHelpConfigVVV   = "help-configuration-vvv"
	ActionBalance         = "balance"
	ActionAddToken        = "addToken"
	ActionAddChain        = "addChain"
)

// ==============================
// COMMANDER INTERFACE
// ==============================

type Commander interface {
	Dispatch(action string, args map[string]string) (string, error)
}

// ==============================
// DEFAULT COMMANDER
// ==============================

type DefaultCommander struct {
	handlers map[string]func(args map[string]string) (string, error)
}

/******************************************************************************
 * Function Name : NewCommander
 * Purpose       : Performs its designated operation.
 * Inputs        : None (see function signature)
 * Return        : Type: *DefaultCommander
 * Complexity    : Time: O(1), Space: O(1)
 * Error Cases   : None
 * Number Of Lines: 10
 ******************************************************************************/
func NewCommander() *DefaultCommander {
	return &DefaultCommander{
		handlers: make(map[string]func(map[string]string) (string, error)),
	}
}

/******************************************************************************
 * Function Name : Register
 * Purpose       : Performs its designated operation.
 * Inputs        : None (see function signature)
 * Return        : None
 * Complexity    : Time: O(1), Space: O(1)
 * Error Cases   : None
 * Number Of Lines: 10
 ******************************************************************************/
func (c *DefaultCommander) Register(action string, handler func(map[string]string) (string, error)) {
	c.handlers[action] = handler
}

/******************************************************************************
 * Function Name : Dispatch
 * Purpose       : Performs its designated operation.
 * Inputs        : None (see function signature)
 * Return        : Type: (string, error)
 * Complexity    : Time: O(1), Space: O(1)
 * Error Cases   : Unknown action error
 * Number Of Lines: 10
 ******************************************************************************/
func (c *DefaultCommander) Dispatch(action string, args map[string]string) (string, error) {
	handler, ok := c.handlers[action]
	if !ok {
		return "", fmt.Errorf("unknown action: %s", action)
	}
	return handler(args)
}

/******************************************************************************
 * Function Name : ValidateAction
 * Purpose       : Performs its designated operation.
 * Inputs        : None (see function signature)
 * Return        : Type: error
 * Complexity    : Time: O(1), Space: O(1)
 * Error Cases   : Invalid action error
 * Number Of Lines: 10
 ******************************************************************************/
func ValidateAction(action string) error {
	valid := map[string]bool{
		ActionStatus:          true,
		ActionReloadConfig:    true,
		ActionReloadLog:       true,
		ActionRestart:         true,
		ActionStop:            true,
		ActionStart:           true,
		ActionSuspend:         true,
		ActionResume:          true,
		ActionPromote:         true,
		ActionDemote:          true,
		ActionShutdown:        true,
		ActionFetchMarket:     true,
		ActionFetchDB:         true,
		ActionHelp:            true,
		ActionHelpConfig:      true,
		ActionHelpConfigVVV:   true,
	}
	if valid[action] {
		return nil
	}
	return errors.New("invalid action: " + action)
}

/******************************************************************************
 * Function Name : AllActions
 * Purpose       : Performs its designated operation.
 * Inputs        : None (see function signature)
 * Return        : Type: []string
 * Complexity    : Time: O(1), Space: O(1)
 * Error Cases   : None
 * Number Of Lines: 10
 ******************************************************************************/
func AllActions() []string {
	return []string{
		ActionStatus,
		ActionReloadConfig,
		ActionReloadLog,
		ActionRestart,
		ActionStop,
		ActionStart,
		ActionSuspend,
		ActionResume,
		ActionPromote,
		ActionDemote,
		ActionShutdown,
		ActionFetchMarket,
		ActionFetchDB,
		ActionHelp,
		ActionHelpConfig,
		ActionHelpConfigVVV,
	}
}