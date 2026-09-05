SHELL := /usr/bin/env bash

BIN_DIR	        := bin
GO 							?= go
ATLAS          	?= atlas
COMPOSE_FILE   	:= compose.yml
COMPOSE        	?= docker compose -f $(COMPOSE_FILE)
IMAGE_REGISTRY 	?= rajabinekoo
IMAGE_TAG      	?= dev

export POSTGRES_DSN ?= postgres://sigryx:sigryx@localhost:5432/sigryx?sslmode=disable
export POSTGRES_SCHEMA ?= vault

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
	@$(call dotenv) $(ATLAS) migrate apply --env runtime --var schema_name="$${POSTGRES_SCHEMA:-vault}"

.PHONY: migrate-lint
migrate-lint:
	@$(call dotenv) $(ATLAS) migrate lint --env local --latest 1

.PHONY: migrate-status
migrate-status:
	@$(call dotenv) $(ATLAS) migrate status --env runtime --var schema_name="$${POSTGRES_SCHEMA:-vault}"

.PHONY: migrate-hash
migrate-hash:
	@$(call dotenv) $(ATLAS) migrate hash --dir file://migrations


.PHONY: run-server
run-server:
	@$(call dotenv,indexer) $(GO) run ./cmd

.PHONY: image
image:
	docker build -f Dockerfile -t $(IMAGE_REGISTRY)/sigryx:$(IMAGE_TAG) .

.PHONY: compose-up compose-down compose-reset compose-logs compose-ps
compose-up:
	$(COMPOSE) up --build -d

compose-down:
	$(COMPOSE) down

compose-reset:
	$(COMPOSE) down -v --remove-orphans

compose-logs:
	$(COMPOSE) logs -f sigryx

compose-ps:
	$(COMPOSE) ps

# Backward-compatible aliases.
.PHONY: infra-up infra-down infra-logs
infra-up: compose-up
infra-down: compose-down
infra-logs: compose-logs

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
