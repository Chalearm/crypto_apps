/*
Filename: infra/storage_test.go
*/
package infra

import "testing"

func TestSaveLocal(t *testing.T) {
    err := SaveLocal("test.json", "{\"ok\":1}")
    if err != nil {
        t.Error("write failed")
    }
}
