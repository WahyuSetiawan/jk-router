# JKRouter Makefile
GO     := /nix/store/i77g9dmcd399rmxk8688qfr4g2wzgk37-go-1.26.7/share/go/bin/go
BIN    := /tmp/jkrouter
CMD    := ./jkserver/cmd
DATA   := $(HOME)/.jkrouter
PORT   ?= 20128

# Load .env if present (overrides defaults above)
-include .env

.PHONY: build test run dashboard backup export import clean help

all: build

build:
	$(GO) build -o $(BIN) $(CMD)
	@echo "✓ $(BIN) built"

test:
	$(GO) test ./... -count=1

run: build
	$(BIN) serve --port $(PORT) --data-dir $(DATA)

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
	$(GO) clean -cache

help:
	@echo "JKRouter — AI Routing Gateway"
	@echo ""
	@echo "  make build    Build binary to /tmp/jkrouter"
	@echo "  make test     Run all tests"
	@echo "  make run      Build and run on :$(PORT)"
	@echo "  make dashboard Open browser to dashboard"
	@echo "  make backup   Pre-migration backup"
	@echo "  make export   Export config as JSON"
	@echo "  make import file=config.json  Import config"
	@echo "  make clean    Remove binary + go cache"
