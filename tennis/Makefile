SHELL := /bin/zsh

.PHONY: get-stats fetch-rates fetch-career fill-rates-dr calibrate-alpha calibrate-form backtest sim tui

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

sim:
	go run ./cmd/simmatch $(if $(PLAYER_A),-player-a="$(PLAYER_A)",) \
		$(if $(PLAYER_B),-player-b="$(PLAYER_B)",) \
		$(if $(SURFACE),-surface=$(SURFACE),) \
		$(if $(FORMAT),-format=$(FORMAT),) \
		$(if $(ALPHA),-alpha=$(ALPHA),) \
		$(if $(SIMS),-sims=$(SIMS),) \
		$(if $(SCORE),-score="$(SCORE)",) \
		$(if $(FIRST_SERVER),-first-server=$(FIRST_SERVER),)

tui:
	go run ./cmd/tui
