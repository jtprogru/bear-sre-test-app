#!make
SHELL := /bin/bash
.SILENT:
.DEFAULT_GOAL := help

# Global vars
export SYS_GO=$(shell which go)
export SYS_PY3=$(shell which python3)
export SYS_GOFMT=$(shell which gofmt)
export SYS_DOCKER=$(shell which docker)

export BINARY_DIR=dist
export BINARY_NAME=bear-sre-test-app
export DOCKER_REPO="ghcr.io/jtprogru/$(BINARY_NAME)"
export LATEST_COMMIT_SHA=$(shell git rev-parse --short HEAD)

include .env
export $(shell sed 's/=.*//' .env)

.PHONY: run.cmd
## Run as go run cmd/app/main.go
run.cmd: cmd/app/main.go
	$(SYS_GO) run cmd/app/main.go

.PHONY: run.bin
## Run as binary
run.bin: build.bin
	source .env && ./$(BINARY_DIR)/$(BINARY_NAME)

.PHONY: run.dc
## Run in Docker
run.dc: build.img
	$(SYS_DOCKER) compose up -d

.PHONY: down.dc
## Down Docker compose
down.dc:
	$(SYS_DOCKER) compose down -v

.PHONY: install-deps
## Install all requirements
install-deps: go.mod
	$(SYS_GO) mod tidy

.PHONY: build.bin
## Build bin file from go
build.bin: cmd/app/main.go
	$(SYS_GO) mod download && CGO_ENABLED=0 $(SYS_GO) build -o ./$(BINARY_DIR)/$(BINARY_NAME) cmd/app/main.go

.PHONY: build.img
## Build docker image
build.img: Dockerfile
	$(SYS_DOCKER) build -t $(DOCKER_REPO):$(LATEST_COMMIT_SHA) .

.PHONY: fmt
## Run go fmt
fmt:
	$(SYS_GOFMT) -s -w .

.PHONY: vet
## Run go vet ./...
vet:
	$(SYS_GO) vet ./...

.PHONY: tidy
## Run go mod tidy
tidy:
	$(SYS_GO) mod tidy

.PHONY: clean
## Clean all artifacts
clean:
	rm -rf $(BINARY_DIR)

.PHONY: test
## Run all test
test:
	go test --short -coverprofile=cover.out -v ./...
	make test.coverage

.PHONY: emoji_download
## Run python3 internal/emj/emoji-helper.py emoji_download
emoji_download:
	$(SYS_PY3) internal/emj/emoji-helper.py emoji_download

.PHONY: emoji_map
## Run python3 internal/emj/emoji-helper.py emoji_map
emoji_map:
	$(SYS_PY3) internal/emj/emoji-helper.py emoji_map

#.PHONY: test.integration
### Run test integration
#test.integration:
#	docker run --rm -d -p 27019:27017 --name $$TEST_CONTAINER_NAME -e MONGODB_DATABASE=$$TEST_DB_NAME mongo:4.4-bionic
#
#	GIN_MODE=release go test -v ./tests/
#	docker stop $$TEST_CONTAINER_NAME

#.PHONY: test.coverage
### Run test coverage
#test.coverage:
#	go tool cover -func=cover.out

#.PHONY: swag
### Run swag
#swag:
#	swag init -g internal/app/app.go

.PHONY: lint
## Run golangci-lint
lint:
	golangci-lint -v run --out-format=colored-line-number

#.PHONY: gen
### Run mockgen
#gen:
#	mockgen -source=internal/service/service.go -destination=internal/service/mocks/mock.go
#	mockgen -source=internal/repository/repository.go -destination=internal/repository/mocks/mock.go

.PHONY: help
## Show this help message
help:
	@echo "$$(tput bold)Available rules:$$(tput sgr0)"
	@echo
	@sed -n -e "/^## / { \
		h; \
		s/.*//; \
		:doc" \
		-e "H; \
		n; \
		s/^## //; \
		t doc" \
		-e "s/:.*//; \
		G; \
		s/\\n## /---/; \
		s/\\n/ /g; \
		p; \
	}" ${MAKEFILE_LIST} \
	| LC_ALL='C' sort --ignore-case \
	| awk -F '---' \
		-v ncol=$$(tput cols) \
		-v indent=19 \
		-v col_on="$$(tput setaf 6)" \
		-v col_off="$$(tput sgr0)" \
	'{ \
		printf "%s%*s%s ", col_on, -indent, $$1, col_off; \
		n = split($$2, words, " "); \
		line_length = ncol - indent; \
		for (i = 1; i <= n; i++) { \
			line_length -= length(words[i]) + 1; \
			if (line_length <= 0) { \
				line_length = ncol - indent - length(words[i]) - 1; \
				printf "\n%*s ", -indent, " "; \
			} \
			printf "%s ", words[i]; \
		} \
		printf "\n"; \
	}' \
	| more $(shell test $(shell uname) == Darwin && echo '--no-init --raw-control-chars')
