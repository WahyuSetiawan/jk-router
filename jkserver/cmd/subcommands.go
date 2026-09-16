package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/settings"
)

type ExportData struct {
	Providers  []json.RawMessage `json:"providers"`
	Connections []json.RawMessage `json:"connections"`
	Combos     []json.RawMessage `json:"combos"`
	APIKeys    []json.RawMessage `json:"api_keys"`
	ProxyPools []json.RawMessage `json:"proxy_pools"`
	Timestamp  string            `json:"timestamp"`
	Version    string            `json:"version"`
}

// Dashboard opens the browser to the dashboard URL.
func Dashboard(port int) {
	url := fmt.Sprintf("http://localhost:%d/dashboard/", port)
	fmt.Printf("Opening %s\n", url)
	if err := openBrowser(url); err != nil {
		log.Printf("Could not open browser: %v", err)
		fmt.Printf("Please open %s manually\n", url)
	}
}

// Backup runs a pre-migration backup.
func Backup(dataDir string) {
	dbPath := settings.DBPath(dataDir)
	backupDir := settings.BackupDir(dataDir)
	if err := db.BackupBeforeMigration(dbPath, backupDir); err != nil {
		log.Fatalf("backup failed: %v", err)
	}
	fmt.Printf("Backup created in %s\n", backupDir)
}

// Restore restores from a backup .gz file.
func Restore(backupPath, dataDir string) {
	dbPath := settings.DBPath(dataDir)
	if fi, err := os.Stat(backupPath); err != nil || fi.Size() < 10 {
		log.Fatalf("backup file not found or too small: %s", backupPath)
	}
	cleanPath := backupPath
	if strings.HasSuffix(backupPath, ".gz") {
		decompressed := strings.TrimSuffix(backupPath, ".gz")
		cmd := exec.Command("gzip", "-dk", backupPath)
		if err := cmd.Run(); err != nil {
			log.Fatalf("decompress: %v", err)
		}
		cleanPath = decompressed
	}
	tmpPath := filepath.Join(filepath.Dir(dbPath), ".jkrouter-restoring.db")
	if err := copyFile(cleanPath, tmpPath); err != nil {
		log.Fatalf("copy: %v", err)
	}
	if err := os.Rename(tmpPath, dbPath); err != nil {
		log.Fatalf("rename: %v", err)
	}
	fmt.Printf("Restored from %s -> %s\n", cleanPath, dbPath)
}

// ExportConfig exports current database state as JSON to stdout.
func ExportConfig(dataDir string) {
	dbPath := settings.DBPath(dataDir)
	d, err := db.Open(dataDir, dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	exp := ExportData{
		Timestamp: time.Now().Format(time.RFC3339),
		Version:   "0.3.0",
	}

	tables := map[string]*[]json.RawMessage{
		"providers":  &exp.Providers,
		"connections": &exp.Connections,
		"combos":     &exp.Combos,
		"api_keys":   &exp.APIKeys,
		"proxy_pools": &exp.ProxyPools,
	}
	for table, dest := range tables {
		rows, err := d.Query("SELECT * FROM " + table + " LIMIT 0")
		if err != nil {
			continue
		}
		cols, _ := rows.Columns()
		for rows.Next() {
			vals := make([]interface{}, len(cols))
			for i := range vals {
				vals[i] = new([]byte)
			}
			if err := rows.Scan(vals...); err != nil {
				continue
			}
			obj := make(map[string]interface{})
			for i, col := range cols {
				b := vals[i].(*[]byte)
				obj[col] = string(*b)
			}
			b, _ := json.Marshal(obj)
			*dest = append(*dest, b)
		}
	}
	out, _ := json.MarshalIndent(exp, "", "  ")
	fmt.Println(string(out))
}

// ImportConfig imports config from a JSON file into the database.
func ImportConfig(importPath, dataDir string) {
	data, err := os.ReadFile(importPath)
	if err != nil {
		log.Fatalf("read import file: %v", err)
	}
	var exp ExportData
	if err := json.Unmarshal(data, &exp); err != nil {
		log.Fatalf("parse JSON: %v", err)
	}
	dbPath := settings.DBPath(dataDir)
	d, err := db.Open(dataDir, dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer d.Close()

	importItems := func(table string, items []json.RawMessage) {
		for _, item := range items {
			var row map[string]interface{}
			json.Unmarshal(item, &row)
			labels := make([]string, 0, len(row))
			vals := make([]interface{}, 0, len(row))
			for k, v := range row {
				labels = append(labels, k)
				vals = append(vals, v)
			}
			ph := make([]string, len(labels))
			for i := range ph {
				ph[i] = "?"
			}
			q := fmt.Sprintf("INSERT OR REPLACE INTO %s (%s) VALUES (%s)",
				table, strings.Join(labels, ","), strings.Join(ph, ","))
			d.EnqueueWriteSync(func(qb *db.Queue) {
				qb.DB().Exec(q, vals...)
			})
		}
	}

	importItems("providers", exp.Providers)
	importItems("connections", exp.Connections)
	importItems("combos", exp.Combos)
	importItems("api_keys", exp.APIKeys)
	importItems("proxy_pools", exp.ProxyPools)
	fmt.Printf("Imported config from %s\n", importPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = in.Seek(0, 0)
	if err != nil {
		return err
	}
	_, err = copyBuffer(out, in)
	return err
}

func copyBuffer(dst, src *os.File) (int64, error) {
	_, err := dst.Seek(0, 0)
	if err != nil {
		return 0, err
	}
	_, err = dst.ReadFrom(src)
	return 0, err
}

func openBrowser(url string) error {
	switch os.Getenv("XDG_SESSION_TYPE") {
	case "wayland":
		return exec.Command("wlrun", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

// Update checks for a newer release on GitHub and replaces the running binary.
func Update() {
	self, err := os.Executable()
	if err != nil {
		log.Fatalf("update: cannot find self: %v", err)
	}
	dst := self + ".new"

	// Determine download URL from runtime info.
	osName := runtime.GOOS
	arch := runtime.GOARCH
	if osName == "linux" {
		osName = "linux"
	}
	filename := fmt.Sprintf("jkrouter-%s-%s", osName, arch)
	url := fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", githubRepo, filename)
	fmt.Printf("Checking %s ...\n", url)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		log.Fatalf("update: HTTP %d — no release found for %s/%s", resp.StatusCode, osName, arch)
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		log.Fatalf("update: %v", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(dst)
		log.Fatalf("update: %v", err)
	}
	out.Close()

	// Atomic swap: rename old binary to backup, then rename new in place.
	backup := self + ".bak"
	if err := os.Rename(self, backup); err != nil {
		os.Remove(dst)
		log.Fatalf("update: cannot backup current binary: %v", err)
	}
	if err := os.Rename(dst, self); err != nil {
		os.Rename(backup, self) // rollback
		log.Fatalf("update: cannot install new binary: %v", err)
	}
	os.Remove(backup) // clean up backup
	fmt.Println("Updated successfully. Restart jkrouter to use the new version.")
}
