# Monorepa wrapper: delegates to polymarket/ and tennis/ modules.
# From repo root: make test-workspace runs both modules; make test runs polymarket coverage gate only.
SHELL := /bin/zsh

.PHONY: run up down push-binary live-clob place-order test creds deploy-wallet venv localstack-init-secret
.PHONY: get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest sim
.PHONY: test-workspace

run up down push-binary live-clob place-order test creds deploy-wallet venv localstack-init-secret:
	$(MAKE) -C polymarket $@

get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest sim:
	$(MAKE) -C tennis $@

test-workspace:
	go test ./tennis/... ./polymarket/...

