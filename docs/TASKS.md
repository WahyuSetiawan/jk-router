# JKRouter — Task Breakdown

> Rujukan: `docs/PRD.md`. Tiap task: **owner** = modul Go/file yang disentuh, **done** = kriteria terukur (bisa dicek tanpa membaca ulang seluruh PRD). Task dalam satu sprint **berurutan** (bisa dikerjakan paralel antar-sprint setelah Sprint 1 selesai). Angka = estimasi hari kerja 1 dev.
> Konvensi: `[BE]` backend Go, `[FE]` Nuxt, `[X]` keduanya, `[DOC]`.

---

## Sprint 1 — Skeleton & end-to-end passthrough (~1 minggu)

### 1.0 `[BE]` Scaffolding repo
- `go mod init jkrouter`; struktur folder sesuai PRD §6 (`jkserver/internal/{api,engine,executors,translator,providers,rtk,db,settings,proxypool}`, `web/`, `main.go`).
- Chi router registered: `/health`, `/v1/*` (middleware auth key placeholder), `/api/dashboard/*`, static Nuxt placeholder di `/dashboard`.
- `settings.ResolveDataDir()` → `~/.jkrouter` (`DATA_DIR` env override); auto-create dir saat start.
- **Done**: `go build ./...` hijau; `jkrouter serve` (flag `--port`, default 20128) menjalankan server; `curl :20128/health` → 200.

### 1.1 `[BE]` DB layer (WAL + writer goroutine)
- `internal/db`: driver `modernc.org/sqlite`, open dengan `PRAGMA journal_mode=WAL; busy_timeout=5000;` (PRD §4.1).
- Migration embed: file `.sql` di `db/migrations/`, runner urut, guard "sudah jalan" idempotent.
- **Writer goroutine tunggal + channel buffered** (bukan worker pool, §4.1): `db.EnqueueWrite(func)` — full channel = drop + counter, tidak pernah block caller. Periodic WAL checkpoint (interval dari settings, default 5m).
- Tabel awal: `providers`, `connections`, `combos`, `api_keys`, `settings`, `usage_log` (skema final menyusul Sprint 4 — cukup placeholder minimal untuk Sprint 1-3, di-migrasi nanti).
- **Done**: unit test `WriterConcurrency` — 100 goroutine enqueue parallel, 0 `database is locked`, 0 loss kecuali sengaja channel penuh (counter naik, request tetap 200).

### 1.2 `[BE]` `/v1/models` + `/v1/chat/completions` passthrough 1 provider
- 1 registry file contoh: `providers/registry/openai.go` — bentuk final: `models` katalog manual + `validateUrl` + `transports` (PRD §4.2, item 4).
- Executor `default.go`: OpenAI-compatible passthrough (tanpa translasi — Sprint 2 baru menambah).
- Non-streaming + **streaming SSE** (`http.Flusher` per chunk, §P1.3). Auth: Bearer key lokal dari tabel `api_keys` (generate 1 key bootstrap saat first-run, tampilkan 1x di terminal — dashboard UI-nya menyusul Sprint 4).
- **Done**: `curl :20128/v1/chat/completions` dengan `OPENAI_API_KEY` asli → respons streaming nyata dari OpenAI; `curl :20128/v1/models` → daftar model.

### 1.3 `[X]` Nuxt scaffold + embed ke binary
- `web/`: `nuxt@3` + `vue` + `tailwindcss@4` + `pinia` + `chart.js`; `ssr:false`, `nuxt generate` → static; palet Catppuccin Mocha sesuai wireframe (`docs/wireframes/`).
- `jkserver/web/`: `go:embed` hasil generate; serve di `/dashboard/*` (SPA fallback index.html).
- Halaman awal: **hanya** 1 placeholder (dashboard shell + sidebar layout sesuai wireframe §5.2, kosong isi) — halaman nyata mulai Sprint 4.
- **Done**: `go build` → 1 binary `<15MB` berisi dashboard (PRD §7), `:20128/dashboard` menampilkan shell.

