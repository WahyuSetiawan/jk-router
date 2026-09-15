# PRD — JKRouter

> Local AI Routing Gateway ala 9Router, dibangun ulang dengan **Go (backend) + Frontend ringan (Nuxt 3)**.
> Referensi utama: `/mnt/ssd/Linux/router/9router/` (harus dibaca `CLAUDE.md`, `docs/ARCHITECTURE.md`, `open-sse/AGENTS.md` sebelum implement).
> Status: Draft v1 — 2026-09-15

---

## 1. Ringkasan

JKRouter adalah **local AI routing gateway**: satu endpoint OpenAI-compatible (`/v1/*`) yang me-routing trafik dari AI coding tools (Claude Code, Cursor, Codex, Cline, OpenClaw, Copilot, Gemini CLI, dst.) ke **40+ provider upstream & 100+ model**, dengan:

- **Translasi format** antar client ↔ provider (OpenAI/Anthropic/Gemini/dll), SSE streaming end-to-end.
- **Auto-fallback**: model-combo fallback → multi-account fallback per provider, dengan retry + token refresh otomatis.
- **Manajemen kredensial**: OAuth flow + API key, refresh token, kuota per akun.
- **Usage & quota tracking**: logging permintaan, token usage, harga.
- **Proxy pools**: kumpulan proxy yang bisa di-bound per **connection/akun provider** (bukan global), dengan health-test (auto-deactivate saat gagal), `noProxy` bypass list, dan `strictProxy` (gagal proxy = gagal request, bukan fallback direct).
- **RTK (token saver)**: filter/injeksi prompt yang menghemat 20–40% token.
- **Dashboard web**: konfigurasi provider, akun, model combo, monitoring usage.
- **CLI launcher**: install → start server → buka dashboard → tray icon.

Bedanya dengan 9Router: **engine routing ditulis ulang dalam Go** (bukan Node) untuk single-binary, memory rendah, dan konkurensi SSE yang lebih stabil. 9Router dipakai sebagai spesifikasi perilaku — JKRouter harus *berperilaku sama*, bukan copy kode (Node→Go rewrite).

---

## 2. Masalah

9Router (Node/Next.js) bekerja tapi:
- Runtime Node + dependency SQLite chain (bun:sqlite → better-sqlite3 → node:sqlite → sql.js) itu fragmented dan berat (~500MB node_modules).
- Proxy SSE di balik custom-server Next.js menambah hop dan kompleksitas.
- Single-binary Go: install = satu file, resource minimal, cocok dijalankan sebagai service/tray tanpa Node di machine user.

---

## 3. Tech Stack

| Lapisan | Pilihan | Alasan |
|---|---|---|
| **Engine / Gateway** | Go 1.23+ (`jkserver/`) | single binary net/http, SSE stabil, konkurensi murah. Router: chi (ringan, middleware mudah); stdlib `net/http` method patterns juga cukup — chi dipilih demi middleware logging/auth yang sudah matang. |
| **Persistensi** | SQLite via `modernc.org/sqlite` (pure-Go, no CGO) | nol dependency native → binary statis lintas-OS, menghilangkan seluruh adapter-chain 9Router. File DB tetap `~/.jkrouter/jkrouter.db`. |
| **Frontend Dashboard** | **Nuxt 3 + Vue, Tailwind CSS v4**, build ke static (`nuxt generate`) diserve oleh Go sebagai handler `StaticFS` | Ringan, SSR tidak diperlukan karena semua data dari API Go ring. Alternatif pertimbangan (ditolak): SvelteKit (ekosistem komponen lebih kecil), HTMX (dashboard interaktif/monaco tidak praktis), preact+SSE (rebuild semua chart sendiri). |
| **State dashboard** | Pinia + fetch biasa | tidak perlu library berat; chart pakai `chart.js` (bukan recharts 3MB). |
| **CLI launcher** | Go juga: `jkrouter` = subcommand dari binary yang sama (`jkrouter serve`, `jkrouter dashboard`, `jkrouter update`) | 9Router perlu CLI package terpisah karena Node; Go cukup satu binary dengan `--daemon`, tray via `systray` Go (kebalikan masalah Kaspersky 9Router). |
| **Proxy upstream** | stdlib `net/http` `Transport.Proxy` + `golang.org/x/net/proxy` (SOCKS5) | pengganti `undici ProxyAgent` + `socks-proxy-agent`. Tipe pool native: `http`, `socks5`; relay `vercel`/`cloudflare`/`deno` = plain HTTP call ke URL relay. |
| **Auth/HTTP** | stdlib + `golang.org/x/oauth2` | — |
| **MITM cert** (fitur 9Router `src/mitm`) | `crypto/tls` stdlib + CA self-signed runtime | menggantikan node-forge/selfsigned. |

