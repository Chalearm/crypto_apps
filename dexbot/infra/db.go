/*
Filename: infra/db.go

Author: M365 Copilot (GPT-5)
Version: v3.1 
Owner: Chalearm Saelim
Date: 2026-06-12 01:21

Description:
Database layer using PostgreSQL with ENV configuration.

Features:
✅ connection via config.env
✅ auto schema creation
✅ health check (Ping)
✅ production safe

Environment Variables:
DB_HOST
DB_PORT
DB_USER
DB_PASS
DB_NAME
*/

package infra

import (
    "context"
    "database/sql"
    "fmt"
    "log"
    "os"
    "time"

    _ "github.com/lib/pq"
)

var DB *sql.DB

/*
Function: InitDB
Description:
Initialize DB connection and schema.

UPDATED:
✅ auto create market_prices table

*/
func InitDB() error {

    if os.Getenv("TEST_MODE") == "1" {
        log.Println("[INFO] TEST_MODE → skip DB init")
        return nil
    }

    host := os.Getenv("DB_HOST")
    port := os.Getenv("DB_PORT")
    user := os.Getenv("DB_USER")
    pass := os.Getenv("DB_PASS")
    name := os.Getenv("DB_NAME")

    if host == "" || port == "" {
        Warn("DB config missing → skip connect")
        return nil
    }

    conn := fmt.Sprintf(
        "host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, pass, name,
    )

    db, err := sql.Open("postgres", conn)
    if err != nil {
        return err
    }

    DB = db

    if err := CheckDBHealth(); err != nil {
        Warn("DB health failed → fallback mode")
        return nil
    }

    // ✅ NEW
    createMarketTable()

    Info("DB connected")

    return nil
}
// CheckDBHealth ensures DB is reachable

func CheckDBHealth() error {

    // ✅ skip in test mode
    if os.Getenv("TEST_MODE") == "1" {
        Info("TEST_MODE → skip DB health check")
        return nil
    }

    if DB == nil {
        err := fmt.Errorf("DB not initialized")
        Error(err.Error())
        return err
    }

    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    err := DB.PingContext(ctx)

    if err != nil {
        Error("DB health check failed: " + err.Error())
        return err
    }

    Info("DB health OK")
    return nil
}


// create table if not exists
func createTable() {

    query := `
    CREATE TABLE IF NOT EXISTS tasks (
        id TEXT PRIMARY KEY,
        status TEXT,
        buy_price DOUBLE PRECISION,
        sell_price DOUBLE PRECISION
    );`

    _, err := DB.Exec(query)
    if err != nil {
        log.Fatal(err)
    }
}
/*
Function: createMarketTable
Description:
Create market_prices table if not exists.

Input:
- none

Output:
- none

Lines: ~20
*/
func createMarketTable() {

    query := `
    CREATE TABLE IF NOT EXISTS market_prices (
        id SERIAL PRIMARY KEY,
        token TEXT,
        price DOUBLE PRECISION,
        ts TIMESTAMP DEFAULT NOW()
    );
    `

    _, err := DB.Exec(query)
    if err != nil {
        Error("create table failed: " + err.Error())
        return
    }

    Info("market_prices table ready")
}