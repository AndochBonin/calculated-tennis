SHELL := /bin/zsh

.PHONY: run build test

run:
	$(MAKE) -C tennis web

build test:
	$(MAKE) -C tennis $@
