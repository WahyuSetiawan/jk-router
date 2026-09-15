// Package main is the JKRouter entry point.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"jkrouter/jkserver/internal/api"
	"jkrouter/jkserver/internal/db"
	"jkrouter/jkserver/internal/providers/refresh"
	"jkrouter/jkserver/internal/settings"
	"jkrouter/jkserver/internal/translator"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		args = []string{"serve"}
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		if args[0] == "--help" || args[0] == "-h" {
			fmt.Fprintf(os.Stderr, `JKRouter - AI Routing Gateway

Usage:
  jkrouter [command] [flags]

Commands:
  serve      Start the server (default)
  dashboard  Open the dashboard in browser
  backup     Create a pre-migration backup
  restore    Restore from a backup file (.gz)
  export     Export current config as JSON to stdout
  import     Import config from JSON file

Flags (global):
  --data-dir string   Data directory (default ~/.jkrouter)
  --port int          HTTP port (default 20128)

Examples:
  jkrouter serve --port 8080
  jkrouter dashboard
  jkrouter backup
  jkrouter restore /tmp/backup.gz
  jkrouter export --data-dir /tmp/mydata > config.json
  jkrouter import config.json --data-dir /tmp/mydata
`)
			os.Exit(0)
		}
		args = []string{"serve"}
	}

	dataDir := settings.DefaultDataDir()
	port := settings.DefaultPort
	for i, a := range args {
		if a == "--data-dir" && i+1 < len(args) {
			dataDir = args[i+1]
		}
		if a == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				port = p
			}
		}
	}

	cmd := args[0]
	switch cmd {
	case "serve", "":
		runServer(dataDir, port)
	case "dashboard":
		Dashboard(port)
	case "backup":
		Backup(dataDir)
	case "restore":
		if len(args) < 2 {
			log.Fatal("usage: jkrouter restore <backup.gz> [--data-dir /path]")
		}
		Restore(args[1], dataDir)
	case "export":
		ExportConfig(dataDir)
	case "import":
		if len(args) < 2 {
			log.Fatal("usage: jkrouter import <config.json> [--data-dir /path]")
		}
		ImportConfig(args[1], dataDir)
	default:
		log.Fatalf("unknown command: %s", cmd)
	}
}

func runServer(dataDir string, port int) {
	if err := settings.EnsureDataDir(dataDir); err != nil {
		log.Fatalf("cannot create data dir %s: %v", dataDir, err)
	}

	dbPath := settings.DBPath(dataDir)
	if err := db.BackupBeforeMigration(dbPath, settings.BackupDir(dataDir)); err != nil {
		log.Printf("[backup] pre-migration backup skipped: %v (non-fatal)", err)
	}
	d, err := db.Open(dataDir, dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer d.Close()

	transReg := translator.NewRegistry()
	translator.RegisterOpenAIClaudeTranslators(transReg)

	key, err := api.EnsureBootstrapKey(d)
	if err != nil {
		log.Fatalf("bootstrap key: %v", err)
	}
	fmt.Fprintf(os.Stderr, "\n  jkrouter bootstrap key: %s\n", key)
	fmt.Fprintf(os.Stderr, "  data dir : %s\n", dataDir)
	fmt.Fprintf(os.Stderr, "  listening: http://localhost:%d\n\n", port)

	refMgr := refresh.NewManager(refresh.WithInterval(6*time.Hour), refresh.WithAPIKey(key))
	refMgr.Start()
	defer refMgr.Stop()

	api.SeedPricing(d)

	logPath := settings.LogPath(dataDir)
	ul := api.NewUsageLogger(d, logPath)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	v1 := chi.NewRouter()
	v1.Mount("/", api.Router(d, transReg, ul))
	r.Mount("/v1", v1)

	r.Get("/api/dashboard/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"ok","ts":"%s"}`, time.Now().Format(time.RFC3339))
	})
	r.Mount("/api/dashboard", api.DashboardRouter(d))

	r.Handle("/dashboard/*", http.StripPrefix("/dashboard/", http.FileServer(http.FS(webFS))))

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/dashboard/", http.StatusMovedPermanently)
		_ = req
	})
	r.Get("/dashboard", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/dashboard/", http.StatusMovedPermanently)
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("starting jkrouter on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
