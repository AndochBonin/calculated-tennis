# Monorepa wrapper: delegates to polymarket/ and tennis/ modules.
# From repo root: make test-workspace runs both modules; make test runs polymarket coverage gate only.
SHELL := /bin/zsh

.PHONY: run up down push-binary live-clob test creds deploy-wallet venv localstack-init-secret
.PHONY: get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest
.PHONY: test-workspace

# make run builds and launches the tennis web UI. Polymarket's own run stays reachable via
# `make -C polymarket run`.
run:
	$(MAKE) -C tennis web

up down push-binary live-clob test creds deploy-wallet venv localstack-init-secret:
	$(MAKE) -C polymarket $@

get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest:
	$(MAKE) -C tennis $@

test-workspace:
	go test ./tennis/... ./polymarket/...

