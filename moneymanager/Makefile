SHELL := /bin/zsh

PROTO_DIR := proto
GEN_DIR := gen

.PHONY: proto check-balance run test

# Generate Go stubs from proto/moneymanager/v1/service.proto.
# Install once:
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
# Ensure $(go env GOPATH)/bin is on PATH.
proto:
	@command -v protoc >/dev/null 2>&1 || { \
		echo "protoc not found; install from https://grpc.io/docs/protoc-installation/"; \
		exit 1; \
	}
	@command -v protoc-gen-go >/dev/null 2>&1 || { \
		echo "protoc-gen-go not found; run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; \
		exit 1; \
	}
	@command -v protoc-gen-go-grpc >/dev/null 2>&1 || { \
		echo "protoc-gen-go-grpc not found; run: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"; \
		exit 1; \
	}
	@mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		moneymanager/v1/service.proto

check-balance:
	go run ./cmd/checkbalance

run:
	go run ./cmd/server

test:
	go test ./...
