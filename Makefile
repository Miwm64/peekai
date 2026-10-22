VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin

.PHONY: build test lint app clean

## build: binary for this machine in bin/
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN)/peekai ./cmd/peekai

## test: unit tests with the race detector
test:
	go test -race ./...

## lint: same linter version as CI
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 run

## app: macOS universal peekai.app in bin/ (Apple Silicon + Intel)
app:
	CGO_ENABLED=1 GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/peekai-arm64 ./cmd/peekai
	CGO_ENABLED=1 GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(BIN)/peekai-amd64 ./cmd/peekai
	lipo -create -output $(BIN)/peekai-universal $(BIN)/peekai-arm64 $(BIN)/peekai-amd64
	scripts/macos-app.sh $(BIN)/peekai-universal $(VERSION) $(BIN)

clean:
	rm -rf $(BIN)
