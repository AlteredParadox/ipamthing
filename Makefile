BINARY := ipamthing
# Stamped into the binary as main.version. The release workflow overrides
# this with the tag; local builds report the git description, or "dev".
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: vet test cover cover-html build build-linux build-windows notices notices-check license-headers license-check sonar clean

# gofmt -l prints unformatted files; fail if there are any, then go vet.
# CI runs exactly this (see .github/workflows/ci.yml).
vet:
	@unformatted="$$(gofmt -l .)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

# Run all tests
test:
	go test ./...

# Run tests with coverage report (what the SonarQube Cloud scan imports)
cover:
	go test -covermode=atomic -coverprofile=coverage.out ./...
	@echo "--- coverage summary ---"
	@go tool cover -func=coverage.out | tail -1

# Open coverage report in browser
cover-html: cover
	go tool cover -html=coverage.out

# Build for the current platform. THIRD_PARTY_LICENSES.md is embedded into
# the binary (legal.go), so it is regenerated first and what ships always
# matches what is linked in. The committed copy exists so a bare `go build`
# still works.
build: notices
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Cross-compile for Linux amd64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY)-linux-amd64 .

# Cross-compile for Windows amd64
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY).exe .

# Regenerate the third-party notices from the modules actually linked in.
notices:
	@./scripts/gen-third-party-licenses.sh >/dev/null

# Fail if the committed notices are stale. Deliberately NOT part of the CI
# gate: Dependabot bumps go.mod but cannot regenerate this file, so gating
# every PR on it would make each gomod PR red for a reason the bot can't fix.
# The release workflow runs it instead — that is the point where the notice
# legally has to match the artifact being published.
notices-check:
	@tmp=$$(mktemp); \
	./scripts/gen-third-party-licenses.sh "$$tmp" >/dev/null; \
	if ! diff -q "$$tmp" THIRD_PARTY_LICENSES.md >/dev/null; then \
		echo "FAIL: THIRD_PARTY_LICENSES.md is stale — run 'make notices'"; \
		diff -u THIRD_PARTY_LICENSES.md "$$tmp" | head -40; \
		rm -f "$$tmp"; exit 1; \
	fi; \
	rm -f "$$tmp"; \
	echo "notices-check: THIRD_PARTY_LICENSES.md is current"

# Stamp the AGPL notice on any first-party source missing it.
license-headers:
	@./scripts/license-headers.sh

# Cheap and deterministic (no network, no build), so this one DOES gate CI.
license-check:
	@./scripts/license-headers.sh --check

# Run a SonarQube Cloud analysis locally (requires sonar-scanner on PATH and
# SONAR_TOKEN exported). CI does this on every push and PR.
sonar: cover
	sonar-scanner

clean:
	rm -f $(BINARY) $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 $(BINARY).exe coverage.out
