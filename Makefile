.PHONY: help build run seed dev-web generate-css generate-templ generate-proto test clean lint check-nolint install deploy

BINARY_NAME=relay-authz
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X 'main.version=${VERSION}' -X 'main.commit=${COMMIT}' -X 'main.date=${DATE}'"

help:
	@echo "relay.nostr.io — Authenticated Nostr Relay"
	@echo ""
	@echo "  make build           - Build the binary"
	@echo "  make run             - Build and run locally (dev config + seed admins)"
	@echo "  make seed            - Seed admin npubs from configs/seed-admins.toml"
	@echo "  make dev-web         - Run with live reload (air + templ + tailwind)"
	@echo "  make generate-templ  - Generate Go code from templ templates"
	@echo "  make generate-css    - Generate Tailwind CSS"
	@echo "  make generate-proto  - Generate Go code from proto files"
	@echo "  make test            - Run tests"
	@echo "  make lint            - Run linter"
	@echo "  make clean           - Clean build artifacts"
	@echo "  make install         - Install binary to GOPATH/bin"
	@echo "  make deploy          - Deploy to production"

build: generate-templ generate-css
	CGO_ENABLED=0 nix develop -c go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/${BINARY_NAME}

run: build
	@mkdir -p tmp
	./bin/${BINARY_NAME} --config configs/dev.toml --seed configs/seed-admins.toml

seed: build
	@mkdir -p tmp
	./bin/${BINARY_NAME} --config configs/dev.toml --seed configs/seed-admins.toml &
	@sleep 1 && kill $$!
	@echo "Admin npubs seeded into tmp/relay-authz.db"

dev-web:
	@mkdir -p tmp
	@echo "Starting development servers..."
	@make -j3 dev-air dev-templ dev-tailwind

dev-air:
	nix develop -c air

dev-templ:
	nix develop -c templ generate --watch --proxy=http://localhost:8090 --open-browser=false

dev-tailwind:
	nix develop -c sh -c 'cat $$BTK_THEME_PATH static/css/custom.css > static/css/input.css && tailwindcss -i static/css/input.css -o static/css/output.css --watch'

generate-templ:
	nix develop -c templ generate

generate-css:
	nix develop -c sh -c 'cat $$BTK_THEME_PATH static/css/custom.css > static/css/input.css && tailwindcss -i static/css/input.css -o static/css/output.css --minify'

generate-proto:
	nix develop -c protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/nauthz.proto

test:
	nix develop -c go test -v -race -failfast ./...

check-nolint:
	@if grep -rn '//nolint' --include='*.go' --exclude='*_test.go' . 2>/dev/null; then \
		echo "ERROR: nolint directives forbidden in production code"; \
		exit 1; \
	fi

lint: check-nolint
	nix develop -c golangci-lint run --max-issues-per-linter=1 --max-same-issues=1

clean:
	rm -rf bin/ dist/ tmp/ views/*_templ.go views/components/*_templ.go static/css/output.css static/css/input.css

install: build
	nix develop -c go install ${LDFLAGS} ./cmd/${BINARY_NAME}

deploy:
	bash deploy.sh
