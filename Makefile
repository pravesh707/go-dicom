# godicom — common developer tasks
GO ?= go
PKGS := ./...
VERSION := $(shell sed -nE 's/.*Version = "([0-9]+\.[0-9]+\.[0-9]+)".*/\1/p' version.go)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
LDFLAGS := -X main.commit=$(COMMIT)

.PHONY: all build test race vet fmt fmt-check lint tidy cover examples version clean

all: fmt-check vet test

version:
	@echo "godicom v$(VERSION) ($(COMMIT))"

build:
	$(GO) build $(PKGS)

test:
	$(GO) test $(PKGS)

race:
	$(GO) test -race $(PKGS)

vet:
	$(GO) vet $(PKGS)

fmt:
	$(GO) fmt $(PKGS)

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

# Requires golangci-lint (optional): https://golangci-lint.run
lint:
	golangci-lint run

tidy:
	$(GO) mod tidy

cover:
	$(GO) test -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -func=coverage.out

# Build the command-line tools into ./bin (with the git commit embedded)
examples:
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/echoscu ./cmd/echoscu
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/echoscp ./cmd/echoscp

clean:
	rm -rf bin coverage.out