### 1.4 `[BE]` CLI subcommand dasar
- `main.go`: `serve` (foreground), `dashboard` (buka browser ke URL dashboard), `restore` (placeholder — diisi Sprint 4.6). Flag global `--port`, `--data-dir`.
- Tray (`systray`) ditunda ke Sprint 5 (P2, sesuai PRD §8 risiko Wayland).
- **Done**: `./jkrouter serve` jalan; `./jkrouter dashboard` membuka browser.

**Gate Sprint 1**: semua §7 acceptance criteria yang hanya butuh 1 provider hijau (item 1,2,3 dari daftar §7).

---

## Sprint 2 — Translator engine + registry (~1 minggu)

### 2.1 `[BE]` Translator engine
- `internal/translator`: interface `Translator{TranslateRequest, TranslateResponse, TranslateStream(ctx, src io.Reader) (io.ReadCloser, error)}` + registry `map[pair]Translator` (pair = `fromFormat:toFormat`), pivot OpenAI default, direct-route hanya untuk pair rapuh (thinking blocks, tool ids, non-base64 image, is_error — catatan PRD §4).
- Format supported tahap awal: `openai` (client), `claude`/anthropic-messages (client + target), `gemini` (target).
- **Done**: test round-trip: `openai→claude→openai` identik modulo lossy field (list lossy per-pair terdokumentasi di test, bukan guesswork).

### 2.2 `[BE]` Registry + executor khusus (5 provider core)
- File per provider (template `REGISTRY_TEMPLATE.go`): anthropic, gemini, deepseek, groq, cerebras — `transports` (termasuk multi-endpoint format-aware ala `deepseek.js` 9Router), `models` katalog awal, `validateUrl` bila ada.
- `openrouter`, `kiro`, `opencode-free`, `glm` menyusul (bisa paralel, 1 sesi tiap hari).
- Executor `default.go` dipakai semua OpenAI-compatible (groq, cerebras, openrouter, glm); anthropic & gemini & deepseek-anthropic-endpoint dapat executor khusus sesuai `transports`.
- **Done**: `/v1/chat/completions` dari client OpenAI berhasil di-translate & balik lagi untuk tiap provider core (manual test dengan key masing-masing, log per-provider).

### 2.3 `[BE]` Auto-refresh models (`validateUrl`)
- Job berkala + pemicu manual: fetch `validateUrl` per provider, parse response upstream, **tambah** model baru + capability (`modalities`) ke DB; **tidak pernah hapus** model katalog manual (PRD §4.2).
- Cache hasil di SQLite; fallback: provider tanpa `/models` di-skip diam-diam.
- **Done**: ubah response `/v1/models` setelah simulasi upstream menambah 1 model baru (test mock HTTP server, bukan live call).

### 2.4 `[BE]` Capability detection + reordering (§4.2 step 1–2)
- `detectRequiredCapabilities(body)`: scan `messages[].content[]` type `image_url`/`image` (OpenAI) dan `type:"image"` block (Anthropic) → set `{vision}` (pdf/audio/video ditambahkan nanti bareng media providers P2, cukup stub function signature sekarang).
- `reorderByCapabilities(models, required)`: float model support ke depan.
- Test fixture: request dengan gambar + combo `[text-only-a, vision-b]` → a diturukan, b dipanggil.

**Gate Sprint 2**: PRD §7 item 2 (Claude Code → JKRouter → upstream Anthropic, streaming, ter-translate) hijau.

---

## Sprint 3 — Fallback, credential, OAuth, proxy pools, capacity adapter (~1–1.5 minggu)

