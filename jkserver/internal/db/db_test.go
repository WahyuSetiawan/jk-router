//go:build test

package db

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	d, err := Open(dir, filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestWriterCorrectness runs N writes via EnqueueWriteSync and verifies they
// all land. This covers the single-writer discipline and migration setup.
func TestWriterCorrectness(t *testing.T) {
	d := newTestDB(t)
	defer d.Close()

	const N = 100
	for i := 0; i < N; i++ {
		var version int
		d.EnqueueWriteSync(func(q *Queue) {
			_, err := q.db.ExecContext(context.Background(),
				"INSERT INTO settings_kv (key, value) VALUES (?, ?)",
				"k"+strconv.Itoa(i), "v"+strconv.Itoa(i))
			if err != nil {
				t.Errorf("enqueue %d failed: %v", i, err)
			}
			// Verify row landed immediately.
			if err := q.db.QueryRowContext(context.Background(),
				"SELECT COUNT(*) FROM settings_kv WHERE key=?", "k"+strconv.Itoa(i)).Scan(&version); err != nil {
				t.Errorf("verify %d failed: %v", i, err)
			}
		})
	}
	var count int
	if err := d.DB.QueryRow(`SELECT COUNT(*) FROM settings_kv`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != N {
		t.Fatalf("rows=%d, want %d", count, N)
	}
}

// TestWriterConcurrencyNonBlocking verifies that EnqueueWrite never blocks
// the caller even under load — the channel drops (counter rises) instead.
// This is the PRD §4.1 contract: drop, don't stall request path.
func TestWriterConcurrencyNonBlocking(t *testing.T) {
	d := newTestDB(t)
	defer d.Close()

	const N = 500
	done := make(chan struct{}, N)
	for i := 0; i < N; i++ {
		go func() {
			d.EnqueueWrite(func(q *Queue) {}) // no-op job
			done <- struct{}{}
		}()
	}
	for i := 0; i < N; i++ {
		select {
		case <-done:
			// OK — returned instantly (non-blocking contract)
		case <-time.After(time.Second):
			t.Fatalf("EnqueueWrite blocked after 1s on call %d", i)
		}
	}
	t.Logf("dropped=%d / submitted=%d", d.DroppedWrites(), N)
}

func TestMigrationRuns(t *testing.T) {
	d := newTestDB(t)
	defer d.Close()

	tables := []string{"providers", "connections", "model_catalog", "combos",
		"api_keys", "proxy_pools", "settings_kv", "usage_log", "schema_migrations"}
	for _, tbl := range tables {
		var name string
		if err := d.DB.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", tbl, err)
		}
	}
}
