.PHONY: build dev test test-integration lint ui ui-watch docker release clean

BINARY      := bin/dnsmon
MODULE      := github.com/t0mer/dnsmon
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS     := -s -w \
               -X $(MODULE)/internal/version.Version=$(VERSION) \
               -X $(MODULE)/internal/version.Commit=$(COMMIT) \
               -X $(MODULE)/internal/version.Date=$(DATE)

build: ui
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/dnsmon

dev:
	@which air > /dev/null || go install github.com/air-verse/air@latest
	air -c .air.toml

test:
	go test -race -count=1 ./...

test-integration:
	./scripts/test-integration.sh

lint:
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin
	golangci-lint run

ui:
	cd web && npm ci && npx tailwindcss -i src/css/tailwind.src.css -o dist/css/app.css --minify
	mkdir -p web/dist/js
	cp web/src/js/*.js web/dist/js/
	cp web/src/*.html web/dist/

ui-watch:
	cd web && npx tailwindcss -i src/css/tailwind.src.css -o dist/css/app.css --watch

docker:
	docker build -t dnsmon:latest .

docker-compose-up:
	docker compose up -d

release:
	BUILD_MODE=prod VERSION=$(VERSION) bash scripts/build.sh

clean:
	rm -rf $(BINARY) web/dist/css/app.css web/dist/*.html web/dist/js
