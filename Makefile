BINARY := contract-cli
MAIN_PACKAGE := ./cmd/contract-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X cn.qfei/contract-cli/internal/build.Version=$(VERSION) -X cn.qfei/contract-cli/internal/build.Commit=$(COMMIT) -X cn.qfei/contract-cli/internal/build.Date=$(DATE)

.PHONY: test build install release-assets release-check package-dry-run local-install-check release-snapshot clean

test:
	go test ./...

build:
	./build.sh

install:
	go install -ldflags "$(LDFLAGS)" $(MAIN_PACKAGE)

release-assets:
	scripts/build-release-assets.sh

package-dry-run:
	tests/release/package-dry-run.sh

local-install-check:
	tests/release/local-install.sh

release-check: test
	tests/cli_e2e/smoke.sh
	tests/release/package-dry-run.sh
	tests/release/local-install.sh

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist bin $(BINARY)