**Sengaja TIDAK dipakai**: ORM (sqlc atau raw query saja), gRPC, message queue, microservice — semuanya over-engineering untuk app desktop-lokal.

---

## 4. Arsitektur

Paralel langsung dengan 9Router:

| 9Router (Node) | JKRouter (Go) |
|---|---|
| `next.config.mjs` rewrite `/v1/*` → `/api/v1/*` | chi route `/v1/*` langsung |
| `src/sse/handlers/chat.js` | `jkserver/internal/api/chat.go` (parse, combo expansion, account-selection loop) |
| `open-sse/handlers/chatCore.js` | `jkserver/internal/engine/core.go` (detect format, translate, dispatch, retry/refresh) |
| `open-sse/executors/*` | `jkserver/internal/executors/` — `default.go` untuk semua OpenAI-compatible; executor khusus per provider non-OpenAI-compatible |
| `open-sse/translator/*` (pivot OpenAI; direct-route pair utk pair rapuh) | `jkserver/internal/translator/` — interface `Translator{TranslateRequest, TranslateResponse, TranslateStream}`; registry `map[pair]Translator` dengan **direct route** utk pair fragile (thinking blocks, tool ids, non-base64 image, is_error), pivot OpenAI untuk sisanya |
| `open-sse/providers/registry/*` (1 file/provider, auto-generated index) | `jkserver/internal/providers/registry/` — 1 file/provider registrasi ke slice global via `init()`; template `REGISTRY_TEMPLATE.go`. Tiap file mendefinisikan: `models` (katalog manual), `validateUrl` (URL `/models` upstream untuk auto-refresh), `transport`/`transports`, quirks. |
| `open-sse/rtk/*` (caveman, ponytail, headroom, filters) | `jkserver/internal/rtk/` |
| `src/lib/db/*` (adapter chain sqlite) | `jkserver/internal/db/` — satu driver modernc-sqlite, migration embed |
| `src/app/api/proxy-pools/*` + `open-sse/utils/proxyFetch.js` | `jkserver/internal/proxypool/` — CRUD + tester + `Transport` per pool yang dipakai executor upstream |
| `src/lib/db/backup.js` (pre-migration safety backup, exclude requestDetails, retensi 3, tanpa restore) | `jkserver/internal/db/backup.go` — perbaiki: tambah restore + export/import config penuh (lihat §4.3) |
| `cli/` (npm package terpisah) | subcommand dari binary utama |
| Dashboard Next.js + API compat | Nuxt SPA → `/dashboard/*` diserve static; API dashboard = `/api/dashboard/*` di Go |

**Request flow (identik dengan 9Router):**
`/v1/chat/completions` → parse + expand model combo → loop kandidat (combo × akun) → translator request → executor upstream (retry, token refresh, mark kuota) → translator response/stream → SSE ke client. Kegagalan satu kandidat lanjut ke kandidat berikutnya, transparan bagi client.

### 4.1 Detail spesifikasi routing & resilience

**Account states (circuit breaker per connection/akun, in-memory + persist di SQLite):**

