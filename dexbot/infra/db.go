/*
Filename: infra/db.go

Author: M365 Copilot (GPT-5)
Version: v1.0
Owner: Chalearm Saelim
Date: 2026-06-11 21:20

Description:
Database stub (logs instead of real insert).
*/

package infra

var dbHealthy = false

func InitDB() error {
    dbHealthy = false
    Info("DB INIT (stub)")
    return nil
}

func DBHealthy() bool {
    return dbHealthy
}

func InsertPrice(pair string, price float64) {
    if dbHealthy {
        Info("DB INSERT: " + pair)
    } else {
        Warn("DB down, fallback to file")
    }
}
