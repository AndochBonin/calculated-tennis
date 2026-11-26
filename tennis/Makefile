SHELL := /bin/zsh

.PHONY: get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest web

get-stats:
	go run ./cmd/getstats $(if $(PLAYER),-player="$(PLAYER)",) \
		$(if $(SURFACE),-surface=$(SURFACE),)

fetch-rates:
	go run ./cmd/fetchrates $(if $(MATCHES),-matches=$(MATCHES),) \
		$(if $(OUT),-out=$(OUT),) \
		$(if $(MERGE),-merge=true,)

fill-rates-dr:
	go run ./cmd/fetchrates $(if $(MATCHES),-matches=$(MATCHES),) \
		$(if $(OUT),-out=$(OUT),) \
		-merge -fill-dr

fetch-career:
	go run ./cmd/fetchcareer $(if $(MATCHES),-matches=$(MATCHES),) \
		$(if $(DIR),-dir=$(DIR),) \
		$(if $(MERGE),-merge=true,)

calibrate-alpha:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	go run ./cmd/calibratealpha

calibrate-form:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	@test -d tennisabstract/testdata/career && ls tennisabstract/testdata/career/*.json >/dev/null 2>&1 || $(MAKE) fetch-career
	go run ./cmd/calibrateform

backtest:
	@test -f tennisabstract/testdata/player_rates_2024.json || $(MAKE) fetch-rates
	@test -d tennisabstract/testdata/career && ls tennisabstract/testdata/career/*.json >/dev/null 2>&1 || $(MAKE) fetch-career
	go run ./cmd/backtestbets $(if $(MIN_PICK),-min-pick=$(MIN_PICK),) \
		$(if $(STAKE),-stake=$(STAKE),) \
		$(if $(SIMS),-sims=$(SIMS),)

web:
	cd web && npm ci && npm run build
	go run ./cmd/web