| State | Trigger | Perilaku |
|---|---|---|
| `active` | — | Siap menerima request. |
| `cooling_down` | HTTP 429 (atau 409 quota, per provider) | Akun di-skip saat account-selection; durasi dari header `Retry-After` / field provider (`resetAt`), fallback default 60s. 9Router juga punya *strike breaker*: 429 beruntun dengan pembacaan kuota optimistik → lock lebih agresif (port perilaku ini). |
| `disabled` | HTTP 401/403 (key invalid) atau kuota habis periode panjang | Di-skip sampai re-auth / aktifkan manual dari dashboard. |

Transisi state terjadi di engine setelah executor mengembalikan hasil; berlaku untuk **semua** endpoint (chat, embeddings, search, tts/stt, image/video) seperti pola `unavailableResponse` 9Router.

**Mid-stream SSE handling:**
- **Sebelum chunk pertama dikirim ke client** → failover ke kandidat berikutnya masih boleh ( transparan).
- **Setelah chunk pertama (header `data:` pertama) dikirim** → *point of no return*: error upstream diterjemahkan jadi event error SSE OpenAI-compatible (`data: {"error": ...}`), flush, tutup koneksi. Tidak pernah restart stream dari provider lain — itu akan menghasilkan respons terpotong/duplikat yang merusak client.

**Concurrency & SQLite:**
- `PRAGMA journal_mode=WAL; busy_timeout=5000;` saat open (paritas dengan `src/lib/db/schema.js` 9Router), plus checkpoint periodic biar file `-wal` kecil.
- Semua write usage/log dari hot path request dikirim ke **satu writer goroutine via channel buffered** (bukan worker pool — SQLite single-writer, pool justru menambah write contention). Jika channel penuh → drop log + counter, jangan pernah block request. Dashboard/API writes share goroutine yang sama.

### 4.2 Capability-aware routing (vision/image fallback)

Mirip `open-sse/providers/capabilities.js` + `open-sse/services/capacityAdapter.js` + `combo.js#detectRequiredCapabilities` di 9Router:

**Sumber capability per model** (urutan prioritas):
1. `modalities.input/output` dari respons API `/models` upstream (jika provider punya).
2. Katalog manual per-model di `capabilities.go` (paritas `capabilities.js` 9Router).
3. Name-pattern fallback (`visionPatterns.js`) — mis. nama model mengandung `-vision`, `vl`, `multimodal` → di-treat support image. Ini last resort supaya model baru yang belum di-katalog tetap bisa terima gambar.

**Flow saat request membawa gambar:**
1. `detectRequiredCapabilities(body)` → `{vision:true}` (juga deteksi pdf/audio/video input).
2. `reorderByCapabilities(comboModels, [vision])` — kalau ada model di combo yang support vision → model itu di-flot ke urutan depan, sisanya jadi fallback. Request di-serve oleh yang paling depan, tanpa user perlu tahu.
3. Kalau **tidak ada** model di combo yang support vision → `augmentModelsWithCapacityAdapter(models, [vision], settings)`: ambil *capacity adapter pool* (konfigurasi `settings.capacityAdapter.vision.models`), prepend sebagai kandidat pertama. Strategi: `fallback` (default) atau `round-robin` (toggle di Settings).
4. Setelah adapter dipakai, `withCapacityAdapterStripping` membuang kandidat adapter dari log sukses/fail berikutnya supaya UI usage tidak mengira adapter itu anggota combo user.
5. Bonus: `stripHistoryForContext` — jika model tujuan (termasuk adapter) punya context window lebih kecil dari request, history tengah di-trim (sistem + turn user terakhir yang membawa media selalu dijaga) sebelum dikirim.

