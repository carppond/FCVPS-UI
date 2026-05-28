# Multi-stage build: web (Node) → backend (Go) → runtime (Alpine + Nginx).
# Final image: ~30MB, runs Go binary + Nginx in a single container.

# ────────────────────────────────────────────────────────────
# Stage 1: Build web frontend
# ────────────────────────────────────────────────────────────
FROM node:20-alpine AS web-builder
WORKDIR /app
RUN npm install -g pnpm@9
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# ────────────────────────────────────────────────────────────
# Stage 2: Build Go backend
# ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS go-builder
WORKDIR /app
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -trimpath \
    -o /out/hub ./cmd/server

# ────────────────────────────────────────────────────────────
# Stage 3: Runtime
# ────────────────────────────────────────────────────────────
FROM alpine:3.20
RUN apk add --no-cache nginx ca-certificates tini && \
    mkdir -p /run/nginx /data /var/www/html

COPY --from=go-builder /out/hub /usr/local/bin/hub
COPY --from=web-builder /app/dist /var/www/html
COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENV SHIGUANG_DATA_DIR=/data \
    SHIGUANG_HTTP_ADDR=127.0.0.1:8080 \
    SHIGUANG_LOG_LEVEL=info \
    SHIGUANG_LOG_FORMAT=json

EXPOSE 80
VOLUME /data

ENTRYPOINT ["/sbin/tini", "--", "/entrypoint.sh"]
