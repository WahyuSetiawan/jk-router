package db

import (
    "sync"
    "testing"
    "time"
)

func TestDeadlock(t *testing.T) {
    dir := t.TempDir()
    dbPath := dir + "/test.db"
    
    d, err := Open(dir, dbPath)
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    
    // Test QueryRow
    t.Log("Testing QueryRow...")
    var count int
    err = d.QueryRow("SELECT COUNT(*) FROM settings_kv").Scan(&count)
    if err != nil {
        t.Fatalf("QueryRow failed: %v", err)
    }
    t.Logf("QueryRow OK: %d", count)
    
    // Test Query
    t.Log("Testing Query...")
    rows, err := d.Query("SELECT key, value FROM settings_kv")
    if err != nil {
        t.Fatalf("Query failed: %v", err)
    }
    rows.Close()
    t.Log("Query OK")
    
    // Test concurrent queries with WaitGroup
    t.Log("Testing concurrent queries...")
    var wg sync.WaitGroup
    for i := 0; i < 3; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            var c int
            err := d.QueryRow("SELECT COUNT(*) FROM settings_kv").Scan(&c)
            if err != nil {
                t.Errorf("goroutine %d QueryRow failed: %v", n, err)
            }
        }(i)
    }
    
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        t.Log("Concurrent queries OK")
    case <-time.After(3 * time.Second):
        t.Fatal("concurrent queries TIMEOUT")
    }
    
    // Test EnqueueWriteSync followed by QueryRow
    t.Log("Testing EnqueueWriteSync then QueryRow...")
    d.EnqueueWriteSync(func(q *Queue) {
        q.DB().Exec("INSERT OR REPLACE INTO settings_kv (key, value) VALUES ('test_key', 'test_value')")
    })
    t.Log("EnqueueWriteSync OK")
    
    err = d.QueryRow("SELECT COUNT(*) FROM settings_kv").Scan(&count)
    if err != nil {
        t.Fatalf("QueryRow after write failed: %v", err)
    }
    t.Logf("QueryRow after write OK: %d", count)
    
    // Test multiple sequential QueryRows
    t.Log("Testing multiple sequential QueryRows...")
    for i := 0; i < 5; i++ {
        var c int
        err = d.QueryRow("SELECT COUNT(*) FROM settings_kv").Scan(&c)
        if err != nil {
            t.Fatalf("sequential QueryRow %d failed: %v", i, err)
        }
    }
    t.Log("Sequential QueryRows OK")
    
    d.Close()
}

func TestUsageStatsQuery(t *testing.T) {
    dir := t.TempDir()
    dbPath := dir + "/test.db"
    
    d, err := Open(dir, dbPath)
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    defer d.Close()
    
    // Run the exact query from UsageStatsHandler
    row := d.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN status='success' THEN 1 ELSE 0 END),0), COALESCE(SUM(CASE WHEN status='error' OR status='fallback' THEN 1 ELSE 0 END),0), COALESCE(SUM(tok_in),0) FROM usage_log WHERE ts >= datetime('now', '-7 days')")
    var total, success, errors, tokensIn int
    if err := row.Scan(&total, &success, &errors, &tokensIn); err != nil {
        t.Fatalf("usage stats query failed: %v", err)
    }
    t.Logf("usage stats query OK: %d %d %d %d", total, success, errors, tokensIn)
}