**Model list per provider — manual atau auto?**
Dua lapis, keduanya:
- **Katalog manual**: field `models: []ModelDef` di tiap file `registry/*.go` — sumber kebenaran untuk `/v1/models` gabungan. Ini yang harus diisi developer saat menambah provider baru. Tidak otomatis.
- **Auto-refresh**: field `validateUrl` (mis. `https://api.deepseek.com/models`) dipanggil backend berkala/tombol "Refresh models" di dashboard. Respons di-parse, capability tiap model (jika ada `modalities`) di-simpan. Ini cuma memperkaya katalog, bukan menggantikan — model yang ada di response upstream tapi belum ada di katalog manual tetap di-tambah otomatis, model yang hilang dari upstream tidak otomatis dihapus dari catalog (biar bisa di-pin manual). Jika provider tidak punya `/models` endpoint, refresh di-skip dan katalog manual satu-satunya sumber.

**Ditambah ke fitur:** Settings mendapat toggle *Capacity Adapter* (per capability: enabled / roundRobin / list models pool) — mirip halaman token-saver, tapi untuk routing gambar/audio.

### 4.3 Backup & restore

9Router hanya punya **safety backup pre-migration** (`src/lib/db/backup.js`): salin DB ke `BACKUPS_DIR` (ekslusi tabel `requestDetails` biar kecil), retensi 3 file, **tanpa jalur restore** — recovery manual copy file balik. JKRouter upgrade jadi paritas fitur penuh:

- **Backup otomatis**: sebelum setiap migrasi skema DB, snapshot penuh ke `~/.jkrouter/backups/<label>-<version>-<timestamp>/jkrouter.db` (metode ATTACH-DB kecil seperti 9Router, eksklusi `requestDetails`). Retensi 10 snapshot (bukan 3 — DB kecil, disk murah).
- **Restore**: `jkrouter restore <path>` (CLI) + tombol di halaman Settings (dashboard). Pilih snapshot dari daftar `backups/`, verifikasi checksum SHA-256 sebelum swap. DB aktif di-lock + WAL checkpoint penuh, file lama di-ganti, service restart graceful.
- **Export/Import config (JSON, portable)**: tombol "Export config" menghasilkan satu file JSON berisi seluruh state konfigurasi (providers, connections, combos, proxy pools, API keys, capacity adapter settings, pricing overrides) — **bukan** tabel usage/requestDetails (terlalu besar & tidak perlu dipindah). Ini upgrade di atas 9Router (yang tidak punya). Cocok untuk: backup ke cloud/sync antar machine, atau pindah dari 9Router (importer membaca format `~/.9router/` + DB-nya, mapping 1:1 di §4.1).
- **Snapshot file tetap bisa di-copy manual** di luar fitur (safety net sama dengan 9Router), tapi tidak lagi satu-satunya jalur recovery.

---

## 5. Fitur (scope v1 — paritas dengan 9Router)

