/******************************************************************************
 * File Name       : commander.go
 * File Path       : governance/commander.go
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
 *   Governance command dispatcher. Provides CLI + API command interface per myreq2.txt ?4. Supported actions: status, reload-config, restart, stop, start, shutdown. Daemon lifecycle commands route through
 *
 * Responsibilities:
 *   - Implement core functionality for governance package.
 *
 * Usage :
 *   Directory : governance/
 *
 *   Build :
 *     go build ./governance
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./governance
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/governance
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
 *   [Types] Struct definitions in this file
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
package governance

import (
	"errors"
	"fmt"
)

// ==============================
// ACTION CONSTANTS
// ==============================

// Commander actions per myreq2.txt ?4.
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
)

// ==============================
// COMMANDER INTERFACE
// ==============================

/*
Interface: Commander
Description:
  Defines the command dispatch contract. Each daemon implements
  this to expose CLI actions.

Methods:
  - Dispatch(action string, args map[string]string) (string, error)

Lines: ~4
*/
type Commander interface {
	Dispatch(action string, args map[string]string) (string, error)
}

// ==============================
// DEFAULT COMMANDER
// ==============================

/*
Struct: DefaultCommander
Description:
  Default implementation of Commander. Dispatches to handler functions
  registered for each action.

Fields:
  - handlers map[string]func(args map[string]string) (string, error)

Lines: ~5
*/
type DefaultCommander struct {
	handlers map[string]func(args map[string]string) (string, error)
}

/*
Function: NewCommander
Description:
  Creates a new DefaultCommander with no registered handlers.

Input:
  - none

Output:
  - *DefaultCommander: Initialized commander

Lines: ~8
*/
func NewCommander() *DefaultCommander {
	return &DefaultCommander{
		handlers: make(map[string]func(map[string]string) (string, error)),
	}
}

/*
Function: Register
Description:
  Registers a handler function for a given action name.

Input:
  - action  string                                          : Action name (use Action* constants)
  - handler func(args map[string]string) (string, error)    : Handler function

Output:
  - none

Lines: ~3
*/
func (c *DefaultCommander) Register(action string, handler func(map[string]string) (string, error)) {
	c.handlers[action] = handler
}

/*
Function: Dispatch
Description:
  Routes an action to its registered handler. Returns an error
  if no handler is registered for the action.

Input:
  - action string            : Action name (e.g., "status", "restart")
  - args   map[string]string : Key-value arguments (e.g., {"daemon":"school"})

Output:
  - string : Result message from the handler
  - error  : Non-nil if action is unknown or handler returns error

Lines: ~12
*/
func (c *DefaultCommander) Dispatch(action string, args map[string]string) (string, error) {
	handler, ok := c.handlers[action]
	if !ok {
		return "", fmt.Errorf("unknown action: %s", action)
	}
	return handler(args)
}

/*
Function: ValidateAction
Description:
  Checks whether an action name is a known, supported action.

Input:
  - action string : Action name to validate

Output:
  - error : non-nil if action is not recognized

Lines: ~15
*/
func ValidateAction(action string) error {
	valid := map[string]bool{
		ActionStatus:        true,
		ActionReloadConfig:  true,
		ActionReloadLog:     true,
		ActionRestart:       true,
		ActionStop:          true,
		ActionStart:         true,
		ActionSuspend:       true,
		ActionResume:        true,
		ActionPromote:       true,
		ActionDemote:        true,
		ActionShutdown:      true,
		ActionFetchMarket:   true,
		ActionFetchDB:       true,
		ActionHelp:          true,
		ActionHelpConfig:    true,
		ActionHelpConfigVVV: true,
	}
	if valid[action] {
		return nil
	}
	return errors.New("invalid action: " + action)
}

/*
Function: AllActions
Description:
  Returns a sorted list of all supported actions.

Input:
  - none

Output:
  - []string : All action constant values

Lines: ~25
*/
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
