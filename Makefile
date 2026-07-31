#!make
SHELL := /bin/bash
.DEFAULT_GOAL := help

# .env намеренно НЕ подключается глобально. Раньше здесь стоял `include .env`
# без дефиса — без файла падала любая цель, включая help. Но и `-include .env`
# в паре с `export` не годится: SRV_ADDR уехал бы в окружение всех дочерних
# процессов, включая go test, и conformance тестировал бы сервис на чужом
# порту. Переменные из .env нужны только целям запуска — см. DOTENV ниже.
DOTENV = set -a; [ -f .env ] && . ./.env; set +a;

GO          ?= $(shell command -v go)
BINARY_DIR  ?= dist
BINARY_NAME ?= bear-sre-test-app
BINARY      := $(BINARY_DIR)/$(BINARY_NAME)
DOCKER_REPO ?= ghcr.io/jtprogru/bear-sre-test-app
BASE_URL    ?= http://127.0.0.1:8080
SECRET_FILE ?= testdata/jtprogru.test


.PHONY: run
## Запустить через go run
run:
	@$(DOTENV) $(GO) run ./cmd/app -debug

.PHONY: run.bin
## Собрать и запустить бинарь
run.bin: build.bin
	@$(DOTENV) ./$(BINARY) -debug

.PHONY: build.bin
## Собрать бинарь
build.bin:
	CGO_ENABLED=0 $(GO) build -ldflags="-w -s" -o ./$(BINARY) ./cmd/app

.PHONY: build.img
## Собрать docker-образ
build.img:
	docker build -t $(DOCKER_REPO):local .

.PHONY: up
## Поднять через docker compose
up: testdata
	docker compose up -d --build

.PHONY: down
## Погасить docker compose
down:
	docker compose down -v

.PHONY: testdata
## Создать секретный файл для /secret
testdata:
	@test -s $(SECRET_FILE) || { \
		mkdir -p $$(dirname $(SECRET_FILE)); \
		dd if=/dev/urandom of=$(SECRET_FILE) bs=1024 count=4 status=none; \
		echo "created $(SECRET_FILE)"; \
	}

.PHONY: test
## Прогнать unit-тесты под race detector
test:
	$(GO) test -race -coverprofile=cover.out ./...

.PHONY: test.coverage
## Показать покрытие
test.coverage: test
	$(GO) tool cover -func=cover.out

.PHONY: lint
## Запустить golangci-lint
lint:
	golangci-lint run --timeout 5m

.PHONY: lint.docker
## Запустить hadolint
lint.docker:
	hadolint Dockerfile

.PHONY: sec
## gosec + govulncheck
sec:
	gosec -quiet ./...
	govulncheck ./...

.PHONY: fmt
## Отформатировать код
fmt:
	gofmt -s -w .

.PHONY: vet
## Запустить go vet
vet:
	$(GO) vet ./...

.PHONY: tidy
## Запустить go mod tidy
tidy:
	$(GO) mod tidy

.PHONY: check
## Полная проверка — то же, что гоняет CI
check: fmt vet lint sec test.coverage lint.docker

.PHONY: load
## Нагрузочный прогон k6 (сервис должен быть уже запущен)
load:
	BASE_URL=$(BASE_URL) k6 run k6/load.js

.PHONY: clean
## Убрать артефакты сборки
clean:
	rm -rf $(BINARY_DIR) cover.out

.PHONY: help
## Показать это сообщение
help:
	@printf '%sAvailable targets:%s\n\n' "$$(tput bold 2>/dev/null)" "$$(tput sgr0 2>/dev/null)"
	@awk ' \
		/^## / { sub(/^## /, ""); doc = doc (doc ? " " : "") $$0; next } \
		/^[a-zA-Z0-9][a-zA-Z0-9_.\/-]*:/ { \
			t = $$1; sub(/:.*/, "", t); \
			if (doc != "") printf "  \033[36m%-18s\033[0m %s\n", t, doc; \
			doc = ""; next \
		} \
		{ doc = "" } \
	' $(MAKEFILE_LIST) | sort -f
