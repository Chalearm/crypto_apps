/*
Filename: infra/sync_test.go

Description:
Test sync functionality

*/

package infra

import (
    "os"
    "testing"
)

/*
Test: create local file and sync

*/
func TestSyncLocalFile(t *testing.T) {

    file := "data/buffer/test.json"

    content := `{"token":"BTT","price":1.2}`

    _ = SaveLocal(file, content)

    RunSyncCycle()

    // file should either exist or be removed (no crash)
}

/*
Test: invalid json

*/
func TestSyncInvalidJSON(t *testing.T) {

    file := "data/buffer/bad.json"

    _ = os.WriteFile(file, []byte("invalid"), 0644)

    RunSyncCycle()
}