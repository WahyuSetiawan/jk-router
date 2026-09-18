# Testing Guidelines — JKRouter

Aturan testing untuk `jkserver/` (Go) dan `web/` (Nuxt). Prinsip: **setiap feature baru wajib dibawa bersama testnya**. Feature tanpa test = belum selesai.

## Aturan Umum

1. **Test mengikuti kode, bukan sebaliknya.** Test ditulis bareng PR/commit feature, bukan "nanti".
2. **`make test` hijau adalah syarat commit.** Test merah → tidak boleh commit (kecuali sedang sengaja memfiks, dan jangan di-commit dalam keadaan merah).
3. **Test root-cause, bukan symptom.** Kalau test gagal karena bug di tempat lain, fiks sumbernya — jangan skip test.
4. **Minimal, tapi ada.** Satu test yang benar-benar gagal kalau logika rusak > 10 test yang hanya mengukur coverage.
5. **Jangan tambah dependency test baru** tanpa keperluan mendesak (lihat AGENT.md). Untuk Go: cukup `testing` stdlib. Untuk web: lihat setup Vitest di bawah.

---

## jkserver/ (Go Backend)

### Konvensi

- File test: `*_test.go` **di package yang sama** (`package api`, bukan `package api_test`), letakkan sebelah file yang dites.
- **Table-driven tests** adalah pola default:

```go
func TestRouteFallback(t *testing.T) {
    tests := []struct{
        name  string
        input string
        want  string
    }{
        {"provider utama mati", "bad-key", "fallback"},
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := route(tt.input)
            if got != tt.want {
                t.Errorf("route(%q) = %q, want %q", tt.input, got, tt.want)
            }
        })
    }
}
```

- Pakai `t.TempDir()` untuk filesystem, `httptest` untuk HTTP handler — semua stdlib.
- DB test: pakai SQLite in-memory/temp dir, ikuti pola `internal/db/db_test.go` yang sudah ada.

### Kapan test wajib (checklist feature baru)

| Feature | Test wajib |
|---|---|
| HTTP handler baru (`internal/api/`) | `httptest` happy path + 1 error path (400/404/500) |
| Logika engine/routing/fallback | Unit test pure function-nya, table-driven |
| Executor/translator format | Test terjemahan input→output untuk 1–2 kasus representatif |
| Perubahan DB/schema/migrasi | Test migrasi up dari DB versi sebelumnya |
| Bug fix | Test yang **gagal sebelum fix**, hijau setelah fix (regression test) |

### Menjalankan

```bash
make test                  # semua: go test ./... -count=1
go test ./internal/engine # satu package
go test -run TestNama ./internal/engine
```

---

## web/ (Nuxt Dashboard)

### Setup (sekali saja, belum dilakukan)

Satu-satunya dependency yang diizinkan untuk testing frontend: **Vitest** (sudah jadi standar Nuxt, ringan, tanpa browser).

```bash
cd web
bun add -d vitest @nuxt/test-utils @vue/test-utils
```

`web/package.json` → tambah script:
```json
"test": "vitest run",
"test:watch": "vitest"
```

Buat `web/vitest.config.ts` minimal. **Jangan** setup Playwright/e2e browser dulu — smoke test via Go API test yang sudah ada lebih murah. Tambah e2e hanya kalau komponen interaktif kompleks (chart, form multi-step) terbukti rusak tanpa kedeteksi.

### Tipe test & kapan dipakai

| Tipe | Untuk | Tools |
|---|---|---|
| **Unit composable** | Logika di `composables/` (pure logic, formatting, filter) | Vitest langsung |
| **Component smoke** | Setiap page/komponen UI baru: apakah render tanpa error + elemen kunci ada | `@vue/test-utils` mount |
| **Behavior interaksi** | Klik/toggle yang mengubah state (misal toggle settings RTK) | `trigger` + assert state/emisi |

### Konvensi

- File test: `pages/__tests__/token-saver.test.ts`, `composables/__tests__/useI18n.test.ts` (atau sebelah file, `*.test.ts`).
- Semua fetch/dashboard component butuh data — jangan mock berlebihan. Bungkus fetch jadi composable kalau perlu supaya bisa di-mock satu titik.
- Smoke test UI minimal:

```ts
import { mount } from '@vue/test-utils'
import TokenSaver from '~/pages/token-saver.vue'

it('renders RTK page dengan heading', () => {
  const wrapper = mount(TokenSaver)
  expect(wrapper.text()).toContain('Token Saver')
})
```

### Checklist feature UI baru

1. Page/component baru → **smoke test** (render + 1 assert konten kunci).
2. Ada interaksi (klik, form, toggle) → **1 behavior test** untuk aksi utama.
3. Komponen memanggil API → mock endpoint di level composable/fetch, bukan HTTP global.
4. `bun test` di `web/` hijau sebelum commit.

### UI yang hanya menampilkan data dari API

Test-nya cukup: smoke render + assert bahwa data yang diterima tampil. Jangan mengetes styling/class CSS.

---

## Definisi Selesai (untuk agent & manusia)

Feature/fix dikatakan **selesai** kalau:

- [ ] Kode jalan sesuai spec
- [ ] `make test` hijau (Go)
- [ ] `bun test` hijau (web, kalau ada perubahan UI/composable)
- [ ] Bug fix membawa regression test
- [ ] Tidak ada test yang di-skip (`t.Skip` / `it.skip`) tanpa komentar `// ponytail:` / alasan
