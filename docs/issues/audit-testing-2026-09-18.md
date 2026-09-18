# Audit Testing — JKRouter (per PRD §7)

**Tanggal**: 2026-09-18  
**Target**: cek setiap fitur PRD §5 — sudah/punya test? Kualitas?

## Ringkasan

| Area | Test ada? | Catatan |
|---|---|---|
| Go backend | ✅ Ada (13 file, ~45 test) | Fokus unit test; integrasi tipis |
| Web dashboard | ❌ Kosong | Vitest belum di-install, 0 file test |
| **Coverage PRD §7** | ⚠️ ~60% | Critical gaps di auth, backup/restore, UI |

---

## Test yang Sudah Ada (Go)

### `jkserver/internal/api/dashboard_test.go` — 1 test
- **`TestDashboardEndpoints`**: GET smoke `providers, connections, combos, proxy-pools, api-keys` dengan fake cookie `session=test`.
- **Kekurangan**: Tidak ada POST/PUT/DELETE; cookie auth palsu (`hash=test` bukan bcrypt); tidak test error case (401/404/400); hanya 1 endpoint suite.

### `jkserver/internal/crypto/keys_test.go` — 2 test
- Encrypt/decrypt roundtrip + wrong-key fail. ✅ P1 credential mgmt.

### `jkserver/internal/db/db_test.go` — 3 test
- Writer correctness, concurrency non-blocking, migration runs. ✅ P1 schema.
- `deadlock_test.go` (+2): deadlock + usage stats query. ✅ Usage tracking.
- `migrate_debug_test.go` (+1): util debug — bukan regression test.

### `jkserver/internal/engine/account_test.go` — 6 test
- Store ops, cooling down, strike breaker escalation, filter disabled, combo build candidates, strip history context. ✅ P1 account states + §4.2 capability.

### `jkserver/internal/engine/combo_test.go` — 6 test
- Detect vision/tool capability, sort models, has capability checks. ✅ P1 §4.2.

### `jkserver/internal/engine/routing_test.go` — 4 test
- Combo fallback, capability filtering reject, proxy pool transport, no-candidates-all-disabled. ✅ P1 routing + proxy pool integration.

### `jkserver/internal/oauth/state_test.go` — 3 test
- Create/consume state, unknown state nil, expired state nil. ✅ P1 OAuth flow dasar.

### `jkserver/internal/providers/refresh/manager_test.go` — 2 test
- Adds new models, skips provider without validateUrl. ✅ P1 model auto-refresh.

### `jkserver/internal/proxypool/pool_test.go` — 4 test
- CRUD, update status on unhealthy, extract host/port. ✅ P1 proxy pools CRUD.

### `jkserver/internal/rtk/filters_test.go` — 5 test
- Caveman, ponytail, headroom, system inject, no-op when disabled. ✅ P2 RTK token saver.

### `jkserver/internal/translator/translator_test.go` — 6 test
- OpenAI↔Claude roundtrip, with image, with tool use, required capabilities, model caps, translate response identity. ✅ P1 translation + §4.2 capability.

---

## Gap — Fitur PRD tanpa / belum cukup test

### 🔴 Critical (PRD §7 Acceptance Criteria)

| # | Fitur PRD | Status | Rekomendasi |
|---|---|---|---|
| 1 | **Auth endpoint login** `/api/dashboard/login` (bcrypt verify) | ❌ | Test `auth_test.go`: wrong password → 401, correct → 200 + session cookie |
| 2 | **Middleware RequireAuth** — block unauthenticated | ⚠️ Thin | Dashboard test pakai fake cookie — harus test juga unauthorized path |
| 3 | **Backup & restore** (§4.3) | ❌ | Test: buat DB, trigger backup, verifikasi file ada; restore → data sama |
| 4 | **Export/import config** (JSON) | ❌ | Test: export → parse JSON valid → import ke DB kosong → verify same state |
| 5 | **`/v1/chat/completions` end-to-end** (streaming + non-streaming) | ⚠️ Partial | Routing test mock executor, tapi tidak ada integration test dengan chi router + real SSE |
| 6 | **SSE streaming** — backpressure + point-of-no-return | ⚠️ Partial | `routing_test.go` mock executor; perlu test flusher + mid-stream error |
| 7 | **Account state transitions** (429→cooling, 401→disabled) | ✅ | `account_test.go` sudah cover. Tambah: test circuit breaker di routing loop |

### 🟡 Important (P1 features kurang test)

| # | Fitur | Status | Rekomendasi |
|---|---|---|---|
| 8 | **Dashboard CRUD** (POST create provider/connection/combo/pool, DELETE) | ❌ | `dashboard_test.go` hanya GET — tambah endpoint CRUD smoke |
| 9 | **Proxy pool transport** applied ke executor | ⚠️ Partial | `routing_test.go` ada TestProxyPoolTransportApplied tapi hanya mock; perlu integration dengan http.Transport actual |
| 10 | **Capability adapter pool** (capacity adapter round-robin/fallback) | ⚠️ Partial | Engine test ada, tapi tidak test capacity adapter selection logic secara eksplisit |
| 11 | **RTK filters** injected ke routing pipeline | ✅ (unit) | Filter test ada; tambah test:请求带 RTK enabled → body dimodify sebelum kirim ke executor |
| 12 | **Usage logging** — write + read back | ✅ (unit) | `db_test.go` writer, `deadlock_test.go` usage stats. Tapi tidak ada test: log entry benar setelah request via handler |
| 13 | **MCP server** (`/api/mcp`) | ❌ | 0 test — cek `internal/mcp/` kosong test |
| 14 | **Media endpoints** (TTS/STT/Image) | ❌ | `internal/media/routes.go` ada function test tapi tidak di test file terpisah |

### 🟢 Web Dashboard (Nuxt) — 0 test sama sekali

| # | Halaman/fitur | Rekomendasi |
|---|---|---|
| 15 | `pages/providers.vue` | Smoke: render + elements providers table ada |
| 16 | `pages/combos.vue` | Smoke: render + combo list |
| 17 | `pages/settings.vue` | Smoke: render + backup/restore buttons |
| 18 | `pages/token-saver.vue` | Smoke + toggle RTK filters interaction |
| 19 | `composables/useI18n.ts` | Unit: translation key lookup |

**Setup yang dibutuhkan**: `bun add -d vitest @nuxt/test-utils @vue/test-utils` — sesuai guidelines `docs/guidelines/testing.md`.

---

## Kesimpulan

1. **Core engine + translator + RTK** sudah ditest dengan baik (unit test).
2. **Auth & dashboard API** — weak: hanya 1 test file, hanya GET, fake auth.
3. **Backup/restore, export/import** — belum ada test sama sekali (PRD §7 acceptance criteria).
4. **MCP + Media** — nol test.
5. **Web/Nuxt** — nol test. Vitest belum di-setup.
6. **SSE streaming integration** — perlu test end-to-end dengan chi router + mock upstream.

## Prioritas Fix (menurut PRD §7)

1. 🔴 **Auth + middleware RequireAuth** — acceptance criteria "dashboard login" harus testable.
2. 🔴 **Backup/restore** — acceptance criteria eksplisit.
3. 🔴 **Export/import config** — acceptance criteria eksplisit.
4. 🟡 **Dashboard CRUD endpoints** — POST/PUT/DELETE smoke.
5. 🟡 **MCP + Media** — minimal smoke test.
6. 🟢 **Web Vitest setup** — mulai dengan 1 smoke test per halaman P1.
