# FaangJobs — build & run helpers
.DEFAULT_GOAL := help
SHELL := /bin/bash
DATA ?= ./data
ADDR ?= :8080

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: web
web: ## Build the React frontend into internal/webui/dist
	cd web && npm install && npm run build

.PHONY: sync-data
sync-data: ## Sync ./data into the server's embedded snapshot (FULL=1 keeps descriptions)
	@go run ./tools/snapshot -src $(DATA) -dst internal/dataset/data $(if $(FULL),-full,)

.PHONY: binaries
binaries: sync-data ## Build both Go binaries into ./bin (server embeds frontend + data snapshot)
	go build -o bin/crawler ./cmd/crawler
	go build -o bin/server  ./cmd/server

.PHONY: build
build: web binaries ## Build frontend + both binaries

.PHONY: crawl
crawl: ## Run the crawler over the full registry
	go run ./cmd/crawler -data $(DATA)

.PHONY: crawl-fast
crawl-fast: ## Crawl only a small subset (greenhouse+ashby+amazon) for a quick demo
	go run ./cmd/crawler -data $(DATA) -only greenhouse,ashby,amazon,netflix,workday

.PHONY: server
server: ## Run the web server (reads $(DATA))
	go run ./cmd/server -data $(DATA) -addr $(ADDR)

.PHONY: dev-web
dev-web: ## Run the Vite dev server (proxies /api to :8080)
	cd web && npm run dev

.PHONY: list
list: ## Print the resolved company catalog
	go run ./cmd/crawler -list

.PHONY: sources
sources: ## Print registered source adapters
	go run ./cmd/crawler -sources

.PHONY: test
test: ## Run Go tests
	go test ./...

.PHONY: clean
clean: ## Remove binaries and crawled data
	rm -rf bin $(DATA)
