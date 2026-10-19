.PHONY: vet test cover cover-html build build-linux build-windows sonar clean

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

# Build for the current platform
build:
	go build -o ipamthing .

# Cross-compile for Linux amd64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing-linux-amd64 .

# Cross-compile for Windows amd64
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o ipamthing.exe .

# Run a SonarQube Cloud analysis locally (requires sonar-scanner on PATH and
# SONAR_TOKEN exported). CI does this on every push and PR.
sonar: cover
	sonar-scanner

clean:
	rm -f ipamthing ipamthing-linux-amd64 ipamthing-linux-arm64 ipamthing.exe coverage.out
