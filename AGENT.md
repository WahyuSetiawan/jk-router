# JKRouter — Agent Guide

Panduan singkat untuk AI agent yang bekerja di repo ini. Bacakan dulu sebelum mulai mengubah apa pun.

## Project Ini Apa

**JKRouter** = local AI routing gateway. Satu endpoint OpenAI-compatible (`/v1/*`) di `:20128` yang me-routing trafik dari AI coding tools (Claude Code, Cursor, Codex, Cline, Copilot, Gemini CLI, dll.) ke **40+ provider & 100+ model**.

- Bahasa: **Go 1.23+** (backend), **TypeScript/Vue 3 + Nuxt 3** (dashboard frontend)
- Bentuk: **single binary** (`/tmp/jkrouter` saat build, bisa dipindah)
- Persistensi: **SQLite** (pure-Go via `modernc.org/sqlite`, no CGO)
- Data dir: `~/.jkrouter/`

## Struktur Repo

```
JKRouter/
├── jkserver/          # Go backend
│   ├── cmd/           # main.go + static embed assets
│   └── internal/
│       ├── api/       # HTTP handlers (chi router)
│       ├── engine/    # routing engine, auto-fallback
│       ├── executors/ # provider executors
│       ├── translator/ # format translate (OpenAI↔Anthropic↔dst)
│       ├── providers/ # registry provider
│       ├── rtk/       # token-saver filter/inject
│       ├── db/        # SQLite layer + WAL writer goroutine
│       ├── settings/  # config resolution (~/.jkrouter)
│       ├── proxypool/ # proxy pool management
│       ├── mcp/       # MCP support
│       └── oauth/     # OAuth flows
├── web/               # Nuxt 3 dashboard
│   ├── pages/         # 15 halaman dashboard
│   ├── composables/
│   ├── plugins/
│   └── nuxt.config.ts
├── docs/              # PRD, TASKS, sprint docs
│   ├── guidelines/    # Testing guidelines & konvensi
│   └── issues/        # Audit report & tracking masalah
├── Makefile           # semua command di sini
└── go.mod / web/package.json
```

## Commands yang Penting

```bash
# Build binary
make build               # → /tmp/jkrouter

# Test
make test                # go test ./...

# Run server
make run                 # :20128, data-dir ~/.jkrouter
make dev                 # build + nuxt dev auto-wait

# Frontend
make build-ui            # cd web && bun build
make embed-ui            # copy .output → jkserver/cmd/
make deploy              # build-ui + embed-ui + build (full pipeline)

# Data
make backup / export / import file=x.json
```

## Package Manager: Bun

Repo ini menggunakan **Bun**, bukan pnpm atau npm:

```bash
# ✅ Benar
bun install
bun dev
bun build
cd web && bun build

# ❌ SALAH — jangan pakai
npm install
npm run dev
pnpm install
npx ...
```

- Bun ada di nix store: `/nix/store/...-bun-1.3.13/bin/bun`
- Lockfile: `bun.lock` (root) dan `web/bun.lock`
- Makefile sudah set `BUN := /nix/store/.../bun`
- Bun secara otomatis migrate dari lockfile pnpm lama saat install

## Go — Aturan

- **No CGO**: semua build `CGO_ENABLED=0`. Go ada di `/nix/store/...-go-1.26.7/bin/go` (pakai langsung, jangan `nix develop`).
- **Chi router**: middleware chain di `jkserver/internal/api/`.
- **DB writer goroutine**: satu channel buffered, nunca block caller.
- **Static embed**: UI Nuxt di-embed ke binary via `//go:embed`.
- **Binary target <15MB** (target PRD). Saat ini ~18MB.

## Frontend — Aturan

- Nuxt 3, Tailwind CSS v4, Pinia + fetch biasa.
- Chart pakai Chart.js (bukan recharts).
- Build output → `web/.output/public/` lalu di-copy ke `jkserver/cmd/`.

## Testing — WAJIB

Guidelines lengkap: **`docs/guidelines/testing.md`**.
Laporan gap audit (status test per fitur PRD): **`docs/issues/audit-testing-2026-09-18.md`**.

