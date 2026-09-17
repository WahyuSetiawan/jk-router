# JKRouter — Remaining Tasks (Status Review 2026-09-17)

> Cross-reference: `docs/PRD.md` §7 acceptance criteria + `docs/TASKS.md` vs real codebase.
> Semua sprint 1–6 sudah di-commit ke master. Task di bawah adalah yang **belum dikerjakan**.

---

## ✅ Sprint 1–6: COMPLETE

| Sprint | Status | Commit Terakhir |
|--------|--------|-----------------|
| Sprint 1 — Core routing | ✅ 100% | (sebelumnya) |
| Sprint 2 — Dashboard API | ✅ 100% | (sebelumnya) |
| Sprint 3 — Failover + cache | ✅ 100% | (sebelumnya) |
| Sprint 4 — Media providers | ✅ 100% | 7bbbbb0 |
| Sprint 5 — Dashboard UI + i18n | ✅ 100% | c996964 |
| Sprint 5.5 — Media providers | ✅ 100% | 7bbbbb0 |
| Sprint 5.6 — Video endpoint | ✅ 100% | (sprint 5.5) |
| Sprint 6 — RTK + deploy | ✅ 100% | 51f6f7d + 99ec5ea |

**Sprint 6 items:**
- 6.1 Binary <15MB → 12MB ✅
- 6.2 RTK filters wired ke engine ✅
- 6.3 token-saver.vue page ✅
- 6.4 Settings RTK toggle aktif ✅

---

## 🔴 PRD §7 Acceptance Criteria — RESOLVED

| # | Kriteria | Status |
|---|----------|--------|
| 7.1 | Binary <15MB | ✅ 12MB |
| 7.2 | Claude Code streaming → Anthropic | ✅ Streaming works, RTK filters applied |
| 7.3 | MCP server | ✅ /v1/api/mcp |
| 7.4 | Dashboard login | ✅ bcrypt |
| 7.5 | i18n ID+EN | ✅ useI18n composable |

---

## ⏸ Open / Deferred (User Decision Needed)

| ID | Item | Catatan |
|----|------|---------|
| 4 | pxpipe/skills skip | PRD §10 #4 — belum ada keputusan user |
| 5.3 | Provider batch berikutnya | Tunggu permintaan user |
| 5.7 | Cloud sync + MITM | P2, terakhir setelah packaging stabil |
| 5.1 | Tray systray | Deferrd — CGO/Wayland risk per PRD §8 |
| P2.15 | Proxy relay deploy | ✅ DONE (baeff0a) — Vercel/Cloudflare/Deno deploy handlers + frontend modal |
| Dashboard Capacity Adapter UI | API sudah ada, belum ada toggle di halaman Settings |

---

## 📊 Current State

- **Binary**: `/tmp/jkr_deploy.bin` (12MB, strip)
- **Port**: 20127 (dari `.env`)
- **DB**: `~/.jkrouter/jkrouter.db`
- **Dashboard**: `http://localhost:20127/dashboard/`
- **API base**: `http://localhost:20127/v1/`
- **MCP**: `POST http://localhost:20127/v1/api/mcp`
- **Tests**: 25/25 green (api, db, engine, rtk, translator, oauth, proxypool, crypto)
