/*
Filename: data/query.go

Description:
Simple DB interaction layer:

✅ insert
✅ read
✅ training data
*/

package data

import (
    "database/sql"
    "fmt"
)

// insert price
func InsertPrice(db *sql.DB, token string, price float64) error {

    _, err := db.Exec(
        "INSERT INTO market_prices(token, price) VALUES($1,$2)",
        token, price,
    )

    return err
}

// load prices
func LoadPrices(db *sql.DB, token string) ([]float64, error) {

    rows, err := db.Query(
        "SELECT price FROM market_prices WHERE token=$1 ORDER BY id",
        token,
    )
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var out []float64

    for rows.Next() {
        var p float64
        rows.Scan(&p)
        out = append(out, p)
    }

    return out, nil
}

// insert return
func InsertReturn(db *sql.DB, token string, value float64) {

    _, _ = db.Exec(
        "INSERT INTO returns(token, return) VALUES($1,$2)",
        token, value,
    )
}