### 3.1 `[BE]` Account states + combo fallback (§4.1 + P1.5)
- State machine `active/cooling_down/disabled` per connection (in-memory map + persist flag `disabled` ke DB): 429/409 → `cooling_down` sampai `Retry-After`/`resetAt` (default 60s); 401/403 → `disabled` manual re-enable.
- Strike-breaker sederhana (port pola `antigravityQuota.js`: N 429 berturut → lock lebih lama).
- `handleComboChat`-equivalent: loop combo × akun, **failover hanya sebelum chunk pertama** (guard `streamStarted bool` di handler SSE — setelah itu emit `data: {"error":...}` + tutup, PRD §4.1).
- **Done**: test simulasi — akun1 upstream 429, akun2 sukses, respons akhir 200, kedua percobaan tercatat di `usage_log` dengan flag `fallback=true` untuk baris pertama.

### 3.2 `[BE]` Credential mgmt: API key + refresh token
- Simpan per connection di SQLite: `auth_type` (apikey|oauth), secret ter-enkripsi (AES-GCM, key diturunkan dari machine-id — pola 9Router, bukan plain text).
- Token refresh: per-provider refresher (background goroutine cek expiry, dedup concurrent refresh — sama dengan `antigravityQuota.js` 9Router).
- **Done**: token OAuth kedaluwarsa di-refresh otomatis tanpa user noticing (test mock upstream yang menolak token lama + token baru).

### 3.3 `[BE]` OAuth flow pertama (Kiro, sesuai PRD §7)
- `/api/dashboard/oauth/kiro/start` → redirect device/browser flow (ikuti quirk per provider dari `src/app/api/oauth/[provider]` 9Router — baca dulu, jangan menebak) + `/callback/*` handler → simpan token.
- **Done**: connect Kiro dari curl/dashboard flow tersimpan + `/v1/chat/completions` via Kiro sukses.

### 3.4 `[BE]` Proxy pools (P1.9)
- `internal/proxypool`: CRUD (tabel `proxy_pools`, field: name/type[http|socks5|relay]/proxyUrl/noProxy/isActive/strictProxy/testStatus/lastTestedAt).
- `Transport` per pool: http → `http.ProxyURL`, socks5 → `golang.org/x/net/proxy`, relay → direct HTTP ke URL relay. Cache transport per pool ID (bukan per-request).
- `noProxy` honoring: request ke host di list bypass → direct. `strictProxy` true: gagal proxy = error, bukan fallback direct.
- Health test endpoint + **auto-deactivate saat test gagal**; binding `connection.proxy_pool_id` (per-akun).
- **Done**: echo-server lokal sebagai pool `http` → request via akun yang di-bound keluar lewat server itu (verifikasi IP/hostname dari sisi server), pool gagal test → `isActive=false` otomatis.

### 3.5 `[BE]` Capacity adapter + stripHistoryForContext (§4.2 step 3–5)
- `settings.capacityAdapter` (per capability: enabled/roundRobin/models[]) — default pool kosong = `oc/mimo-v2.5-free` (paritas 9Router).
- `augmentModelsWithCapacityAdapter` prepend + strategy; `withCapacityAdapterStripping` di log sukses/fail; `stripHistoryForContext` trim middle (head system + tail user-with-media dipertahankan).
- **Done**: combo tanpa vision model + request bergambar → adapter dipanggil, usage_log menampilkan adapter sebagai `adapter=true` terpisah dari anggota combo.

**Gate Sprint 3**: PRD §7 item 4,5,6,8,9 hijau.

---

## Sprint 4 — Dashboard lengkap + logging + backup (~1.5–2 minggu)

### 4.1 `[BE]` Usage logging + pricing
- `usage_log` (request_id, combo, model_actual, provider, account_id, fallback_from, state_at_start, tok_in, tok_out, latensi_ms, status, adapter_used) via writer goroutine Sprint 1.1; mirror ke `~/.jkrouter/log.txt` (tail, bukan rewrite).
- Pricing: `pricing` tabel (per model, input/output per-1k, update dari `pricing.js` 9Router sebagai seed) + cost calc.
- **Done**: `GET /api/dashboard/usage?provider&model&date` + agregasi stat cards (PRD wireframe §5.2 halaman Usage) menghasilkan angka yang sama dengan manual-count dari `usage_log`.

