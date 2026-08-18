.DEFAULT_GOAL := help

BINARY      := bin/server
PROFILE     := coverage.out
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS     := -s -w -X main.version=$(VERSION)
FUZZTIME    ?= 30s

# Coverage is measured over ./internal/... only. cmd/ is deliberately excluded:
# main() is wiring, and the way to keep it honest is to keep it thin, not to
# write a test that asserts the wiring is wired.
COVER_PKGS  := ./internal/...

.PHONY: help
help: ## List the available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: run
run: ## Run the server with the development defaults
	CALC_CLIENTS_PLAINTEXT=web:dev-secret go run ./cmd/server

.PHONY: build
build: ## Compile a stripped binary into bin/
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/server

.PHONY: test
test: ## Run the full test suite with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests with coverage and enforce the threshold
	go test $(COVER_PKGS) -covermode=atomic -coverprofile=$(PROFILE)
	@go tool cover -func=$(PROFILE) | tail -1
	@./scripts/coverage.sh $(PROFILE)

# OPEN=0 writes the report without launching a browser, which is what the
# repository-root target wants.
OPEN ?= 1

.PHONY: cover-html
cover-html: cover ## Write coverage.html and open it (OPEN=0 to only write)
	go tool cover -html=$(PROFILE) -o coverage.html
	@echo "report: file://$(CURDIR)/coverage.html"
	@[ "$(OPEN)" = "1" ] && command -v open >/dev/null && open coverage.html || true

.PHONY: cover-report
cover-report: cover ## Print per-function coverage
	go tool cover -func=$(PROFILE)

.PHONY: fuzz
fuzz: ## Fuzz the parser; it must never panic
	go test ./internal/calc -run=NONE -fuzz=FuzzParse -fuzztime=$(FUZZTIME)

.PHONY: lint
lint: ## Vet the code and check formatting
	go vet ./...
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed in:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: tidy
tidy: ## Tidy and verify the module graph
	go mod tidy
	go mod verify

.PHONY: hash
hash: ## Bcrypt a client secret for CALC_CLIENTS: make hash SECRET=...
	@test -n "$(SECRET)" || (echo "usage: make hash SECRET=your-secret"; exit 1)
	@go run ./scripts/hashsecret "$(SECRET)"

.PHONY: clean
clean: ## Remove build and coverage artefacts
	rm -rf bin $(PROFILE)
