.PHONY: generate test e2e-live lint lint-ci analyze verify webui restart

CUSTOM_LINT ?= ./custom-golangci-lint

custom-golangci-lint: .custom-gcl.yml
	golangci-lint custom

generate:
	go generate ./core/rpc/gen

test: generate
	go test ./...

e2e-live: generate
	go test -tags e2e_live -run TestLiveDownloadSpawn -v ./test/e2e

lint: generate
	@if [ -x $(CUSTOM_LINT) ]; then \
		$(CUSTOM_LINT) run; \
	else \
		golangci-lint run ./...; \
	fi

lint-ci: generate custom-golangci-lint
	$(CUSTOM_LINT) run

analyze:
	go tool sizeanalyzer -html size-report2.html

verify: test lint

webui:
	cd webui && pnpm run build

restart: webui
	-go run . gateway down 2>/dev/null; true
	go run . gateway