### P1 — Wajib (MVP)
1. **Endpoint `/v1/*` OpenAI-compatible**: `/v1/chat/completions` (streaming + non-streaming), `/v1/models`, `/v1/completions`.
2. **Format translation**: menerima OpenAI Chat & Anthropic Messages; translate ke/format provider. Pivot OpenAI + direct-route pair.
3. **SSE streaming end-to-end** dengan backpressure yang benar (flusher per chunk, `http.Flusher`).
3a. **Capability-aware combo routing (vision/image)** — detail di §4.2: auto-detect modality input, auto-float model vision-capable ke depan combo, capacity-adapter pool global (fallback/round-robin, config di Settings), `stripHistoryForContext` saat model tujuan punya context lebih kecil. Tanpa config user → perilaku langsung benar.
4. **Provider registry**: mulai dari **top-10 provider 9Router** (openai, anthropic/claude, gemini, deepseek, groq, cerebras, openrouter, kiro, opencode-free, glm). Tiap registry file: katalog `models` (manual) + `validateUrl` (auto-refresh dari `/models` upstream, per-provider — lihat §4.2). Sisanya ikut roadmap.
5. **Combo & account fallback**: definisi combo di dashboard; fallback berantai transparan; upstream error 401/429 → coba akun berikutnya.
5a. **Capability-aware combo routing (image/vision)**: request yang membawa gambar otomatis di-cek terhadap capability model target (lihat §4.2). Kalau tidak ada satu pun model di combo yang support, *capacity adapter pool* di-prepend ke urutan coba; kalau ada yang support, model support itu di-flot ke depan (autoSwitch). Tidak perlu konfigurasi user — berlaku otomatis per request.
6. **Credential mgmt**: API key + OAuth flow (callback `/callback/*`), refresh token otomatis, penyimpanan di SQLite (diamankan seperti 9Router).
7. **API key lokal**: generate key dari dashboard, client pakai key itu ke `/v1`.
8. **Usage & logging**: per-request log (model, provider, token in/out, latensi, status) ke SQLite + `~/.jkrouter/log.txt`.
9. **Proxy pools**: CRUD pool (`http`/`socks5`/relay), health-test endpoint (auto `testStatus` + deactivate saat gagal), bind pool ↔ connection provider (per-akun, bukan global), `noProxy` + `strictProxy` honoring, dipakai oleh semua executor upstream. Halaman dashboard terpisah + dropdown di modal tambah API key (meniru `ConnectionsCard.js` 9Router).
9a. **Capacity Adapter settings** (bagian Settings, bukan halaman terpisah): toggle per capability (vision/pdf/audio/video) — enabled, round-robin vs fallback, list models pool. Default off.
10. **Backup & restore** (detail §4.3): safety backup pre-migration (retensi 10) + `jkrouter restore` CLI + tombol restore di Settings + export/import config portable (JSON). Default ON, tanpa config ekstra.
11. **Dashboard (Nuxt)** — lihat inventaris halaman §5.1. P1: Providers, Proxy Pools, Combos, Usage, API Keys, Settings, Logs, Endpoint.
12. **CLI + tray**: `jkrouter` start/stop, auto-open dashboard, tray icon (systray), port default **20128** (dashboard & API satu port).

### 5.1 Inventaris halaman dashboard 9Router → penempatan JKRouter

| Halaman 9Router | JKRouter | Catatan |
|---|---|---|
| providers | **P1** | CRUD + connections + bind proxy pool |
| proxy-pools | **P1** | CRUD + test + bind |
| combos | **P1** | model combo editor |
| usage | **P1** | + sub-tab quota (P2 untuk detail quota per akun) |
| keys (API key lokal) | **P1** | |
| endpoint | **P1** | info URL/key untuk dipakai CLI tools |
| logs / console-log | **P1** | read-only stream |
| settings | **P1** | port, DATA_DIR, tema + tombol Backup/Restore/Export-Import (§4.3) |
| profile | **P1** | login lokal dashboard (password/bcrypt) |
| basic-chat | P2 | playground chat untuk uji routing |
| token-saver (RTK UI) | P2 | bareng RTK |
| translator | P2 | UI debug translasi |
| quota | P2 | detail kuota per akun + window reset |
| cli-tools | P2 | kartu auto-configure per CLI tool (Claude Code, Codex, dll.) |
| media-providers | P2 | TTS/audio providers (elevenlabs, cartesia, dst.) — butuh registry kategori terpisah |
| mitm | P2 | bareng MITM |
| pxpipe | P2 | evaluasi dulu protokolnya; mungkin di-skip |
| skills | P2 | evaluasi kebutuhan |

### 5.2 Wireframe UI

Referensi visual seluruh halaman di atas ada di **`docs/wireframes/wireframe-dashboard.html`** (satu file self-contained, buka langsung di browser). Isinya:

