# JKRouter — Status Implementasi

**Terakhir update**: 2026-09-16  
**Commit**: `16e709a` → `Sprint 5 P2 in progress`  
**Binary**: `/tmp/jkrouter` (12 MB, Go 1.26.7, stripped)  
**Tests**: ✅ 25/25 hijau

---

## Sprint 1 — Skeleton & end-to-end passthrough ✅

| Task | Status | Catatan |
|------|--------|---------|
| 1.0 Repo scaffolding | ✅ | `go.mod`, chi router, settings CLI flags |
| 1.1 DB layer | ✅ | modernc.org/sqlite, WAL mode, single writer goroutine, migrations |
| 1.2 `/v1/models` + `/v1/chat/completions` passthrough | ✅ | 6 provider: openai, anthropic, gemini, deepseek, groq, cerebras |
| 1.3 Nuxt scaffold + embed | ✅ | Catppuccin Mocha, `go:embed`, `jkserver/cmd/embed.go` |
| 1.4 CLI subcommands | ✅ | `serve`, `dashboard`, `backup`, `restore`, `export`, `import` |

---

## Sprint 2 — Translator engine + registry ✅

| Task | Status | Catatan |
|------|--------|---------|
| 2.1 Translator engine | ✅ | Interface + registry + OpenAI↔Claude round-trip test |
| 2.2 Registry expansion | ✅ | 10 provider: +openrouter, kiro, opencode-free, glm |
| 2.3 Auto-refresh models | ✅ | 6-hour interval manager (`providers/refresh`) |
| 2.4 Capability detection | ✅ | `detectRequiredCapabilities`, `reOrderByCapabilities` |

---

## Sprint 3 — Fallback, credential, OAuth, proxy pools ✅

| Task | Status | Catatan |
|------|--------|---------|
| 3.1 Account states + combo fallback | ✅ | `AccountStore`, `ExecuteRouting`, state machine |
| 3.2 Credential mgmt | ✅ | AES-GCM encryption, machine-key derived |
| 3.3 OAuth flow + token refresh | ✅ | OpenAI + Kiro, `/callback/*`, auto-refresh 5min before expiry |
| 3.4 Proxy pools | ✅ | CRUD + health check + `BuildTransport()` |
| 3.5 Capacity adapter | ✅ | Vision/pdf/audio/video fallback model pool |

---

## Sprint 4 — Dashboard lengkap + logging + backup ✅

| Task | Status | Catatan |
|------|--------|---------|
| 4.1 Usage logging | ✅ | `usage_log` table, `~/.jkrouter/log.txt`, pricing |
| 4.2 Dashboard pages (P1) | ✅ | 10 halaman: providers, combos, proxy-pools, usage, api-keys, endpoint, logs, settings, profile, index |
| 4.3 Dashboard API | ✅ | `/api/dashboard/*` — semua CRUD handlers |
| 4.4 API keys lokal UI | ✅ | Generate + bootstrap key |
| 4.5 Backup pre-migration | ✅ | Auto-backup on schema upgrade |
| 4.6 Restore + Export/Import | ✅ | `jkrouter restore`, JSON export/import |

---

## Sprint 5 — Packaging + paritas P2 🔄 In Progress

| Task | Status | Catatan |
|------|--------|---------|
| 5.1 Tray | ⏸️ Deferred | CGO + Wayland risk (PRD §8). `serve` tanpa tray cukup. |
| 5.2 RTK token saver | 🔄 In progress | `internal/rtk/token_saver.go` exists; **integration routing + quota page added** session ini |
| 5.3 Provider batch berikutnya | ⏸️ P2 | 10 provider tambahan — tunggu permintaan user (PRD §5.2) |
| 5.4 Quota per akun | ✅ **DONE session ini** | Migration `0005`, `GET/PUT /connections/{id}/quota`, quota page, auto-disable when exceeded |
| 5.5 Media providers | ⏸️ P2 | TTS/audio — butuh registry kategori terpisah |
| 5.6 Packaging | ✅ | Dockerfile multi-stage, install.sh, docker-compose, .goreleaser, CI release |
| 5.7 Cloud sync + MITM | ⏸️ P2 | Terakhir, setelah packaging stabil |

