# Plan — Penutupan Gap Testing (Audit 2026-09-18)

> Sumber: `docs/issues/audit-testing-2026-09-18.md`. Petunjuk teknis: `docs/guidelines/testing.md`.
> Estimasi total: **~3 jam** (Go 2 jam + Vitest 30m + SSE integration 30m).

---

## Phase 0 — Setup Dasar (30m)

### T0.1 `[BE]` Install Vitest untuk web/
- **Owner**: agent
- **Steps**:
  1. `cd web && bun add -d vitest @nuxt/test-utils @vue/test-utils`
  2. Buat `web/vitest.config.ts` (minimal, gunakan preset Nuxt)
  3. Tambah script `"test": "vitest run"` di `web/package.json`
  4. Verifikasi: `cd web && bun test` → no tests yet, tapi framework jalan.
- **Done**: `bun test` exit 0 (kosong tapi tidak error).

---

## Phase 1 — Critical Gaps (PRD §7 Acceptance Criteria) (75m)

### T1.1 `[BE]` Auth & RequireAuth middleware — `jkserver/internal/api/auth_test.go`
- **Owner**: agent
- **Target**: 4-5 test
- **Test cases**:
  1. `POST /api/dashboard/login` dengan password salah → 401
  2. `POST /api/dashboard/login` dengan password benar → 200 + Set-Cookie session
  3. `GET /api/dashboard/providers` tanpa cookie → 401 (RequireAuth block)
  4. `GET /api/dashboard/providers` dengan cookie valid → 200
- **File baru**: `jkserver/internal/api/auth_test.go`
- **Done**: `go test ./internal/api/ -run TestAuth -v` hijau.

### T1.2 `[BE]` Backup & restore DB — `jkserver/internal/db/backup_test.go`
- **Owner**: agent
- **Target**: 3-4 test
- **Test cases**:
  1. `BackupBeforeMigration` buat file .bak.gz di temp dir
  2. Restore dari file backup → DB berisi data yang sama
  3. Retensi: setelah 10 backup, yang tertua dihapus
- **File baru**: `jkserver/internal/db/backup_test.go`
- **Done**: `go test ./internal/db/ -run TestBackup -v` hijau.

### T1.3 `[BE]` Export/import config JSON — `jkserver/cmd/subcommands_test.go`
- **Owner**: agent
- **Target**: 3 test
- **Test cases**:
  1. `ExportConfig` → JSON valid, field `providers/connections/combos/api_keys/proxy_pools` ada
  2. `ImportConfig` → import JSON ke DB kosong, lalu verify data masuk
  3. Round-trip: export → import → export lagi → hasil identik
- **File baru**: `jkserver/cmd/subcommands_test.go`
- **Done**: `go test ./cmd/ -run TestExportImport -v` hijau.

---

## Phase 2 — Important Gaps (50m)

### T2.1 `[BE]` Dashboard CRUD endpoints — update `jkserver/internal/api/dashboard_test.go`
- **Owner**: agent
- **Target**: tambah 4-5 test case ke existing suite
- **Test cases** (tambah di `TestDashboardEndpoints` atau separate):
  1. `DELETE /api/dashboard/providers/:id` → 200, list providers berkurang
  2. `PUT /api/dashboard/connections/:id` (update proxy_pool_id) → 200
  3. `DELETE /api/dashboard/proxy-pools/:id` → 200
  4. `POST /api/dashboard/providers` dengan body kosong → 400 (validasi)
- **File**: edit `jkserver/internal/api/dashboard_test.go` (tambah, bukan ganti)
- **Done**: `go test ./internal/api/ -run TestDashboardEndpoints -v` tetap hijau +新增用例.

### T2.2 `[BE]` SSE streaming integration — `jkserver/internal/engine/sse_test.go` (baru)
- **Owner**: agent
- **Target**: 3 test
- **Test cases**:
  1. Streaming response: chunk pertama dikirim → flusher aktif
  2. Non-streaming: response body lengkap
  3. Mid-stream error: chunk pertama terkirim, error event emit, koneksi tutup
- **File baru**: `jkserver/internal/engine/sse_test.go`
- **Done**: `go test ./internal/engine/ -run TestSSE -v` hijau.

### T2.3 `[BE]` MCP server smoke — `jkserver/internal/mcp/server_test.go`
- **Owner**: agent
- **Target**: 2-3 test
- **Test cases**:
  1. MCP endpoint `/api/mcp` reachable (HTTP 200)
  2. MCP initialize request → valid JSON-RPC response
- **File baru**: `jkserver/internal/mcp/server_test.go`
- **Done**: `go test ./internal/mcp/ -v` hijau.

### T2.4 `[BE]` Media endpoints smoke — `jkserver/internal/media/routes_test.go`
- **Owner**: agent
- **Target**: 3-4 test
- **Test cases**:
  1. `/v1/audio/speech` → 401 (unauthorized) atau 422 (invalid body)
  2. `/v1/images/generations` → 401
  3. Provider tidak dikenal → 404
- **File baru**: `jkserver/internal/media/routes_test.go`
- **Done**: `go test ./internal/media/ -v` hijau.

---

## Phase 3 — Web Dashboard Smoke Tests (30m)

### T3.1 `[FE]` Vitest setup + smoke test `providers.vue`
- **Owner**: agent
- **Steps**:
  1. Buat `web/pages/__tests__/providers.test.ts`
  2. Mount component, assert heading "Providers" atau table ada
  3. Commit: `bun test` hijau
- **Done**: `cd web && bun test` hijau + 1 passing test.

### T3.2 `[FE]` Smoke test halaman P1 lainnya
- **Owner**: agent
- **Files**:
  - `web/pages/__tests__/combos.test.ts`
  - `web/pages/__tests__/settings.test.ts`
  - `web/pages/__tests__/usage.test.ts`
  - `web/pages/__tests__/keys.test.ts`
  - `web/composables/__tests__/useI18n.test.ts`
- **Pattern**: sama — mount + assert konten kunci ada
- **Done**: `cd web && bun test` → 5 tests passing.

---

## Phase 4 — Cleanup & Validation (15m)

### T4.1 Validasi akhir
- **Owner**: agent
- **Steps**:
  1. `make test` → semua Go test hijau (≥55 test)
  2. `cd web && bun test` → semua Vitest test hijau (≥6 tests)
  3. Update `docs/issues/audit-testing-2026-09-18.md` dengan status "DONE"
- **Done**: `make test` + `bun test` keduanya hijau.

---

## Ringkasan

| Phase | Tasks | Est. | Output |
|---|---|---|---|
| 0 — Setup | T0.1 | 30m | Vitest ready di web/ |
| 1 — Critical | T1.1–1.3 | 75m | 3 file test baru (auth, backup, export/import) |
| 2 — Important | T2.1–2.4 | 50m | 1 file ditambah, 3 file baru |
| 3 — Web | T3.1–3.2 | 30m | 6 smoke test Vue |
| 4 — Cleanup | T4.1 | 15m | Audit report update |
| **Total** | **12 tasks** | **~3 jam** | **~20 test baru** |

**Urutan eksekusi**: Phase 0 → 1 → 2 → 3 → 4 (sequential, setiap phase harus hijau sebelum lanjut).
