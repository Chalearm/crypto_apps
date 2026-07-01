/******************************************************************************
 * File Name       : checkpoint_test.go
 * File Path       : infra/checkpoint_test.go
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
 *   Unit tests for checkpoint manager. 5 positive + 2 negative test cases per coding rule §2. go test ./infra -v -run Checkpoint Directory: dexbot/infra/ - Created during Phase 14. - All test functions be
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
 *   [Test Functions] Test suite: TestSaveAndRestore_Success, TestSaveAndRestore_Overwrite, TestCheckpointExists_Negative, TestRemoveCheckpoint_Success
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
	"os"
	"testing"
)

// testState is a sample struct for checkpoint round-trip tests.
type testState struct {
	Name    string  `json:"name"`
	Version string  `json:"version"`
	Counter int     `json:"counter"`
	Active  bool    `json:"active"`
	Tags    []string `json:"tags"`
}

// ==============================
// POSITIVE TESTS
// ==============================

func TestSaveAndRestore_Success(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	orig := testState{Name: "governance", Version: "v2.0", Counter: 42, Active: true, Tags: []string{"a", "b"}}

	if err := SaveCheckpoint("governance", &orig); err != nil {
		t.Fatalf("SaveCheckpoint failed: %v", err)
	}
	if !CheckpointExists("governance") {
		t.Fatal("Expected checkpoint to exist")
	}

	var restored testState
	if err := RestoreCheckpoint("governance", &restored); err != nil {
		t.Fatalf("RestoreCheckpoint failed: %v", err)
	}
	if restored.Name != "governance" || restored.Counter != 42 {
		t.Errorf("Restored data mismatch: got %+v", restored)
	}
	if len(restored.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(restored.Tags))
	}
}

func TestSaveAndRestore_Overwrite(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	SaveCheckpoint("test", &testState{Name: "first", Counter: 1})
	SaveCheckpoint("test", &testState{Name: "second", Counter: 2})

	var restored testState
	RestoreCheckpoint("test", &restored)
	if restored.Counter != 2 {
		t.Errorf("Expected overwritten value 2, got %d", restored.Counter)
	}
}

func TestCheckpointExists_Negative(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	if CheckpointExists("nonexistent") {
		t.Error("Expected CheckpointExists to return false")
	}
}

func TestRemoveCheckpoint_Success(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	SaveCheckpoint("tmp", &testState{Name: "x"})
	if err := RemoveCheckpoint("tmp"); err != nil {
		t.Fatalf("RemoveCheckpoint failed: %v", err)
	}
	if CheckpointExists("tmp") {
		t.Error("Expected checkpoint removed")
	}
}

func TestLargePayload(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	tags := make([]string, 1000)
	for i := range tags {
		tags[i] = "tag_" + string(rune('A'+i%26))
	}
	orig := testState{Name: "large", Counter: 9999, Tags: tags}

	if err := SaveCheckpoint("large", &orig); err != nil {
		t.Fatalf("Save large checkpoint failed: %v", err)
	}

	var restored testState
	if err := RestoreCheckpoint("large", &restored); err != nil {
		t.Fatalf("Restore large checkpoint failed: %v", err)
	}
	if len(restored.Tags) != 1000 {
		t.Errorf("Large payload tag count mismatch: got %d", len(restored.Tags))
	}
}

// ==============================
// NEGATIVE TESTS
// ==============================

func TestRestoreMissingFile(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	var s testState
	err := RestoreCheckpoint("ghost", &s)
	if err == nil {
		t.Error("Expected error for missing checkpoint")
	}
}

func TestRestoreCorruptJSON(t *testing.T) {
	defer os.RemoveAll(CheckpointDir)

	path := checkpointPath("corrupt")
	os.MkdirAll(CheckpointDir, 0755)
	os.WriteFile(path, []byte("{ not json }"), 0644)

	var s testState
	err := RestoreCheckpoint("corrupt", &s)
	if err == nil {
		t.Error("Expected error for corrupt JSON")
	}
}
