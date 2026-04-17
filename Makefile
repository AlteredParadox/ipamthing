.PHONY: test cover cover-html build build-linux build-windows sonar clean

# Run all tests
test:
	go test ./...

# Run tests with coverage report (used by SonarQube)
cover:
	go test -coverprofile=coverage.out ./...
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

# Run SonarQube analysis (requires sonar-scanner on PATH and coverage.out)
sonar: cover
	sonar-scanner

clean:
	rm -f ipamthing ipamthing-linux-amd64 ipamthing.exe coverage.out
