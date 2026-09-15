// Package db: SQLite (modernc, pure-Go, no CGO) with WAL + single writer goroutine.
package db

import (
	"context"
	"database/sql"
	"embed"
	"encoding/hex"
	"hash/fnv"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"jkrouter/jkserver/internal/crypto"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// DB wraps *sql.DB plus the single-writer queue and backup hooks.
type DB struct {
	*sql.DB
	dataDir  string
	backupFn func(label string)

	writeCh    chan writeJob
	dropped    atomic.Int64
	checkEvery time.Duration
	stop       chan struct{}
}

type writeJob struct {
	fn func(q *Queue)
}

// Queue is the API a writer job sees — same *sql.DB, just named to discourage
// misuse outside the queued context.
type Queue struct { db *sql.DB }

// DB returns the underlying database connection for use inside writer jobs.
func (q *Queue) DB() *sql.DB { return q.db }

// Open creates/opens the SQLite file with WAL mode, runs migrations (with the
// optional pre-migration backup hook), and starts the single writer goroutine.
func Open(dataDir, dbPath string, opts ...Option) (*DB, error) {
	dsn := "file:" + dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	d := &DB{
		DB:         sqlDB,
		dataDir:    dataDir,
		writeCh:    make(chan writeJob, 256),
		checkEvery: 5 * time.Minute,
		stop:       make(chan struct{}),
	}
	for _, o := range opts {
		o(d)
	}
	if d.backupFn == nil {
		d.backupFn = func(string) {}
	}
	if err := migrate(d); err != nil {
		sqlDB.Close()
		return nil, err
	}

	go d.watcher()
	go d.writer()
	return d, nil
}

type Option func(*DB)

func WithBackupHook(fn func(label string)) Option { return func(d *DB) { d.backupFn = fn } }

func WithWALCheckpointInterval(v time.Duration) Option {
	return func(d *DB) {
		if v > 0 {
			d.checkEvery = v
		}
	}
}

func migrate(d *DB) error {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	if _, err := d.DB.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY)`); err != nil {
		return err
	}
	rows, err := d.DB.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	appliedSet := map[int]bool{}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		appliedSet[v] = true
	}
	rows.Close()

	for i, f := range files {
		version := i + 1
		if appliedSet[version] {
			continue
		}
		d.backupFn(strings.ReplaceAll(f, ".sql", ""))
		raw, err := migrationFS.ReadFile("migrations/" + f)
		if err != nil {
			return err
		}
		if _, err := d.DB.Exec(string(raw)); err != nil {
			return err
		}
		if _, err := d.DB.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, version); err != nil {
			return err
		}
	}
	return nil
}

func (d *DB) EnqueueWrite(fn func(q *Queue)) {
	select {
	case d.writeCh <- writeJob{fn: fn}:
	default:
		d.dropped.Add(1)
	}
}

func (d *DB) EnqueueWriteSync(fn func(q *Queue)) {
	done := make(chan struct{})
	d.writeCh <- writeJob{fn: func(q *Queue) {
		fn(q)
		close(done)
	}}
	<-done
}

func (d *DB) DroppedWrites() int64 { return d.dropped.Load() }

func (d *DB) writer() {
	for job := range d.writeCh {
		job.fn(&Queue{db: d.DB})
	}
}

func (d *DB) watcher() {
	ticker := time.NewTicker(d.checkEvery)
	defer ticker.Stop()
	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			if _, err := d.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
				log.Printf("[db] wal_checkpoint failed: %v", err)
			}
		}
	}
}

func (d *DB) Close() error {
	close(d.stop)
	close(d.writeCh)
	return d.DB.Close()
}

func (d *DB) Context() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-d.stop
		cancel()
	}()
	return ctx
}

func machineID(dataDir string) []byte {
	h := fnv.New64a()
	host, _ := os.Hostname()
	h.Write([]byte(host + dataDir))
	return h.Sum(nil)
}

var machineKeyMu sync.Mutex
var machineKeyCache []byte

func machineKey(dataDir string) []byte {
	machineKeyMu.Lock()
	defer machineKeyMu.Unlock()
	if machineKeyCache == nil {
		machineKeyCache = machineID(dataDir)
	}
	return machineKeyCache
}

func hashKey(plain string) string {
	sum := fnv.New64a()
	sum.Write([]byte(plain))
	return hex.EncodeToString(sum.Sum(nil))
}

func HashAPIKey(plain string) string { return hashKey(plain) }

// --- credential encryption (AES-GCM, machine-bound key) ---

// EncryptSecret encrypts a plaintext credential with the machine key.
func EncryptSecret(plain string) (string, error) {
	return crypto.EncryptGS([]byte(plain), crypto.MachineKey(""))
}

// DecryptSecret decrypts a base64-encoded AES-GCM ciphertext.
func DecryptSecret(encoded string) (string, error) {
	dec, err := crypto.DecryptGS(encoded, crypto.MachineKey(""))
	return string(dec), err
}
