SHELL := /usr/bin/env bash
GO ?= go

.PHONY: git-hooks
git-hooks:
	git config core.hooksPath .githooks
	@chmod +x .githooks/commit-msg .githooks/pre-commit
	@echo "git hooks enabled (core.hooksPath=.githooks)"

.PHONY: ent-generate
ent-generate:
	$(GO) generate ./internal/ent/...
	
.PHONY: fmt vet test test-race ent-generate check
fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
vet:
	go vet ./...
test:
	go test ./...
test-race:
	go test -race ./...
ent-generate:
	go generate ./internal/ent/...
check: fmt vet test test-race