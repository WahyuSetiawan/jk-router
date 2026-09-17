# JKRouter Makefile
GO     := /nix/store/i77g9dmcd399rmxk8688qfr4g2wzgk37-go-1.26.7/bin/go
GOROOT := /nix/store/i77g9dmcd399rmxk8688qfr4g2wzgk37-go-1.26.7/share/go
BIN    := /tmp/jkrouter
CMD    := ./jkserver/cmd
DATA   := $(HOME)/.jkrouter
PORT   ?= 20128
BUN    := /nix/store/fs2axbvhmim3y1dwzpf5mh56569h15df-bun-1.3.13/bin/bun

# modernc.org/sqlite is pure-Go, no CGO needed
GOENV = CGO_ENABLED=0 GOROOT=$(GOROOT)

# Load .env if present (overrides defaults above)
-include .env

.PHONY: build test run dev dashboard backup export import clean help build-ui embed-ui deploy

all: build

build:
	COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
	BUILDTIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ)
	env $(GOENV) $(GO) build -ldflags="-s -w -X main.Version=0.3.0 -X main.GitCommit=$$COMMIT -X main.BuildTime=$$BUILDTIME" -o $(BIN) $(CMD)
	@echo "✓ $(BIN) built"

# ─── Frontend (Nuxt) ───────────────────────────────────────────────────

build-ui:
	@echo "🛠  Building Nuxt UI..."
	cd web && $(BUN) run build
	@echo "✓ web/.output/ ready"

embed-ui:
	@echo "📦 Embedding UI into jkserver/cmd/..."
	rm -rf jkserver/cmd/assets jkserver/cmd/_nuxt jkserver/cmd/favicon.ico
	cp -r web/.output/public/* jkserver/cmd/
	@echo "✓ UI embedded"

# Build UI + embed + rebuild Go binary (full deploy pipeline)
deploy: build-ui embed-ui build
	@echo "✅ Deployed with embedded UI"

# ─── Development ───────────────────────────────────────────────────────

watch-ui:
	@echo "👀 Watching Nuxt changes..."
	cd web && $(BUN) run dev

test:
	env $(GOENV) $(GO) test ./... -count=1

run: build
	$(BIN) serve --port $(PORT) --data-dir $(DATA)

dev: build
	@./dev.sh

dashboard: build
	$(BIN) dashboard --port $(PORT)

backup: build
	$(BIN) backup --data-dir $(DATA)

export: build
	$(BIN) export --data-dir $(DATA)

import: build
	$(BIN) import $(file) --data-dir $(DATA)

clean:
	rm -f $(BIN)
	env $(GOENV) $(GO) clean -cache

help:
	@echo "JKRouter — AI Routing Gateway"
	@echo ""
	@echo "  Go / Server"
	@echo "    make build       Build binary to /tmp/jkrouter"
	@echo "    make test        Run all tests"
	@echo "    make run         Build and run on :$(PORT)"
	@echo "    make dev         Build + run backend, then start Nuxt dev (auto-wait)"
	@echo "    make dashboard   Open browser to dashboard"
	@echo "    make deploy      Build UI + embed + rebuild binary (full release)"
	@echo ""
	@echo "  Frontend (Nuxt)"
	@echo "    make build-ui    Build Nuxt to web/.output/ (via bun)"
	@echo "    make embed-ui    Copy .output → jkserver/cmd/ (embed)"
	@echo "    make watch-ui    Run Nuxt dev server (hot reload)"
	@echo ""
	@echo "  Data / Misc"
	@echo "    make backup      Pre-migration backup"
	@echo "    make export      Export config as JSON"
	@echo "    make import file=config.json  Import config"
	@echo "    make clean       Remove binary + go cache"