- **8 wireframe halaman P1**: Providers (tabel connection + state badge active/cooling_down/disabled sesuai §4.1 + bind proxy pool), Proxy Pools (kartu pool + health status + test button + relay), Combos (fallback chain list + contoh card *capability-aware routing* §4.2 + card *refresh models per provider*), Endpoint (blok copy-paste per CLI tool + smoke test curl), Usage & Logs (stat cards + chart + tabel request dengan baris failover terpisah), API Keys, Settings (termasuk default resilience §4.1 + block Capacity Adapter), Profile/Login.
- **Sidebar navigasi** konsisten semua halaman: Routing (Providers/Combos/Endpoint) → Infrastructure (Proxy Pools/Usage & Logs) → System (API Keys/Settings/Profile).
- **9 sketsa halaman P2** (basic-chat, token-saver, quota, cli-tools, media-providers, translator, mitm, pxpipe, skills) sebagai garis besar saja — detail menyusul saat fitur di-sprint.
- Palet: Catppuccin Mocha, font monospace — wireframe layout & struktur, bukan final design (implementasi Nuxt tetap boleh beda detail visual).
- Prinsip ke depan: tiap wireframe halaman baru wajib masuk file ini sebelum coding, supaya paritas dengan 9Router bisa diverifikasi per-halaman.

### P2 — Paritas penuh
12. **RTK token saver** (caveman, ponytail, headroom, autodetect, system-inject) — port perilaku dari `open-sse/rtk/`.
13. **Sisanya 40+ provider** — migrasi bertahap sesuai permintaan.
14. **Quota tracking per akun** + reset window.
15. **Proxy pool deployment relay** (vercel/cloudflare/deno worker deploy endpoints seperti `src/app/api/proxy-pools/*-deploy`) — butuh worker template terpisah; jadwalkan terpisah.
16. **Cloud sync (opsional)** — harap protokol 9Router dipelajari dulu (`docs/ARCHITECTURE.md`).
17. **MITM capture mode** (`src/mitm`) untuk debugging protokol provider.
18. **Media providers** (TTS/audio: elevenlabs, cartesia, fish-audio, edge-tts, dll.) — registry kategori terpisah dari chat.
19. **Basic-chat playground** + translator debug UI + kartu auto-configure CLI tools (dashboard `cli-tools` 9Router).
20. **MCP server** (`/api/mcp`), tags, provider-nodes graph, i18n multi-bahasa dashboard.

### Non-goals
- Multi-user/SaaS, DB server (posgres), k8s — project ini tetap **lokal-first**.
- Restrukturisasi perilaku 9Router — targetnya *behavior-compatible* (schema endpoint & respons identik) supaya dokumentasi 9Router berlaku juga.

---

## 6. Struktur Repo (JKRouter)

```
JKRouter/
├── main.go                    # entry: serve / dashboard / CLI subcommands
├── jkserver/
│   ├── internal/
│   │   ├── api/               # chi routes: /v1/*, /api/dashboard/*, /callback/*
│   │   ├── engine/            # core routing: combo expansion, retry, fallback
│   │   ├── executors/         # per-provider upstream (default.go + khusus)
│   │   ├── translator/        # format translation + registry
│   │   ├── providers/registry/# 1 file per provider
│   │   ├── rtk/               # token saver filters
│   │   ├── db/                # sqlite (modernc), migrations embed
│   │   └── settings/          # DATA_DIR resolution (~/.jkrouter)
│   └── web/                   # embed static build Nuxt (go:embed)
├── web/                       # Nuxt 3 project (dashboard)
│   ├── pages/                 # providers, accounts, combos, usage, keys, logs, settings
│   ├── components/
│   └── nuxt.config.ts         # ssr:false, static generate
├── docker-compose.yml
├── Dockerfile                 # multi-stage: node build web → go build embed → scratch/distroless
└── docs/PRD.md
```

---

## 7. Acceptance Criteria (MVP)

