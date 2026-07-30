# syntax=docker/dockerfile:1

FROM oven/bun:1 AS web-builder
WORKDIR /web
COPY web/package.json web/bun.lock* ./
RUN bun install --frozen-lockfile || bun install
COPY web/ .
RUN bun run build

FROM golang:1.24-bookworm AS go-builder
WORKDIR /src
ENV GOTOOLCHAIN=auto
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web-builder /web/dist ./web/dist
RUN CGO_ENABLED=0 go build -o /out/sentry-lite ./cmd/sentry-lite

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-builder /out/sentry-lite /app/sentry-lite
COPY --from=go-builder /src/web/dist /app/web/dist
COPY --from=go-builder /src/migrations /app/migrations
ENV WEB_DIST=/app/web/dist
ENV SQLITE_PATH=/data/sentry-lite.db
ENV DATA_DIR=/data
EXPOSE 8080
CMD ["/app/sentry-lite"]
