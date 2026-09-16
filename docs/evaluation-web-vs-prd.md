# Evaluasi: web/ vs PRD §5.1 + TASKS §4.2 + Wireframe

**Tanggal:** 2026-09-15  
**Status:** Banyak gap — perlu refactor besar

---

## Gap Ringkasan

| # | Masalah | Severity | Fix |
|---|---------|----------|-----|
| 1 | Tidak ada shared layout dengan sidebar nav | **P1** | Buat `components/Layout.vue` |
| 2 | Halaman `endpoint.vue` missing (P1) | **P1** | Buat baru |
| 3 | Halaman `profile.vue` missing (P1) | **P1** | Buat baru |
| 4 | Tema Catppuccin belum diterapkan di CSS | **P1** | Tambah CSS tokens ke `main.css` |
| 5 | `proxy-pools.vue` field name salah (`label`→`name`, `proxy_list`→`proxy_url`) | **P1** | Fix Vue |
| 6 | `combos.vue` model_ids parse salah (JSON string vs array) | **P1** | Fix Vue |
| 7 | `api-keys.vue` key display salah (backend return `key_hash`, bukan `key`) | **P2** | Fix Vue + backend |
| 8 | `usage.vue` tidak ada chart.js, tidak ada tab Logs | **P2** | Tambah chart.js + tabs |
| 9 | Tidak ada endpoint `/api/dashboard/endpoint` | **P1** | Buat di Go |
| 10 | Tidak ada settings API endpoint (read/write) | **P2** | Buat di Go |
| 11 | Tidak ada password/auth API untuk profile | **P2** | Buat di Go |
| 12 | `chart.js` belum diinstall | **P2** | `pnpm add chart.js` |
| 13 | `connections.vue` redundan — wireframe gabung di Providers | **P3** | Bisa di-hapus atau di-integrate |

---

## Detail Per Halaman

### 1. Providers (`providers.vue`)
- **API shape benar**: grouped by provider, accounts per provider ✓
- **Gap**: CSS pakai `bg-gray-900` bukan Catppuccin token
- **Gap**: Tidak ada state badge (active/cooling/disabled) sesuai wireframe
- **Gap**: Tidak ada proxy pool binding per account
- **Gap**: Tidak ada "Connect OAuth" button

### 2. Connections (`connections.vue`)
- **Redundant**: Wireframe gabung connections (accounts) di dalam Providers page
- **API shape**: Beda field — connections pakai `provider_id` (string) tapi UI tampilkan sebagai angka

### 3. Combos (`combos.vue`)
- **Bug**: `form.model_ids` adalah string (v-model input text), tapi API expect `[]string`. Form kirim `{"model_ids": "gpt-4o,claude"}` → JSON invalid
- **Bug**: Display `c.model_ids?.slice(1,-1)` assume string format `[a,b]` tapi backend kirim JSON array stringified → `["gpt-4o","claude"]` — slice work but wrong approach
- **Gap**: CSS Catppuccin belum

### 4. Proxy Pools (`proxy-pools.vue`)
- **Bug**: Form field `label` → harus `name`
- **Bug**: Form field `proxy_list` → harus `proxy_url`  
- **Bug**: Form field `description` → tidak ada di DB proxy_pools
- **Bug**: Display `p.label` → harus `p.name`
- **Bug**: Display `p.proxy_list` → harus `p.proxy_url`
- **Gap**: Tidak ada health test button
- **Gap**: CSS Catppuccin belum

### 5. Usage (`usage.vue`)
- **Missing**: Chart.js chart (wireframe: bar chart tokens per hari per provider)
- **Missing**: Tab "Logs (live stream)" — perlu SSE endpoint
- **Missing**: Tab "Quota" (P2, skip dulu)
- **Bug**: Stats card menghitung dari raw usage log, bukan aggregate endpoint
- **Gap**: CSS Catppuccin belum

### 6. API Keys (`api-keys.vue`)
- **Bug**: `newKey.value = j.key || ''` — API return format: `{"id":"...","key":"..."}` tapi test show ini work? Check handler...
- **Gap**: Tidak ada kolom "Last used" dan "Requests count" (DB tidak track ini)
- **Gap**: CSS Catppuccin belum

### 7. Settings (`settings.vue`)
- **Bug**: Semua button pakai `alert()` placeholder
- **Missing**: Export/Import config real API call
- **Missing**: Backup/Restore UI
- **Missing**: Resilience defaults UI
- **Missing**: Capacity adapter settings UI
- **Gap**: Tidak ada backend settings endpoint

### 8. Index (`index.vue`)
- **Layout**: Standalone, no sidebar
- **Stats**: Manual compute dari usage log — should use dedicated stats endpoint
- **Missing**: Chart.js overview

### 9. Endpoint — MISSING
- Wireframe §4, P1: Copy-paste endpoint info untuk CLI tools
- Backend: `/api/dashboard/endpoint` belum ada

### 10. Profile — MISSING  
- Wireframe §8, P1: Login / change password
- Backend: `/api/dashboard/auth/*` belum ada

---

## Priority Plan

### Fase 1: Foundation (layout + tema)
1. Tambah Catppuccin CSS tokens ke `assets/main.css`
2. Buat `components/Layout.vue` dengan sidebar nav (8+2 items sesuai wireframe)
3. Wrap semua existing pages dengan Layout

### Fase 2: Critical missing pages
4. Buat `pages/endpoint.vue` (P1)
5. Buat `pages/profile.vue` (P1, minimal: change password form)

### Fase 3: Fix broken forms
6. Fix `proxy-pools.vue`: field names (`label`→`name`, `proxy_list`→`proxy_url`)
7. Fix `combos.vue`: model_ids parsing (JSON stringify on create, parse on display)
8. Fix `api-keys.vue`: key display + copy-to-clipboard

### Fase 4: Enhance existing pages
9. Add chart.js to package.json + usage chart
10. Add state badges to providers page
11. Settings: connect to real backend (remove alerts)

### Fase 5: Backend additions
12. Add `/api/dashboard/endpoint` handler
13. Add `/api/dashboard/settings` handler (GET/PUT)
14. Add `/api/dashboard/auth/change-password` handler
15. Add usage stats summary endpoint (aggregate for 7d)

### Skipping (P2/P3)
- connections.vue → integrate into providers.vue later
- quota tab → P2
- live stream logs → P2 (SSE)
- profile auth flow → basic form first, full bcrypt later