- [ ] `go build` menghasilkan **satu binary** `<15MB` yang menjalankan API + dashboard (web di-embed).
- [ ] `./jkrouter serve` → dashboard live di `:20128/dashboard`, `curl :20128/v1/models` mengembalikan daftar model gabungan.
- [ ] Claude Code diarahkan ke `http://localhost:20128` dengan API key dari dashboard → request sukses streaming, ter-translate ke provider upstream.
- [ ] Kegagalan provider #1 otomatis fallback ke #2 tanpa client tahu (log mencatat keduanya).
- [ ] Akun yang kena 429 masuk `cooling_down` (skipped saat selection, muncul di dashboard) dan kembali `active` setelah Retry-After; akun 401 jadi `disabled` sampai diaktifkan manual.
- [ ] Error upstream **setelah** chunk pertama SSE tidak memicu failover — client menerima event `data: {"error":...}` dan koneksi ditutup bersih.
- [ ] OAuth connect minimal 1 provider (contoh: Kiro AI) dari dashboard menyimpan token & refresh otomatis.
- [ ] Proxy pool bisa dibuat dari dashboard, test-nya lulus (testStatus=active), dan request ke provider via pool yang di-bound benar-benar keluar lewat proxy itu (verifikasi via log IP upstream / echo server lokal).
- [ ] Usage page menampilkan token/cost/latensi per request, filter per provider/model/hari.
- [ ] Migrasi skema DB otomatis memicu backup snapshot; `jkrouter restore <path>` mengembalikan DB & service jalan lagi tanpa data hilang (verifikasi: buat combo sebelum restore, masih ada setelahnya).
- [ ] "Export config" menghasilkan file JSON yang bisa di-import di machine lain (fresh install `jkrouter`) dan semua provider/akun/combo/pool sama persis.
- [ ] `go test ./...` hijau untuk: translator round-trip unit tests, combo fallback simulation, dan key auth.

---

## 8. Risiko

| Risiko | Mitigasi |
|---|---|
| Perilaku translator 9Router sangat banyak dan hard to port | Mulai pivot-OpenAI generic; direct-route per-pair dibuat saat ada bug nyata, bukan preemptive. |
| Pure-Go sqlite (modernc) lebih lambat dari better-sqlite3 | Volume data aplikasi lokal kecil; benchmark saat fase usage-logging. |
| Tray di Linux Wayland beragam | Tray = fitur P2; `serve` tanpa tray harus cukup. |
| Provider OAuth flow 9Router punya quirks per-provider (device code, PKCE, endpoint-discovery) | Port satu provider dulu end-to-end, jadikan template, lalu batch. |

---

## 9. Roadmap indikatif

1. **Sprint 1** — Skeleton Go: chi routes, `/v1/models`, `/v1/chat/completions` dengan 1 provider OpenAI-compatible (passthrough), SSE streaming, static Nuxt serve. (~1 pekan)
2. **Sprint 2** — Translator engine + registry + Anthropic client format direct. (~1 pekan)
3. **Sprint 3** — Combo/account fallback + credential + OAuth flow pertama + **proxy pools (CRUD, test, bind, executor via pool)**. (~1 pekan)
4. **Sprint 4** — Dashboard lengkap (semua halaman P1 §5.1) + usage logging + API keys + endpoint/profile + backup/restore (§4.3). (~1–2 pekan)
5. **Sprint 5+** — RTK, provider batch, cloud sync, MITM, packaging (Docker, GitHub Release, install script).

---

## 10. Keputusan terbuka (perlu konfirmasi)

1. Port tetap **20128** (sama dengan 9Router) — biar tool setting lama tinggal ganti URL? *(draft: ya, tapi bikin `--port`)*
2. Lisensi: ikut MIT seperti 9Router.
3. Nama binary: `jkrouter` (confirm).
4. `pxpipe` dan `skills` (dashboard 9Router) — port atau skip permanen? Perlu dipahami dulu nilai pakainya untuk user JKRouter. *(draft: skip di v1, tulis ulang ke sini kalau jadi dibutuhkan)*
5. Dashboard login lokal (password) — 9Router punya (`src/app/login`, bcrypt). Default ON atau OFF untuk localhost-first? *(draft: ON, bind 0.0.0.0 wajib password, localhost opsional)*
6. i18n dashboard: mulai id-ID + en saja, sisanya ikut PR?
