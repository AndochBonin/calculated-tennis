COVERAGE_THRESHOLD ?= 80
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

run:
	go run .

up:
	docker compose up -d --build
	make localstack-init-secret
	docker compose restart polymarket

down:
	docker compose down


live-clob:
	go run ./cmd/liveclob

# Place a 1-share BUY GTC limit order on the live CLOB (real funds / real API).
place-order:
	go run ./cmd/placeorder $(if $(PRICE),-price=$(PRICE),) $(TOKEN)

# Tennis Abstract player stats (live fetch; optional Redis cache via REDIS_ADDR / REDIS_URL).
get-stats:
	go run ./cmd/getstats $(if $(PLAYER),-player="$(PLAYER)",)

# Build calibration cache: slug → 2024 hold/break from matches CSV (optional Redis cache).
fetch-rates:
	go run ./cmd/fetchrates $(if $(MATCHES),-matches=$(MATCHES),) \
		$(if $(OUT),-out=$(OUT),) \
		$(if $(MERGE),-merge=true,)

# Per-player career match lists as JSON (from matches CSV; use MERGE=1 to skip existing files).
fetch-career:
	go run ./cmd/fetchcareer $(if $(MATCHES),-matches=$(MATCHES),) \
		$(if $(DIR),-dir=$(DIR),) \
		$(if $(MERGE),-merge=true,)

# Per-surface alpha grid on 2025 test matches (5000 sims, alpha 1–50). Fetches rates if missing.
calibrate-alpha:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	go run ./cmd/calibratealpha

# Per-surface form grid (alpha=1, recent form). Preloads career data once per surface; use lower -sims for exploration.
# Optional -workers (default NumCPU). Full player coverage: make fetch-career (or set TENNISABSTRACT_CAREER_DIR).
calibrate-form:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	@test -d tennisabstract/testdata/career && ls tennisabstract/testdata/career/*.json >/dev/null 2>&1 || $(MAKE) fetch-career
	go run ./cmd/calibrateform

# Chronological 2025 backtest (interactive on TTY; pass STAKE/MIN_PICK/SIMS to skip prompts).
backtest:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	@test -d tennisabstract/testdata/career && ls tennisabstract/testdata/career/*.json >/dev/null 2>&1 || $(MAKE) fetch-career
	go run ./cmd/backtestbets $(if $(MIN_PICK),-min-pick=$(MIN_PICK),) \
		$(if $(STAKE),-stake=$(STAKE),) \
		$(if $(SIMS),-sims=$(SIMS),)

# Monte Carlo match win projection (Tennis Abstract hold/break; interactive on TTY).
sim:
	go run ./cmd/simmatch $(if $(PLAYER_A),-player-a="$(PLAYER_A)",) \
		$(if $(PLAYER_B),-player-b="$(PLAYER_B)",) \
		$(if $(FORMAT),-format=$(FORMAT),) \
		$(if $(ALPHA),-alpha=$(ALPHA),) \
		$(if $(SIMS),-sims=$(SIMS),) \
		$(if $(SCORE),-score="$(SCORE)",) \
		$(if $(FIRST_SERVER),-first-server=$(FIRST_SERVER),)

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
