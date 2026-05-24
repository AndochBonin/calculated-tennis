COVERAGE_THRESHOLD ?= 90
SHELL := /bin/zsh

# Build linux/amd64 binary and upload to S3 (after terraform apply).
# Required env: S3_ARTIFACT_BUCKET, S3_ARTIFACT_KEY (terraform outputs artifact_bucket, artifact_key).
# Optional env: AWS_REGION, AWS_PROFILE (passed to aws s3 cp when set).
# Optional make var: PUSH_BINARY_OUT (local path, default dist/polymarket-linux-amd64).
#
# Example:
#   S3_ARTIFACT_BUCKET="$$(terraform -chdir=infra/terraform output -raw artifact_bucket)" \
#   S3_ARTIFACT_KEY="$$(terraform -chdir=infra/terraform output -raw artifact_key)" \
#   AWS_REGION=eu-west-1 \
#   make push-binary
PUSH_BINARY_OUT ?= dist/polymarket-linux-amd64

.PHONY: push-binary
push-binary:
	@if [ -z "$$S3_ARTIFACT_BUCKET" ] || [ -z "$$S3_ARTIFACT_KEY" ]; then \
		echo "S3_ARTIFACT_BUCKET and S3_ARTIFACT_KEY are required (see terraform outputs artifact_bucket, artifact_key)"; \
		exit 2; \
	fi
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(PUSH_BINARY_OUT) .
	@set -e; \
	aws_args=(); \
	[ -n "$${AWS_REGION:-}" ] && aws_args+=(--region "$$AWS_REGION"); \
	[ -n "$${AWS_PROFILE:-}" ] && aws_args+=(--profile "$$AWS_PROFILE"); \
	aws s3 cp "$(PUSH_BINARY_OUT)" "s3://$$S3_ARTIFACT_BUCKET/$$S3_ARTIFACT_KEY" "$${aws_args[@]}"

run:
	go run .

up:
	docker compose up -d --build

down:
	docker compose down

# Seed LocalStack secret from gitignored secret.json (create once per dev machine).
localstack-init-secret:
	@if [ ! -f secret.json ]; then \
		echo "secret.json not found; create it from docs/aws-secrets-manager.md"; \
		exit 2; \
	fi
	AWS_ENDPOINT_URL=http://localhost:4566 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	AWS_REGION=eu-west-1 \
	AWS_DEFAULT_REGION=eu-west-1 \
	aws secretsmanager create-secret \
		--name polymarket/dev \
		--secret-string file://secret.json \
	|| AWS_ENDPOINT_URL=http://localhost:4566 \
	AWS_ACCESS_KEY_ID=test \
	AWS_SECRET_ACCESS_KEY=test \
	AWS_REGION=eu-west-1 \
	AWS_DEFAULT_REGION=eu-west-1 \
	aws secretsmanager put-secret-value \
		--secret-id polymarket/dev \
		--secret-string file://secret.json

# Live CLOB probe: POLYMARKET_API_KEY, POLYMARKET_API_SECRET, POLYMARKET_PASSPHRASE,
# POLYMARKET_ADDRESS (same EOA as METAMASK_KEY in generate_creds).
# POLYMARKET_USER_ADDRESS (?user= for GET /positions on data API).
# Optional: POLYMARKET_DATA_API_BASE_URL (defaults to https://data-api.polymarket.com).
# Optional: POLYMARKET_CLOB_SERVER_TIME=true.
live-clob:
	go run ./cmd/liveclob

# Place a 1-share BUY GTC limit order on the live CLOB (real funds / real API).
# Same env as live-clob, plus a private key for EIP-712 signing:
# POLYMARKET_PRIVATE_KEY (preferred) or METAMASK_KEY.
# Usage: make place-order
# Optional: make place-order PRICE=0.50 TOKEN=<token_id>
place-order:
	go run ./cmd/placeorder $(if $(PRICE),-price=$(PRICE),) $(TOKEN)

# Tennis Abstract player stats (live fetch; optional Redis cache via REDIS_ADDR / REDIS_URL).
# Usage: make get-stats
# Optional: make get-stats PLAYER="jannik sinner"
get-stats:
	go run ./cmd/getstats $(if $(PLAYER),-player="$(PLAYER)",)

venv:
	@source .venv/bin/activate; $$SHELL

creds:
	python3 cmd/python/generate_creds.py

deploy-wallet:
	python3 cmd/python/deploy_wallet.py

test:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out
	go tool cover -html=coverage.out
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print substr($$3, 1, length($$3)-1)}'); \
    echo "Total coverage: $$total%"; \
	if awk "BEGIN {exit !($$total >= $(COVERAGE_THRESHOLD))}"; then \
		echo "Coverage check passed (>= $(COVERAGE_THRESHOLD)%)"; \
	else \
		echo "Coverage check failed (< $(COVERAGE_THRESHOLD)%)"; \
		exit 1; \
	fi
