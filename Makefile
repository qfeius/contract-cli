BINARY := contract-cli
MAIN_PACKAGE := ./cmd/contract-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X cn.qfei/contract-cli/internal/build.Version=$(VERSION) -X cn.qfei/contract-cli/internal/build.Commit=$(COMMIT) -X cn.qfei/contract-cli/internal/build.Date=$(DATE)

.PHONY: test build install release-snapshot clean

test:
	go test ./...

build:
	./build.sh

install:
	go install -ldflags "$(LDFLAGS)" $(MAIN_PACKAGE)

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist bin $(BINARY)
