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

## Workflow Kerja

1. Baca `docs/PRD.md` untuk spesifikasi perilaku.
2. Cek `docs/TASKS.md` untuk status sprint (Sprint 1-5 selesai, Sprint 6 masih jalan).
3. Cek `docs/REMAINING_TASKS.md` untuk yang tersisa.
4. Sebelum edit, **grep semua caller** fungsi yang mau diubah (root-cause fix, bukan symptom).
5. Commit setelah test hijau (`make test`).

## Sprint Saat Ini

| Sprint | Status |
|--------|--------|
| 1–5 | ✅ Selesai |
| 5.1, 5.6 | ✅ Selesai |
| 6 | 🔄 Dalam progress |

**Gap tersisa** (lihat `docs/REMAINING_TASKS.md`):
- `token-saver.vue` page belum ada (RTK UI dedicated)
- Binary ~18MB (target <15MB)
- RTK filters belum di-wire ke engine routing
- Settings RTK toggle masih stub/disabled

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