Ringkasan aturan:

- **Setiap feature baru wajib dibawa bareng testnya.** Go → `*_test.go` table-driven sebelah kode. Web → `*.test.ts` (Vitest, smoke render + 1 behavior test untuk interaksi).
- **Bug fix wajib regression test** yang gagal sebelum fix, hijau setelah fix.
- **Test hijau = syarat commit**: `make test` (Go) dan `bun test` di `web/` (kalau sentuh UI/composable).
- Go: stdlib `testing` + `httptest` + `t.TempDir()` saja, table-driven. Ikuti pola `internal/db/db_test.go`.
- Web: hanya Vitest + `@vue/test-utils` yang diizinkan. No Playwright/e2e browser (belum diperlukan).
- Jangan commit test yang di-skip tanpa alasan.

### Prioritas Gap Test (lihat audit lengkap di `docs/issues/audit-testing-2026-09-18.md`)

| Prioritas | Area | Status | Tindakan |
|---|---|---|---|
| 🔴 Critical | Auth + RequireAuth middleware | ❌ 0 test | Buat `jkserver/internal/api/auth_test.go` |
| 🔴 Critical | Backup & restore DB | ❌ 0 test | Buat `jkserver/internal/db/backup_test.go` |
| 🔴 Critical | Export/import config JSON | ❌ 0 test | Buat `jkserver/cmd/subcommands_test.go` |
| 🟡 Important | MCP server | ❌ 0 test | Buat `jkserver/internal/mcp/server_test.go` |
| 🟡 Important | Media endpoints (TTS/STT/image) | ❌ 0 test | Buat `jkserver/internal/media/routes_test.go` |
| 🟡 Important | Executors | ❌ 0 test | Buat `jkserver/internal/executors/executor_test.go` |
| 🟢 Low | Dashboard CRUD (POST/PUT/DELETE) | ⚠️ 1 suite (GET+POST only) | Tambah endpoint CRUD di `dashboard_test.go` |
| 🟢 Low | Web/Nuxt frontend | ❌ 0 test (Vitest belum setup) | Install Vitest → buat smoke test per halaman P1

## Workflow Kerja

1. Baca `docs/PRD.md` untuk spesifikasi perilaku.
2. Cek `docs/TASKS.md` untuk status sprint (Sprint 1–6 selesai).
3. Cek `docs/issues/audit-testing-2026-09-18.md` untuk gap testing yang perlu ditutup.
4. Cek `docs/plan/penutupan-gap-testing.md` untuk rencana eksekusi penutupan gap.
5. Sebelum edit, **grep semua caller** fungsi yang mau diubah (root-cause fix, bukan symptom).
6. Tulis/ubah kode **bersama testnya** (lihat `docs/guidelines/testing.md`).
7. Commit setelah test hijau (`make test`, dan `bun test` di `web/` bila sentuh UI).

## Sprint Saat Ini

| Sprint | Status |
|--------|--------|
| 1–6 | ✅ Selesai (2026-09-17) |

**Gap testing yang perlu ditutup**: lihat `docs/plan/penutupan-gap-testing.md` (12 tasks, ~3 jam, 4 phase). Prioritas: auth test → backup/restore → export/import → SSE integration → MCP/media → Vitest setup.

## Deviations / Known Issues

- `jkserver/internal/engine/channel_service.go` punya 7 deviation dari PRD — sudah fixed (commit 47eb2ac3).
- `ArticleController` closure fixes: `$deviceType`/`$type` missing di `byType`/`byVideo` (commits 950a3ff9 + 22130bfb).

## Tips untuk Agent

- **Jangan tambah dependency baru** tanpa keperluan mendesak. Standard library Go + stdlib HTTP cukup.
- **Deletion > addition**. Kalau bisa hapus 10 baris, jangan tambah 5 baris config.
- **Binary size matters**. Setiap import menambah ~1-2MB. Pertimbangkan sebelum pakai library.
- **Frontend thin**: dashboard adalah config UI, bukan app berat. Keep components simple.

---

*Guide ini dibuat untuk AI agent. Update kalau ada perubahan signifikan di arsitektur.*
