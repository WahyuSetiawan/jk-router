# JKRouter — Status Implementasi

**Terakhir update**: 2026-09-16 20:45  
**Commit terakhir**: `1e55220`  
**Binary**: `/tmp/jkrouter` (12 MB, Go 1.26.7, stripped)  
**Tests**: ✅ 25/25 hijau

---

## Sprint 1 — Skeleton & end-to-end passthrough ✅

| Task | Status |
|------|--------|
| 1.0 Repo scaffolding | ✅ `go.mod`, chi router, settings, CLI flags `--port`/`--data-dir` |
| 1.1 DB layer | ✅ modernc.org/sqlite, WAL mode, single writer goroutine, migrations |
| 1.2 `/v1/models` + `/v1/chat/completions` | ✅ 6 provider: openai, anthropic, gemini, deepseek, groq, cerebras |
| 1.3 Nuxt scaffold + embed | ✅ Catppuccin Mocha, `go:embed`, 10 halaman P1 |
| 1.4 CLI subcommands | ✅ `serve`, `dashboard`, `backup`, `restore`, `export`, `import` |

## Sprint 2 — Translator engine + registry ✅

| Task | Status |
|------|--------|
| 2.1 Translator engine | ✅ Interface + registry + OpenAI↔Claude round-trip test |
| 2.2 Registry expansion | ✅ 10 provider total (+openrouter, kiro, opencode-free, glm) |
| 2.3 Auto-refresh models | ✅ 6-hour interval (`providers/refresh`) |
| 2.4 Capability detection | ✅ `DetectRequiredCapabilities`, `reOrderByCapabilities` |

## Sprint 3 — Fallback, credential, OAuth, proxy pools ✅

| Task | Status |
|------|--------|
| 3.1 Account states + combo fallback | ✅ `AccountStore`, `ExecuteRouting`, state machine |
| 3.2 Credential mgmt | ✅ AES-GCM, machine-key derived |
| 3.3 OAuth flow + token refresh | ✅ OpenAI + Kiro, auto-refresh 5min before expiry |
| 3.4 Proxy pools | ✅ CRUD + health check + `BuildTransport()` |
| 3.5 Capacity adapter | ✅ Vision/pdf/audio/video fallback pool |

## Sprint 4 — Dashboard lengkap + logging + backup ✅

| Task | Status |
|------|--------|
| 4.1 Usage logging | ✅ `usage_log` table, `~/.jkrouter/log.txt`, pricing |
| 4.2 Dashboard P1 pages | ✅ 10 halaman: providers, combos, proxy-pools, usage, api-keys, endpoint, logs, settings, profile, index |
| 4.3 Dashboard API | ✅ `/api/dashboard/*` semua CRUD |
| 4.4 API keys lokal | ✅ Generate + bootstrap key |
| 4.5 Backup pre-migration | ✅ Auto-backup on schema upgrade |
| 4.6 Restore + Export/Import | ✅ `jkrouter restore`, JSON export/import |

## Sprint 5 — Packaging + Paritas P2 🔄

| Task | Status |
|------|--------|
| 5.1 Tray | ⏸️ Deferred — CGO + Wayland risk (PRD §8). `serve` tanpa tray cukup. |
| 5.2 RTK token saver | ✅ Stub exists; **integration ke routing engine DONE session ini** |
| 5.3 Provider batch berikutnya | ⏸️ P2 — tunggu permintaan user |
| 5.4 Quota per akun | ✅ **DONE session ini** — migration 0005, API quota, auto-disable, page `/quota` |
| 5.5 Media providers | ⏸️ P2 — TTS/audio, perlu registry kategori baru |
| 5.6 Packaging | ✅ Dockerfile, install.sh, docker-compose, .goreleaser, CI release |
| 5.7 Cloud sync + MITM | ⏸️ P2 — terakhir setelah packaging stabil |

---

## Review Gap Fixes ✅

| Gap | Fix |
|-----|-----|
| #1 `requiredCaps` wiring | `capsToList()` + `candidateSupportsCaps()` di `buildCandidates()` |
| #2 `WrapRequest` model order | `req.Model = adapterModel` sebelum marshal |
| #3 State persistence | `SaveStateFunc` callback per transition |
| #4 `RouteLogEntry` fields | Tambah `Model`, `AccountID` |
| #5 Proxy timeout leak | Preserve `orig.Timeout` di `BuildTransport()` |
| #6 `findMeta` priority | Exact provider ID match dulu, model ID fallback |
| #7 Legacy `/completions` | Routes melalui `engine.ExecuteRouting` |

---

## Acceptance Criteria PRD §7

| # | Criteria | Status |
|---|----------|--------|
| 1 | Binary <15MB, API+dashboard embed | ✅ 12 MB |
| 2 | `./jkrouter serve` → dashboard :20128, `/v1/models` work | ✅ |
| 3 | Claude Code routing + streaming + translate | ✅ |
| 4 | Auto fallback provider #1 → #2 | ✅ |
| 5 | 429→cooling_down, 401→disabled | ✅ |
| 6 | Mid-stream failover guard (PRD §4.1) | ✅ `StreamGuard atomic.Bool` |
| 7 | OAuth connect + token refresh | ✅ OpenAI + Kiro |
| 8 | Proxy pool CRUD + test + transport | ✅ |
| 9 | Usage page + filter | ✅ |
| 10 | Backup + restore | ✅ |
| 11 | Export/import config JSON | ✅ |
| 12 | `go test ./...` hijau | ✅ 25/25 |

---

## Sprint 5 P2 — Sisa (non-blocking)

| # | Task | Estimasi | Notes |
|---|------|----------|-------|
| 5.1 | Tray (systray) | ~3h | CGO required; skip di Linux Wayland |
| 5.3 | 10 provider batch berikutnya | ~4h | Tunggu list dari user |
| 5.5 | Media providers (TTS/audio) | ~6h | Registry kategori baru |
| 5.7 | Cloud sync + MITM | ~8h | Pelajari protokol 9Router dulu |

---

## Ringkasan Perubahan Session Ini

### Backend (`jkserver/`)
- `engine/account.go`: Field `QuotaLimit`, `QuotaWindowSec`, `QuotaResetAt`
- `engine/routing.go`: 
  - `buildCandidates` skip akun over quota
  - Setelah success, catat token ke `rtk.Saver` + auto-disable jika exceed
  - `QuotaStore *rtk.Saver` di `RoutingConfig`
- `api/router.go`: `loadAccountStore` baca kolom quota; `rtk.New(24h)` injected
- `api/dashboard.go`: `GET/PUT /connections/{id}/quota`
- `db/migrations/0005_quota.sql`: ALTER TABLE accounts ADD 3 kolom

### Frontend (`web/`)
- `pages/quota.vue`: Halaman baru — per-account quota dengan progress bar + editor
- `layouts/default.vue`: Link "Quota" di sidebar

### Dokumentasi
- `docs/IMPLEMENTATION_STATUS.md`: Status sprint-by-sprint lengkap
