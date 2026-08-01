FROM golang:1.26-alpine3.23 AS builder
LABEL authors="Mikhail Savin <jtprogru@gmail.com>"

WORKDIR /src

# Слой зависимостей отдельно от исходников: правка кода не инвалидирует кеш модулей.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# -w -s выбрасывают отладочную информацию и таблицу символов: образ меньше,
# а для рантайма они не нужны.
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /out/app ./cmd/app

FROM alpine:3.24
LABEL authors="Mikhail Savin <jtprogru@gmail.com>"
LABEL org.opencontainers.image.source="https://github.com/jtprogru/bear-sre-test-app"

# --no-cache не оставляет индекс пакетов в слое.
# UID числовой: с буквенным Kubernetes не может проверить runAsNonRoot.
# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates \
    && addgroup -S -g 10001 appgroup \
    && adduser -S -u 10001 -G appgroup appuser

# Конфиг кладём в образ, иначе контейнер стартует и сразу падает: приложение
# ищет config.yaml, а в compose не было ни volume, ни COPY.
COPY --chown=appuser:appgroup config.example.yaml /etc/testapp/config.yaml
COPY --from=builder --chown=appuser:appgroup /out/app /app/app

EXPOSE 8080

# Контейнер работает не от root.
USER 10001:10001

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD ["wget", "--no-verbose", "--tries=1", "--spider", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/app/app"]
