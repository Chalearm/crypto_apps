/*
Filename: infra/db_test.go
*/
package infra

import "testing"

func TestDBInit(t *testing.T) {
    err := InitDB()
    if err != nil {
        t.Error("db init failed")
    }
}

func TestInsertFallback(t *testing.T) {
    InsertPrice("BTC/USDT", 100)
}