### 4.2 `[FE]` Halaman dashboard P1 (8 halaman, wireframe sebagai acuan per halaman)
Urutan dikerjakan (tergantung API yang sudah ada):
1. **Providers** — list + tabel connections (state badge, proxy pool kolom, priority reorder) + modal Add API key (dropdown proxy pool) + modal Connect OAuth (Kiro).
2. **Proxy Pools** — CRUD + Test button + status health.
3. **Combos** — editor list fallback + card capability-aware (wireframe) + refresh models per provider.
4. **Endpoint** — copy-paste blok (URL + key + snippet per CLI tool) + curl smoke test.
5. **API Keys** — generate/revoke/copy.
6. **Usage & Logs** — stat cards + chart.js + tabel request (baris failover terpisah) + live log tail.
7. **Settings** — port/DATA_DIR/tema + resilience defaults (Sprint 4.6) + Capacity Adapter toggles (3.5) + Backup/Restore/Export-Import (4.6).
8. **Profile/Login** — setup wizard first-run (password bcrypt, bind 0.0.0.0 wajib password sesuai keputusan terbuka §10.5 draft; localhost opsional).
- Semua halaman fetch `/api/dashboard/*` — **API controller-nya dibuat di 4.3**, FE 4.2 mulai setelah route yang di-deps tersedia (urutan nomor di atas = urutan deps backend).
- **Done**: tiap halaman lulus ceklist wireframe (struktur panel sama, tidak pixel-perfect), dark Catppuccin.

### 4.3 `[BE]` Dashboard API controllers (`/api/dashboard/*`)
- CRUD: providers, connections, proxy-pools, combos, api-keys, usage query, settings get/put, logs tail, oauth start/status, models refresh trigger, capacity-adapter settings. Auth: dashboard session (bcrypt password dari 4.2.8) — bukan key API `/v1`.
- **Done**: semua endpoint dipakai halaman 4.2 live (bukan fetch ke endpoint yang belum diimplementasi).

### 4.4 `[BE]` API keys lokal UI
- Halaman Keys (4.2.5) + generate endpoint: key format `jk_...`, hash simpan, plain hanya ditampilkan 1x (copy-once) — poladengan 9Router `apiKeysRepo.js`.
- **Done**: key baru langsung bisa dipakai di `/v1` (test e2e curl), revoke → key lama 401.

### 4.5 `[BE]` Backup pre-migration
- Hook di migration runner (Sprint 1.1): sebelum apply migration, snapshot ATTACH-DB kecil (eksklusi `usage_log`) ke `~/.jkrouter/backups/`, retensi 10, checksum SHA-256 tersimpan.
- **Done**: upgrade skema DB (uji: tambah migration dummy) → folder backup baru muncul, lama ter-prune di atas 10.

### 4.6 `[BE]` Restore + Export/Import config (§4.3)
- `jkrouter restore <path>` CLI: validasi checksum + versi skema → lock DB, WAL checkpoint penuh, swap file, graceful restart service.
- `POST /api/dashboard/backup/restore` (tombol Settings) = jalur sama.
- Export: 1 JSON (providers, connections, combos, proxy_pools, api_keys, settings, capacityAdapter, pricing overrides — **bukan** usage_log/requestDetails); Import: validasi skema + merge rule (nama duplikat = overwrite, baru = append) + **importer 9Router**: baca `~/.9router/` (db.json lama + DB sqlite + `usage.json`/`log.txt` diabaikan) mapping 1:1.
- **Done**: PRD §7 item backup/restore/export-import semua hijau (export → fresh install → import → data identik per-entity).

**Gate Sprint 4**: PRD §7 item 1,7,10,11 + seluruh wireframe halaman P1 fungsional.

---

## Sprint 5 — Packaging & paritas P2 awal (~1–2 minggu, scope bisa di-pause)

