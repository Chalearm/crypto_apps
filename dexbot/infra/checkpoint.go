/******************************************************************************
 * File Name       : checkpoint.go
 * File Path       : infra/checkpoint.go
 * Author          : deepseek-4.0-pro
 * Owner           : Chalearm Saelim
 * Reviewer        : Chalearm Saelim
 * Version         : 1.0.0
 * Status          : Development
 * Created Date    : 2026-07-01 19:25:27 (UTC+7)
 * Modified Date   : 2026-07-01 19:25:27 (UTC+7)
 * Description     : Checkpoint manager for daemon state persistence.
 ******************************************************************************/
package infra

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	CheckpointDir = "runtime"
)

/******************************************************************************
 * Function Name : checkpointPath
 * Purpose       : Constructs the filesystem path for a daemon checkpoint file.
 ******************************************************************************/
func checkpointPath(daemonName string) string {
	return filepath.Join(CheckpointDir, daemonName+"_checkpoint.json")
}

/******************************************************************************
 * Function Name : SaveCheckpoint
 * Purpose       : Saves daemon runtime state to a JSON file atomically.
 ******************************************************************************/
func SaveCheckpoint(daemonName string, state interface{}) error {
	path := checkpointPath(daemonName)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

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

/******************************************************************************
 * Function Name : RestoreCheckpoint
 * Purpose       : Restores daemon state from a checkpoint file.
 ******************************************************************************/
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

/******************************************************************************
 * Function Name : RemoveCheckpoint
 * Purpose       : Deletes a daemon checkpoint file if it exists.
 ******************************************************************************/
func RemoveCheckpoint(daemonName string) error {
	path := checkpointPath(daemonName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

/******************************************************************************
 * Function Name : CheckpointExists
 * Purpose       : Checks whether a checkpoint file exists for a daemon.
 ******************************************************************************/
func CheckpointExists(daemonName string) bool {
	path := checkpointPath(daemonName)
	_, err := os.Stat(path)
	return err == nil
}

// ==============================
// CHECKPOINT ERROR TYPE
// ==============================

type CheckpointError struct {
	Daemon string
	Msg    string
}

func (e *CheckpointError) Error() string {
	return "checkpoint[" + e.Daemon + "]: " + e.Msg
}