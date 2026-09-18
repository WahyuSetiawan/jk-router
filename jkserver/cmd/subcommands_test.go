package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/settings"
)

// setupTestDB creates a temp data dir with an initialized DB and returns the dir path.
func setupTestDB(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := settings.DBPath(tmpDir)
	d, err := db.Open(tmpDir, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()
	// Run migrations by opening with auto-migrate (db.Open handles this).
	return tmpDir
}

// insertTestProviders adds a sample provider and connection to the DB.
func insertTestProviders(t *testing.T, dataDir string) {
	t.Helper()
	dbPath := settings.DBPath(dataDir)
	d, err := db.Open(dataDir, dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer d.Close()

	d.Exec(`INSERT OR REPLACE INTO providers (id, name, created_at) VALUES ('test-prov', 'Test Provider', '2026-01-01')`)
	d.Exec(`INSERT OR REPLACE INTO connections (provider_id, name, auth_type, priority, created_at) VALUES ('test-prov', 'test-conn', 'api_key', 0, '2026-01-01')`)
	d.Exec(`INSERT OR REPLACE INTO combos (name, model_list, strategy, created_at) VALUES ('test-combo', 'gpt-4o,claude-sonnet', 'fallback', '2026-01-01')`)
	d.Exec(`INSERT OR REPLACE INTO proxy_pools (name, ptype, proxy_url, created_at) VALUES ('test-pool', 'http', 'http://proxy:8080', '2026-01-01')`)
	d.Exec(`INSERT OR REPLACE INTO api_keys (key_hash, label, revoked, created_at) VALUES ('hash-test', 'test-key', 0, '2026-01-01')`)
}

func TestExportConfigProducesValidJSON(t *testing.T) {
	dataDir := setupTestDB(t)
	insertTestProviders(t, dataDir)

	// Capture stdout by redirecting os.Stdout temporarily.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	ExportConfig(dataDir)

	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()
	var exp ExportData
	if err := json.Unmarshal([]byte(output), &exp); err != nil {
		t.Fatalf("parse export JSON: %v\noutput: %s", err, output)
	}
	t.Logf("Export output (sample): %d providers, %d connections", len(exp.Providers), len(exp.Connections))
	if exp.Version == "" {
		t.Fatal("export JSON missing Version field")
	}
	if len(exp.Providers) == 0 {
		t.Fatalf("export JSON has no providers, got: %+v", exp)
	}
	if len(exp.Connections) == 0 {
		t.Fatal("export JSON has no connections")
	}
	if len(exp.Combos) == 0 {
		t.Fatal("export JSON has no combos")
	}
	if len(exp.ProxyPools) == 0 {
		t.Fatal("export JSON has no proxy_pools")
	}
	if len(exp.APIKeys) == 0 {
		t.Fatal("export JSON has no api_keys")
	}
}

func TestImportConfigLoadsData(t *testing.T) {
	// Create source DB with data.
	srcDir := setupTestDB(t)
	insertTestProviders(t, srcDir)

	// Export to a temp file.
	exportPath := filepath.Join(t.TempDir(), "export.json")
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ExportConfig(srcDir)
	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	if err := os.WriteFile(exportPath, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("write export file: %v", err)
	}

	// Create destination DB (empty).
	dstDir := setupTestDB(t)

	// Import.
	ImportConfig(exportPath, dstDir)

	// Verify data exists in destination DB.
	dbPath := settings.DBPath(dstDir)
	d, err := db.Open(dstDir, dbPath)
	if err != nil {
		t.Fatalf("open dst db: %v", err)
	}
	defer d.Close()

	var count int
	d.QueryRow("SELECT COUNT(*) FROM providers").Scan(&count)
	if count == 0 {
		t.Fatal("imported 0 providers")
	}
	d.QueryRow("SELECT COUNT(*) FROM connections").Scan(&count)
	if count == 0 {
		t.Fatal("imported 0 connections")
	}
	d.QueryRow("SELECT COUNT(*) FROM combos").Scan(&count)
	if count == 0 {
		t.Fatal("imported 0 combos")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	// Create source DB with data.
	srcDir := setupTestDB(t)
	insertTestProviders(t, srcDir)

	// Export.
	exportPath := filepath.Join(t.TempDir(), "export.json")
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	ExportConfig(srcDir)
	w.Close()
	os.Stdout = oldStdout

	var buf strings.Builder
	io.Copy(&buf, r)
	if err := os.WriteFile(exportPath, []byte(buf.String()), 0644); err != nil {
		t.Fatalf("write export: %v", err)
	}

	// Import into fresh DB.
	dstDir := setupTestDB(t)
	ImportConfig(exportPath, dstDir)

	// Re-export from destination.
	oldStdout2 := os.Stdout
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	ExportConfig(dstDir)
	w2.Close()
	os.Stdout = oldStdout2

	var buf2 strings.Builder
	io.Copy(&buf2, r2)

	// Compare provider counts.
	var exp1, exp2 ExportData
	json.Unmarshal([]byte(buf.String()), &exp1)
	json.Unmarshal([]byte(buf2.String()), &exp2)

	if len(exp1.Providers) != len(exp2.Providers) {
		t.Fatalf("provider count mismatch: src=%d dst=%d", len(exp1.Providers), len(exp2.Providers))
	}
	if len(exp1.Connections) != len(exp2.Connections) {
		t.Fatalf("connection count mismatch: src=%d dst=%d", len(exp1.Connections), len(exp2.Connections))
	}
	if len(exp1.Combos) != len(exp2.Combos) {
		t.Fatalf("combo count mismatch: src=%d dst=%d", len(exp1.Combos), len(exp2.Combos))
	}
}
