// Package db: backup helpers for pre-migration safety.
package db

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// BackupBeforeMigration copies the current DB to a .gz file in the backup dir.
// Retention: keeps only the last 10 backups.
func BackupBeforeMigration(dbPath, backupDir string) error {
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("backup dir: %w", err)
	}
	ts := time.Now().Format("20060102-150405")
	outPath := filepath.Join(backupDir, fmt.Sprintf("jkrouter-%s.bak.gz", ts))

	src, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create backup: %w", err)
	}
	defer dst.Close()

	gz := gzip.NewWriter(dst)
	if _, err := io.Copy(gz, src); err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	gz.Close()
	dst.Sync()

	// Retention: keep only last 10.
	files, err := filepath.Glob(filepath.Join(backupDir, "jkrouter-*.bak.gz"))
	if err != nil || len(files) <= 10 {
		return nil
	}
	sort.Strings(files)
	for _, f := range files[:len(files)-10] {
		os.Remove(f)
	}
	return nil
}
