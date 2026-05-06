COVERAGE_THRESHOLD ?= 80

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
