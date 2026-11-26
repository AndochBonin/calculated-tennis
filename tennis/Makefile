SHELL := /bin/zsh

.PHONY: wasm web build test

# The Go engine compiled for the browser, plus Go's matching JS loader.
wasm:
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o web/public/tennis.wasm ./cmd/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" web/public/

# Dev server with hot reload (rerun `make wasm` after Go changes).
web: wasm
	cd web && npm ci && npm run dev

# Static site in web/dist.
build: wasm
	cd web && npm ci && npm run build

test:
	go test ./...
	cd web && npm test