### 5.1 `[BE]` Tray + auto-open
- `systray` (Linux `libappindicator`-based / Windows native / macOS `Cocoa`); menu: open dashboard, copy API key, stop. Linux Wayland = graceful fallback (no tray, tetap jalan — PRD §8).
- **Done**: `jkrouter` (tanpa subcommand) = serve + tray di 3 OS target (Windows/macOS wajib, Linux X11 wajib, Wayland boleh no-tray).

### 5.2 `[BE]` RTK token saver (P2.12)
- Port perilaku `open-sse/rtk/` (caveman, ponytail, headroom, autodetect, system-inject) ke `internal/rtk/` + halaman token-saver (wireframe P2 sudah ada, detail menyusul).
- **Done**: toggle ON → delta token usage terukur vs baseline (uji: 2 run identik ON/OFF, bandingkan `usage_log`).

### 5.3 `[X]` Provider batch berikutnya (P2.13) — 10 provider tambahan, template dari Sprint 2.2, prioritasi sesuai permintaan user (jangan menebak urutan).

### 5.4 `[BE]` Quota per akun (P2.14)
- Halaman quota (wireframe P2) + tracking window reset (sprint/weekly/midnight, detil provider-specific resetAt ala `antigravityQuota.js`).
- **Done**: akun antigravity (atau provider serupa) 429 karena kuota → status dashboard menampilkan reset window akurat, bukan hanya "cooling 60s".

### 5.5 `[X]` Media providers (P2.18) — TTS/STT/image/video: registry kategori baru + executor + endpoint `/v1/audio/*`, `/v1/images/*`, `/v1/videos/*` (mirip 9Router `imageGeneration.js`/`tts.js`/`stt.js`/`videoGeneration.js` handlers).

### 5.6 `[BE]` Packaging
- `Dockerfile` multi-stage (node build web → go build → distroless), `docker-compose.yml`, GitHub Release (release-please atau manual per tag, 3 OS binary + install script), `jkrouter update` self-update (unduh + swap binary, pattern: backup binary lama dulu).
- **Done**: user fresh OS install 1 perintah → dashboard live.

### 5.7 `[BE]` Cloud sync + MITM (P2.16, P2.17) — terakhir, after packaging stabil; MITM = `crypto/tls` CA self-signed (PRD §3).

---

## Di luar sprint (P2 sisa, dipicu permintaan, bukan jadwal)
- Basic-chat playground, translator debug UI, kartu cli-tools (P2.19)
- MCP server, tags, provider-nodes graph, i18n (P2.20)
- Proxy relay worker deploy (vercel/cloudflare/deno) (P2.15)
- pxpipe/skills (P2 — undecided, PRD §10.4)

---

## Tergantung keputusan terbuka PRD §10 (selesaikan sebelum Sprint 4.2 mulai)
| # | Keputusan | Blocking untuk |
|---|---|---|
| 1 | Port 20128 + `--port` | Sprint 1.0 |
| 3 | Nama binary `jkrouter` | Sprint 1.4, 4.6 |
| 5 | Dashboard login ON/OFF default | Sprint 4.2.8 |
| 6 | i18n scope | Sprint 5.6 |
| 4 | pxpipe/skills skip? | Tidak blocking (P2 anyway) |

## Estimat total (1 dev, termasuk buffer)
- **Sprint 1–4 (MVP P1 penuh)**: ~4–5 minggu
- Sprint 5: ~1–2 minggu
- Item di luar sprint: on-demand

## Anti-goal checklist (ingat sebelum mulai tiap task)
- [ ] Tidak ada ORM/gRPC/message queue (PRD §3 "Sengaja TIDAK dipakai")
- [ ] Tidak ada worker pool untuk SQLite write (1 goroutine, §4.1)
- [ ] Tidak copy-paste perilaku 9Router yang belum diverifikasi di source-nya — grep dulu, baru port (risiko PRD §8)
- [ ] Failover mid-stream = TIDAK BOLA (§4.1 point-of-no-return)
