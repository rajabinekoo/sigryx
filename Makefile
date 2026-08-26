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