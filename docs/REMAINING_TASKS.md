# JKRouter — Remaining Tasks (Status Review 2026-09-17)

> Cross-reference: `docs/PRD.md` §7 acceptance criteria + `docs/TASKS.md` vs real codebase.
> Semua sprint 1–5 sudah di-commit ke master. Task di bawah adalah yang **belum dikerjakan** atau **belum optimal**.

---

## 🔴 Blocking Acceptance Criteria (PRD §7)

### R7.1 Binary <15MB
- **Current**: 18MB (`/tmp/jkr_test.bin`)
- **Target**: <15MB
- **Root cause**: `jkserver/cmd/` embeds full Nuxt `_nuxt/` JS chunks (~511KB) + Go stdlib
- **Fix options**:
  - A. Tree-shake Nuxt chunks (rebuild with `nuxt generate` optimized output)
  - B. Compress embedded assets with gzip in Go handler
  - C. Drop unused Nuxt pages from embed (embed only index.html + critical JS)
- **Estimasi**: 2–3 jam

### R7.2 Claude Code streaming → upstream Anthropic (ter-translate)
- **Current**: Streaming works (`http.Flusher` per chunk), translator registry ada, tapi RTK filters (caveman/ponytail/headroom) **tidak di-applikasi** ke request body sebelum dikirim.
- **Gap**: `engine.ExecuteRouting()` tidak memanggil RTK filter sebelum `exe.ExecuteWithResult()`
- **Fix**: Tambah `cfg.RTKFilters.Apply(&reqBody)` di routing loop sebelum executor dipanggil
- **Estimasi**: 1–2 jam

---

## 🟡 P2 Pages — Missing UI

### P2.12 Token Saver Page
- **Current**: RTK package `internal/rtk/` ada (tracking token usage, sliding window), toggle di Settings page ada tapi **stub** (disabled).
- **Missing**: Halaman dedicated `web/pages/token-saver.vue`
- **Scope**: Toggle caveman/ponytail/headroom, config per-provider token limits, show savings stats
- **Estimasi**: 1 jam

### P2. Dashboard RTK Stats (bukan halaman baru)
- **Current**: `settings.vue` punya section RTK tapi semua checkbox disabled/stub
- **Fix needed**: Hubungkan toggle ke backend setting + aktifkan filter di engine
- **Estimasi**: 30 menit (bersama R7.2)

---

## 🟢 PRD §10 Open Decisions — Sudah Resolved

| # | Keputusan | Status |
|---|---|--------|
| 1 | Port 20128 → `--port` env | ✅ DONE (default 20127 via `.env`) |
| 2 | Lisensi MIT | ✅ DONE |
| 3 | Nama binary `jkrouter` | ✅ DONE |
| 4 | pxpipe/skills skip | ⏸ P2 undecided — belum ada keputusan user |
| 5 | Dashboard login default ON | ✅ DONE (bcrypt) |
| 6 | i18n scope id-ID+en | ✅ DONE |

---

## 📋 Task List untuk Sesudah Ini

| # | Task | Owner | Estimasi | Dependencies |
|---|------|-------|----------|--------------|
| 6.1 | Reduce binary <15MB | BE | 2–3h | — |
| 6.2 | Wire RTK filters ke engine routing | BE | 1–2h | — |
| 6.3 | Buat halaman token-saver.vue | FE | 1h | 6.2 |
| 6.4 | Aktifkan RTK toggle di Settings | FE+BE | 30m | 6.2 |
| 6.5 | Cloud sync (opsional) | BE | 4h | packaging stabil |
| 6.6 | MITM capture mode | BE | 4h | 6.5 |
| 6.7 | Tray systray (deferred) | BE | 3h | CGO, risiko Wayland |
| 6.8 | Provider batch berikutnya | BE | 4h | tunggu permintaan user |
| 6.9 | Proxy relay deploy worker | BE | 3h | vercel/cloudflare template |

---

## ✅ Yang Sudah Selesai (Sprint 1–5 + P2)

| Item | Status | Commit Terakhir |
|------|--------|-----------------|
| Sprint 1: scaffolding, DB, passthrough, Nuxt embed, CLI | ✅ | (multi-commit) |
| Sprint 2: translator engine, 10 registry provider, auto-refresh, capability detection | ✅ | (multi-commit) |
| Sprint 3: account states, combo fallback, OAuth Kiro, proxy pools, capacity adapter | ✅ | (multi-commit) |
| Sprint 4: usage logging, 9 P1 dashboard pages, backup/restore, export/import | ✅ | (multi-commit) |
| Sprint 5.4: Quota per akun | ✅ | `e361d3e` |
| Sprint 5.5: Media providers (TTS/STT/Image/Video) | ✅ | `7bbbbb0` |
| Sprint 5.6: Packaging (Dockerfile, docker-compose, dev.sh) | ✅ | `ac832f2` |
| P2.19: basic-chat, translator, cli-tools pages | ✅ | `93e18de` |
| P2.20: MCP server + i18n ID/EN | ✅ | `967a443`, `c996964` |
| LICENSE MIT | ✅ | `dc918aa` |
| Video generation endpoint + STT buffer fix | ✅ | `dc918aa` |

**Test status**: `go test ./jkserver/...` → **semua hijau** (api, crypto, db, engine, oauth, proxypool, translator)

**Endpoint verify**:
- `GET /v1/models` → 200, 22 model
- `POST /v1/chat/completions` → routing aktif
- `POST /v1/audio/speech` → 401 (perlu auth)
- `POST /v1/api/mcp` → 200, 3 tools
- `GET /api/dashboard/backups` → 200
- `GET /api/dashboard/config/export` → 200
