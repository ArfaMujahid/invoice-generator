# Makefile for the Invoice Generator & Tracker.
# Targets wrap the tooling required by CODING-STANDARDS.md §1.

BINARY := bin/invoice
PKG    := ./...

.PHONY: all build run test race vet fmt fmtcheck lint tidy vuln check clean

all: check build

## build: compile the single application binary.
build:
	go build -o $(BINARY) ./cmd/invoice

## run: build and run the server (override flags via ARGS="-dev -addr :9090").
run:
	go run ./cmd/invoice $(ARGS)

## test: run all tests.
test:
	go test $(PKG)

## race: run tests with the data-race detector (§4, §8).
race:
	go test -race $(PKG)

## vet: run go vet.
vet:
	go vet $(PKG)

## fmt: format the codebase in place.
fmt:
	gofmt -w .

## fmtcheck: fail if any file is not gofmt-clean.
fmtcheck:
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "Not gofmt-clean:"; echo "$$unformatted"; exit 1; \
	fi

## lint: run golangci-lint (install separately; see README).
lint:
	golangci-lint run $(PKG)

## tidy: ensure go.mod/go.sum match imports.
tidy:
	go mod tidy

## vuln: scan for known vulnerabilities in called code paths (§13).
vuln:
	govulncheck $(PKG)

## check: the pre-commit gate — format, vet, and race-tested.
check: fmtcheck vet race

## clean: remove build artifacts.
clean:
	rm -rf bin
