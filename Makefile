SHELL := /usr/bin/env bash

BIN_DIR	        := bin
GO 							?= go
ATLAS          	?= atlas
COMPOSE_FILE   	:= deployments/docker-compose.yml
COMPOSE        	?= docker compose -f $(COMPOSE_FILE)
IMAGE_REGISTRY 	?= crypto_payment
IMAGE_TAG      	?= dev

export POSTGRES_DSN ?= postgres://sigryx:sigryx@localhost:5432/sigryx?sslmode=disable

dotenv = set -a; [ -f .env ] && . .env; set +a;

.PHONY: git-hooks
git-hooks:
	git config core.hooksPath .githooks
	@chmod +x .githooks/commit-msg .githooks/pre-commit
	@echo "git hooks enabled (core.hooksPath=.githooks)"

.PHONY: ent-generate
ent-generate:
	$(GO) generate ./internal/ent/...

.PHONY: migrate-diff
migrate-diff:
	@test -n "$(name)" || (echo "name is required, e.g. make migrate-diff name=add_sessions_table" && exit 1)
	@$(call dotenv) $(ATLAS) migrate diff $(name) --env local

.PHONY: migrate-up
migrate-up:
	@$(call dotenv) $(ATLAS) migrate apply --env local

.PHONY: migrate-lint
migrate-lint:
	@$(call dotenv) $(ATLAS) migrate lint --env local --latest 1

.PHONY: migrate-status
migrate-status:
	@$(call dotenv) $(ATLAS) migrate status --env local

.PHONY: migrate-hash
migrate-hash:
	@$(call dotenv) $(ATLAS) migrate hash --env local


.PHONY: run-server
run-server:
	@$(call dotenv,indexer) $(GO) run ./cmd

.PHONY: image
image:
	docker build -f Dockerfile -t $(IMAGE_REGISTRY)/sigryx:$(IMAGE_TAG) .

.PHONY: infra-up
infra-up:
	$(COMPOSE) up -d

.PHONY: infra-down
infra-down:
	$(COMPOSE) down

.PHONY: infra-logs
infra-logs:
	$(COMPOSE) logs -f

.PHONY: fmt vet test test-race ent-generate check
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
vet:
	go vet ./...
test:
	go test ./...
test-race:
	go test -race ./...
check: fmt vet test test-race

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$* ./cmd

.PHONY: clean
clean:
	rm -rf $(BIN_DIR)


DOCS_DIR ?= docs
NPM ?= npm

.PHONY: docs-install docs-dev docs-build

docs-install:
	cd $(DOCS_DIR) && $(NPM) install

docs-dev:
	cd $(DOCS_DIR) && $(NPM) run dev

docs-build:
	cd $(DOCS_DIR) && $(NPM) run build

docs-preview:
	cd $(DOCS_DIR) && $(NPM) run preview
