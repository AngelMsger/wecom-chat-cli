GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.3.0-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -s -w -X github.com/angelmsger/wecom-chat-cli/pkg/constants.Version=$(VERSION) -X github.com/angelmsger/wecom-chat-cli/pkg/constants.Commit=$(COMMIT) -X github.com/angelmsger/wecom-chat-cli/pkg/constants.BuildTime=$(BUILD_TIME)

.PHONY: build lint test e2e isolation-check docs docs-check site-check cross release-tests vuln
build:
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o bin/wecom-chat-cli ./cmd/wecom-chat-cli
lint:
	test -z "$$($(GO) fmt ./...)"
	$(GO) vet ./...
test:
	$(GO) test -race ./...
e2e:
	$(GO) test -count=1 -tags=e2e ./tests/...
isolation-check:
	$(GO) test -count=1 -tags=isolation ./tests/...
docs:
	$(GO) run ./cmd/gen-docs
docs-check:
	$(GO) run ./cmd/gen-docs --check
site-check:
	python3 scripts/check-site.py
cross:
	VERSION=$(VERSION) COMMIT=$(COMMIT) ./scripts/build.sh
release-tests:
	python3 -m unittest discover -s scripts -p 'test_*.py'
vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