---

## Review Gap Fixes (Sprint 4→5 transition) ✅

| Gap | Fix | Commit |
|-----|-----|--------|
| #1 `requiredCaps` wiring | `capsToList()` + `candidateSupportsCaps()` in `buildCandidates()` | `0618780` |
| #2 `WrapRequest` order | `req.Model = adapterModel` before marshaling | prior |
| #3 State persistence | `SaveStateFunc` callback per transition | `0618780` |
| #4 `RouteLogEntry` fields | Added `Model`, `AccountID` | `0618780` |
| #5 Proxy timeout leak | Preserves `orig.Timeout` in `BuildTransport()` | `0618780` |
| #6 `findMeta` priority | Exact provider ID match first, model ID fallback | `0618780` |
| #7 Legacy `/completions` | Routes through `engine.ExecuteRouting` | `0618780` |

---

## Acceptance Criteria PRD §7

| # | Criteria | Status |
|---|----------|--------|
| 1 | Binary <15MB, API+dashboard embed | ✅ 12 MB stripped |
| 2 | `./jkrouter serve` → dashboard :20128, `/v1/models` work | ✅ |
| 3 | Claude Code routing + streaming + translate | ✅ |
| 4 | Auto fallback provider #1 → #2 | ✅ |
| 5 | 429→cooling_down, 401→disabled | ✅ |
| 6 | Mid-stream failover guard (PRD §4.1) | ✅ `StreamGuard atomic.Bool` |
| 7 | OAuth connect + token refresh | ✅ OpenAI + Kiro, auto-refresh 5min before expiry |
| 8 | Proxy pool CRUD + test + transport | ✅ `testStatus=active`, bind per account |
| 9 | Usage page + filter | ✅ token/cost/latency per request |
| 10 | Backup + restore | ✅ migration safety + `jkrouter restore` |
| 11 | Export/import config JSON | ✅ |
| 12 | `go test ./...` hijau | ✅ 25/25 |

---

## Sprint 5 P2 — Sisa (non-blocking)

| # | Task | Estimasi | Dependency |
|---|------|----------|-----------|
| 5.1 | Tray (systray) | ~3h | CGO → skip atau platform-specific |
| 5.3 | 10 provider batch berikutnya | ~4h | Tunggu list dari user |
| 5.5 | Media providers (TTS/audio) | ~6h | Registry kategori baru |
| 5.7 | Cloud sync + MITM | ~8h | Pelajari protokol 9Router dulu |
| — | RTK full integration | ~2h | Stub sudah ada, perlu UI toggle |

---

## Ringkasan Perubahan Session Ini

### Backend (`jkserver/`)
- **`engine/account.go`**: Tambah field `QuotaLimit`, `QuotaWindowSec`, `QuotaResetAt`
- **`engine/routing.go`**: 
  - `buildCandidates` cek quota → skip akun over limit
  - Setelah success, catat token ke `rtk.Saver` + cek quota → disable jika exceed
  - Tambah `QuotaStore *rtk.Saver` ke `RoutingConfig`
- **`api/router.go`**: 
  - `loadAccountStore` baca kolom quota dari DB
  - `rtk.New(24h)` di-inject ke `RoutingConfig`
- **`api/dashboard.go`**: 
  - `GET /connections/{id}/quota` — status quota
  - `PUT /connections/{id}/quota` — set limit + window
- **`db/migrations/0005_quota.sql`**: ALTER TABLE accounts ADD 3 kolom

### Frontend (`web/`)
- **`pages/quota.vue`**: Halaman baru — list akun + progress bar + set quota per akun
- **`layouts/default.vue`**: Link "Quota" di sidebar

### CI/Build
- Makefile ldflags injects `Version=0.3.0` + commit + build time
- Binary tetap 12MB (di bawah 15MB target)
