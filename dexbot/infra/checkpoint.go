/******************************************************************************
 * File Name       : checkpoint.go
 * File Path       : infra/checkpoint.go
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
 *   Checkpoint manager for daemon state persistence. Every daemon can save its runtime state to a JSON file before shutdown and restore it on startup. Per myreq2.txt §27.
 *
 * Responsibilities:
 *   - Implement core functionality for infra package.
 *
 * Usage :
 *   Directory : infra/
 *
 *   Build :
 *     go build ./infra
 *
 *   Run :
 *     go run .  (from dexbot root)
 *
 *   Test :
 *     go test ./infra
 *
 * Dependencies :
 *   Internal :
 *     - dexbot/infra
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
package infra

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// CheckpointDir is where daemon checkpoint files are stored.
	CheckpointDir = "runtime"
)

/*
Function: checkpointPath
Description:
  Returns the full path for a daemon's checkpoint JSON file.

Input:
  - daemonName string : Daemon name (governance, school, trading, testdaemon)

Output:
  - string : Full path like "runtime/governance_checkpoint.json"

Lines: ~5
*/
func checkpointPath(daemonName string) string {
	return filepath.Join(CheckpointDir, daemonName+"_checkpoint.json")
}

/*
Function: SaveCheckpoint
Description:
  Saves an arbitrary state struct to a daemon's checkpoint JSON file.
  Creates the runtime directory if missing. Atomic via temp-file + rename.

Input:
  - daemonName string      : Daemon identifier
  - state      interface{} : Any JSON-serializable struct

Output:
  - error : Non-nil if write fails

Lines: ~20
*/
func SaveCheckpoint(daemonName string, state interface{}) error {
	path := checkpointPath(daemonName)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file then rename (atomic on most filesystems)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}

	FnTrace("checkpoint saved: " + path)
	return nil
}

/*
Function: RestoreCheckpoint
Description:
  Loads a daemon's state from its checkpoint JSON file into the target struct.
  Returns ErrCheckpointNotFound if the file doesn't exist.

Input:
  - daemonName string      : Daemon identifier
  - target     interface{} : Pointer to struct to populate

Output:
  - error : Non-nil if file missing, corrupt, or parse fails

Lines: ~20
*/
func RestoreCheckpoint(daemonName string, target interface{}) error {
	path := checkpointPath(daemonName)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &CheckpointError{Daemon: daemonName, Msg: "no checkpoint file found"}
		}
		return err
	}

	if len(data) == 0 {
		return &CheckpointError{Daemon: daemonName, Msg: "empty checkpoint file"}
	}

	if err := json.Unmarshal(data, target); err != nil {
		return &CheckpointError{Daemon: daemonName, Msg: "corrupt checkpoint: " + err.Error()}
	}

	FnTrace("checkpoint restored: " + path)
	return nil
}

/*
Function: RemoveCheckpoint
Description:
  Deletes a daemon's checkpoint file. Use after successful shutdown.

Input:
  - daemonName string : Daemon identifier

Output:
  - error : Non-nil if delete fails (excluding not-exist)

Lines: ~8
*/
func RemoveCheckpoint(daemonName string) error {
	path := checkpointPath(daemonName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

/*
Function: CheckpointExists
Description:
  Returns true if a checkpoint file exists for the given daemon.

Input:
  - daemonName string : Daemon identifier

Output:
  - bool : True if checkpoint file exists

Lines: ~8
*/
func CheckpointExists(daemonName string) bool {
	path := checkpointPath(daemonName)
	_, err := os.Stat(path)
	return err == nil
}

// ==============================
// CHECKPOINT ERROR TYPE
// ==============================

/*
Struct: CheckpointError
Description:
  Structured error for checkpoint operations.

Fields:
  - Daemon string : Daemon name
  - Msg    string : Error description

Lines: ~5
*/
type CheckpointError struct {
	Daemon string
	Msg    string
}

func (e *CheckpointError) Error() string {
	return "checkpoint[" + e.Daemon + "]: " + e.Msg
}
