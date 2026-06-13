.PHONY: help
.DEFAULT_GOAL := help

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: *.go  ## Builds the program
	go build -o dist/tgo

test: ## Run tests
	go test ./...

lint: ## Run linter
	golangci-lint run ./...

devprep:  ## Install dev tooling
	pre-commit install
