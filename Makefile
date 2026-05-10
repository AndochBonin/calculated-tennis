COVERAGE_THRESHOLD ?= 90
SHELL := /bin/zsh

run:
	go run .

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
# Usage: make place-order PRICE=0.50 TOKEN=<token_id>
place-order:
	@if [ -z "$(PRICE)" ] || [ -z "$(TOKEN)" ]; then \
		echo "usage: make place-order PRICE=<decimal> TOKEN=<token_id>"; \
		exit 2; \
	fi
	go run ./cmd/placeorder -price=$(PRICE) $(TOKEN)

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
