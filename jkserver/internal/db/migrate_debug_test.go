package db

import (
    "fmt"
    "testing"
)

func TestMigrateDebug(t *testing.T) {
    d, err := Open("/tmp/jkrdebug", "test.db")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer d.Close()
    
    // Check schema
    rows, _ := d.DB.Query("SELECT sql FROM sqlite_master WHERE type='table'")
    defer rows.Close()
    for rows.Next() {
        var sql string
        rows.Scan(&sql)
        fmt.Println(sql)
    }
}
