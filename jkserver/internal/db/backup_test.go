package db

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestBackupBeforeMigrationCreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create a minimal DB file.
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	f.WriteString("fake db content for backup test")
	f.Close()

	err = BackupBeforeMigration(dbPath, backupDir)
	if err != nil {
		t.Fatalf("BackupBeforeMigration: %v", err)
	}

	files, err := filepath.Glob(filepath.Join(backupDir, "jkrouter-*.bak.gz"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 backup file, got %d", len(files))
	}

	// Verify the backup is a valid gzip file.
	gzFile, err := os.Open(files[0])
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer gzFile.Close()
	gz, err := gzip.NewReader(gzFile)
	if err != nil {
		t.Fatalf("gzip new reader: %v", err)
	}
	defer gz.Close()
	content, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	if string(content) != "fake db content for backup test" {
		t.Fatalf("backup content mismatch: %q", string(content))
	}
}

func TestBackupRestoreRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	backupDir := filepath.Join(tmpDir, "backups")

	// Write some content to the DB.
	f, err := os.Create(dbPath)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	f.WriteString("important data that must be preserved")
	f.Close()

	// Create backup.
	if err := BackupBeforeMigration(dbPath, backupDir); err != nil {
		t.Fatalf("backup: %v", err)
	}

	// Find the backup file.
	files, err := filepath.Glob(filepath.Join(backupDir, "jkrouter-*.bak.gz"))
	if err != nil || len(files) != 1 {
		t.Fatalf("find backup: %v", err)
	}

	// Decompress to a new path (simulating what Restore does manually).
	decompressed := filepath.Join(tmpDir, "restored.db")
	gzFile2, err := os.Open(files[0])
	if err != nil {
		t.Fatalf("open backup: %v", err)
	}
	defer gzFile2.Close()
	gz, err := gzip.NewReader(gzFile2)
	if err != nil {
		t.Fatalf("gzip new reader: %v", err)
	}
	defer gz.Close()
	dst, err := os.Create(decompressed)
	if err != nil {
		t.Fatalf("create restored: %v", err)
	}
	if _, err := io.Copy(dst, gz); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	dst.Close()

	// Verify restored content matches original.
	restored, err := os.ReadFile(decompressed)
	if err != nil {
		t.Fatalf("read restored: %v", err)
	}
	if string(restored) != "important data that must be preserved" {
		t.Fatalf("restored content mismatch: %q", string(restored))
	}
}

func TestBackupRetentionKeepsOnlyLast10(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "jkrouter.db")
	backupDir := filepath.Join(tmpDir, "backups")

	// Create 12 backups sequentially.
	for i := 0; i < 12; i++ {
		f, err := os.Create(dbPath)
		if err != nil {
			t.Fatalf("create db: %v", err)
		}
		f.WriteString(fmt.Sprintf("backup number %d", i))
		f.Close()

		if err := BackupBeforeMigration(dbPath, backupDir); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}

	files, err := filepath.Glob(filepath.Join(backupDir, "jkrouter-*.bak.gz"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) > 10 {
		t.Fatalf("expected <= 10 backups, got %d", len(files))
	}
}
