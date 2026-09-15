// Package settings resolves the JKRouter data directory (~/.jkrouter) and core config.
package settings

import (
	"os"
	"path/filepath"
)

// DefaultPort is the dashboard + API port (PRD §10.1: same as 9Router).
const DefaultPort = 20128

// DefaultDataDir is ~/.jkrouter (can be overridden by DATA_DIR env or --data-dir flag).
func DefaultDataDir() string {
	if v := os.Getenv("DATA_DIR"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".jkrouter")
}

// EnsureDataDir creates the data dir + subdirs if missing.
func EnsureDataDir(dir string) error {
	for _, sub := range []string{"", "backups"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func DBPath(dir string) string  { return filepath.Join(dir, "jkrouter.db") }
func LogPath(dir string) string { return filepath.Join(dir, "log.txt") }
func BackupDir(dir string) string {
	return filepath.Join(dir, "backups")
}
