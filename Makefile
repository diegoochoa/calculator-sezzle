.DEFAULT_GOAL := help

# Convenience wrapper over the two services. Each keeps its own toolchain and
# its own targets; this exists so the whole repository can be checked with one
# command rather than two.

.PHONY: help
help: ## List the available targets
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: ## Build and run both services in containers
	docker compose up --build

.PHONY: down
down: ## Stop and remove the containers
	docker compose down

.PHONY: test
test: ## Run both test suites
	$(MAKE) -C backend test
	cd frontend && npm test

.PHONY: lint
lint: ## Lint both sides
	$(MAKE) -C backend lint
	cd frontend && npm run lint

.PHONY: coverage
coverage: ## Run both suites with coverage and enforce both gates
	$(MAKE) -C backend cover
	cd frontend && npm run test:coverage

.PHONY: coverage-html
coverage-html: ## Write both browsable coverage reports and print their paths
	@$(MAKE) -C backend cover-html OPEN=0
	@cd frontend && npm run test:coverage
	@echo
	@echo "Coverage reports written to:"
	@echo "  backend   file://$(CURDIR)/backend/coverage.html"
	@echo "  frontend  file://$(CURDIR)/frontend/coverage/index.html"

.PHONY: clean
clean: ## Remove build and coverage artefacts from both sides
	$(MAKE) -C backend clean
	rm -rf frontend/coverage frontend/dist
